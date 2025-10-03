package config

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
	"github.com/jamesneb/playback-backend/internal/config/grpc"
	resolver "github.com/jamesneb/playback-backend/internal/config/propertyresolver"
	"github.com/jamesneb/playback-backend/internal/config/provider"
)

const (
	DEBOUNCE_INTERVAL = 150 * time.Millisecond
)

// Snapshot is a typed, immutable collection of Config sections
type Snapshot struct {
	GRPCServer *grpc.Config
}

// Plan returns a plan for a snapshot to be committed to the configuration manager
type Plan struct {
	Snapshot Snapshot
	FP       uint64
}

// Manager is a configuration manager, capable of providing section Snapshots with hot reloading via Subscribe.
//
// Manager is safe for concurrent use
type Manager struct {
	providers []provider.Provider

	ptr    atomic.Pointer[Snapshot] // atomic read path
	lastFP atomic.Uint64            // fingerprint of merged keys for no-op elision
	subsMu sync.RWMutex

	subscribers map[string]func(original, updated Snapshot) // non-blocking fan-out in broadcast
}

type Dict struct {
	data   map[string]string
	prefix string
}

func (d Dict) Entries() map[string]string { return d.data }
func (d Dict) Prefix() string             { return d.prefix }

// NewDict returns a read-only overlay, not a live view of provider data
// Callers should not mutate the returned map
func NewDict(m map[string]string) Dict {
	return Dict{data: m}
}

func (d Dict) Resolve(key string) (string, bool) {
	k := d.prefix + base.Normalize(key)
	v, ok := d.data[k]
	return v, ok
}

// Returns a prefix-aware Dict over a property resolver
// Prefixes can be composed - e.g., WithPrefix)"GRPC").WithPrefix("Server") =
// "GRPC_SERVER_
func (d Dict) WithPrefix(prefix string) resolver.PropertyResolver {
	if prefix == "" {
		return d
	}
	return Dict{data: d.data, prefix: d.prefix + base.Normalize(prefix)}
}

func validateAll(s Snapshot) error {
	var errs []error

	if err := s.GRPCServer.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("grpc: %w", err))
	}

	// if err := s.Server.Validate(); err != nil { errs = append(errs, fmt.Errorf("server: %w", err)) }

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...) // Go 1.20+
}

// NewManager creates a new Manager instance with the given providers and then calls Reload
func NewManager(ctx context.Context, providers ...provider.Provider) (*Manager, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("config: no providers")
	}

	m := &Manager{providers: providers, subscribers: make(map[string]func(original, updated Snapshot))}
	if err := m.Reload(ctx); err != nil {
		return nil, err
	}
	m.StartWatchers(ctx, DEBOUNCE_INTERVAL)

	return m, nil
}

// Snapshot returns the current, immutable consolidated snapshot of configuration key-values
// Snapshot is a superset of all configuration values, application logic should only use
// individual subsets at any one time
// Snapshot() is cheap + atomic
func (m *Manager) Snapshot() *Snapshot {
	s := m.ptr.Load()
	if s == nil { // Should never happen, NewManager calls Reload before returning
		return &Snapshot{}
	}

	return s
}

// Subscribe registers a consumer with the config manager
// Calls with duplicate IDs will collide - later call overrides
// Panics are recovered and then dropped
// Handlers should be performant
func (m *Manager) Subscribe(id string, fn func(original, updated Snapshot)) {
	m.subsMu.Lock()
	m.subscribers[id] = fn
	m.subsMu.Unlock()
}

// fetchLayers loads all configuration settings from each provider
// and stores them in separate entries in a map
// later layers OVERRIDE earlier ones - last provider of K in m[k] wins
func (m *Manager) fetchLayers(ctx context.Context) ([]map[string]string, error) {
	out := make([]map[string]string, 0, len(m.providers))

	for _, p := range m.providers {
		kv, err := p.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s load: %w", p.Name(), err)
		}
		out = append(out, kv)
	}
	return out, nil
}

// Decode fills a Snapshot with section config structs by resolving each section with
// a property resolver
func (m *Manager) decode(resolver resolver.PropertyResolver) (Snapshot, error) {
	var cand Snapshot
	var err error

	// Call specific section decoders
	grpcCfg, err := grpc.FromResolver(resolver)
	if err != nil {
		return Snapshot{}, fmt.Errorf("grpc decode: %w", err)
	}
	cand.GRPCServer = &grpcCfg

	return cand, nil
}

// createPlan creates a complete map of all possible configuration settings
// from all providers, to be validated by the config manager
func (m *Manager) createPlan(layers []map[string]string, prev uint64) (bool, Plan, error) {
	merged := base.Merge(layers...)
	fp := base.Fingerprint(merged)
	// Skip no-op only if fingerprint matches AND we've loaded before (prev != 0)
	if fp == prev && prev != 0 {
		return false, Plan{}, nil
	}
	res := NewDict(merged)
	snapshot, err := m.decode(res)
	if err != nil {
		return false, Plan{}, fmt.Errorf("error decoding layers: %w", err)
	}
	if err := validateAll(snapshot); err != nil {
		return false, Plan{}, fmt.Errorf("validate: %w", err)
	}
	plan := Plan{Snapshot: snapshot, FP: fp}

	return true, plan, nil
}

// StartWatchers wires provider change signals into debounced reloads.
//
// Contract:
//   - Only providers implementing Watchable are registered.
//   - Multiple rapid signals are coalesced (debounce window).
//   - Reloads are best effort and idempotent; errors are logged and dropped.
//   - The goroutine exits when ctx is canceled.
//
// Note: Env polling is intended for local/dev; production env changes typically
// require a process restart.
func (m *Manager) StartWatchers(ctx context.Context, debounce time.Duration) {
	events := make(chan struct{}, 1) // coalesce multiple signals

	// Wire provider watchers
	for _, p := range m.providers {
		if w, ok := p.(provider.Watchable); ok {
			go func(w provider.Watchable, name string) {
				// NOTE: Watch should block and call notify() on changes.
				if err := w.Watch(ctx, func() {
					select {
					case events <- struct{}{}:
					default:
					} // don't block
				}); err != nil {
					// TODO: plug logger/metrics here (watcher exited with error)
					// log.Printf("config: watcher %s exited: %v", name, err)
				}
			}(w, p.Name())
		}
	}

	// Debounced reloader
	go func() {
		var timer *time.Timer
		var tick <-chan time.Time // nil until timer armed

		arm := func() {
			if timer == nil {
				timer = time.NewTimer(debounce)
				tick = timer.C
				return
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(debounce)
			tick = timer.C
		}

		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
				}
				return
			case <-events:
				arm()
			case <-tick:
				// Disarm before work so bursts retrigger afterward
				tick = nil
				if err := m.Reload(ctx); err != nil {
					// TODO: log/metric reload error
				}
			}
		}
	}()
}

func (m *Manager) broadcast(original, updated Snapshot) {
	// Copy subscriber fns without holding the lock while invoking them.
	m.subsMu.RLock()
	subs := make([]func(Snapshot, Snapshot), 0, len(m.subscribers))
	for _, fn := range m.subscribers {
		subs = append(subs, fn)
	}
	m.subsMu.RUnlock()

	for _, fn := range subs {
		fn := fn
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// TODO: plug logger/metrics here
					// log.Printf("config broadcast panic: %v", r)
					_ = r // silence SA9003
				}
			}()
			fn(original, updated)
		}()
	}
}

func (m *Manager) Reload(ctx context.Context) error {
	// Step 1 Load from providers (impure I/O)
	layers, err := m.fetchLayers(ctx)
	if err != nil {
		return fmt.Errorf("error reloading: %w", err)
	}

	// Step 2 create replacement plan
	changed, plan, err := m.createPlan(layers, m.lastFP.Load())
	if err != nil {
		return err
	}

	if !changed {
		return nil // no-op, do not notify
	}

	// 3) Atomic swap + async notify
	old := m.ptr.Swap(&plan.Snapshot) // requires Go 1.19+; else Load/Store pair
	m.lastFP.Store(plan.FP)

	if old != nil {
		go m.broadcast(*old, plan.Snapshot) // non-blocking fan-out
	}
	return nil
}
