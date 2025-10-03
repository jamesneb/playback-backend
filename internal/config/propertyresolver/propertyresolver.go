// Package propertyresolver provides an interface for things that resolve
// canonical property keys to string values
//
// a PropertyResolver is read only, and must not perform I/O per call
package propertyresolver

type PropertyResolver interface {
	// Resolve resolves a value from a property key
	// Implementations MUST NOT DO I/O PER CALL.
	// Implementations should be cheap, Resolve should expect
	// uppercase canonical keys
	Resolve(key string) (value string, ok bool)
}
