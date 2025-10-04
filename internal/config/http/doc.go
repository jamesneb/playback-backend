// Package http defines the needed configuration for an HTTP Server
//
// -- Constants.go --
//
// Helpful constants related to config instantiation and readability.
// For example, the HTTP Prefix used to query the Config Manager for JUST HTTP config values
// is defined here. Default config values (used when no provider is present) are defined here.
// For example, DEFAULT_RPS provides a value for the HTTP rate limiter.
//
// -- Section.go ---
//
// Implements [config.Manager] configuration subset. Implements Defaults(), FromResolver, Validate etc.
package http
