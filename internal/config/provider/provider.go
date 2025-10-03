package provider

import (
	"context"
)

// Provider defines a Load function to load configuration values from a source
type Provider interface {
	// Provider loads config from one source and returns a fresh, immutable map
	// of CANONICAL KEYS (UPPER_SNAKE, no prefixes). Implementations must NOT
	// mutate the returned map after Load returns.
	// Overload Precedence: Provider will produce one Layer read by the manager
	// Last layer read wins if collision occurs
	//
	// Load performance should be at most O(keys), DO NOT ALLOCATE beyond returned map
	Load(ctx context.Context) (map[string]string, error)

	// Name() returns the name of this provider for logs/metrics
	Name() string
}

// Watchable marks a provider as capable
// of listening for changes to its source
// and updating a config manager
// Watchables should be idempotent, long-lived
// They should call notify() when changes occur,
// and return a ctx.Done(); Watchables should NOT block the min thread
type Watchable interface {
	// Call notify() when you detect an underlying change.
	Watch(ctx context.Context, notify func()) error
}
