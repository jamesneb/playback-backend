// Defines universal config types
package base

type Port = uint16
type Byte = uint64

// SI constants
const (
	MEGA int = 1_000_000 // Useful in various calculations. Can be combined with Byte for an (MB) value (not MiB)
)

// Math constants
const (
	Infinity string = "\u221E" // "∞"
)

// Config validator types
// A small, local constraint so we don't rely on exp/constraints.
type Number interface {
	~int | ~int32 | ~int64 |
		~uint | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Multiple section config sets can use a Validator
type Validator struct {
	errs   []error
	prefix string
}
