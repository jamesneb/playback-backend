package provider

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	POLL_INTERVAL = 2 * time.Second // Intended for DEV or LOCAL env...PROD env usually requires restart
)

// EnvVarProvider returns a struct for retrieving environment variables from the OS
type EnvVarProvider struct {
	Prefix string // Match at the start of canonicalized key, case insensitive
}

func (p *EnvVarProvider) Load(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, 64)
	for _, kv := range os.Environ() {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		k := strings.ToUpper(kv[:i])
		if p.Prefix != "" && !strings.HasPrefix(k, p.Prefix) {
			continue
		}
		out[k] = kv[i+1:]
	}
	return out, nil
}

func (p *EnvVarProvider) Name() string {
	return "env"
}

func (p EnvVarProvider) Watch(ctx context.Context, notify func()) error {
	// Dev-only: poll and fingerprint. In prod, envs usually require restart.
	interval := POLL_INTERVAL

	var prev uint64
	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			m, _ := p.Load(ctx)
			fp := base.Fingerprint(m)
			if fp != prev {
				prev = fp
				notify()
			}
		}
	}
}
