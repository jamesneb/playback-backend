// Package propertyresolver defines the interface for any
// type wishing to imlement PropertyResolver.
//
// The purpose of a property resolver is to separate external sources like an environment
// from functions that must set values dependent on that environment -- keeping those functions pure
//
// -- CONFIG RETRIEVAL PROCESS --
//
// provider (reads source, returns key-value map) -> propertyresolver (wraps map, decodes) -> Config struct (queries PropertyResolver, inits fields)
package propertyresolver
