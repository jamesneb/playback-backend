// Package grpc defines the needed configuration for a GRPC Server
//
// -- Constants.go --
//
// Helpful constants related config instantiation and readability
// For example, the GRPC Prefix used to query the Config Manager for JUST GRPC config values
// is defined here. Default config values (used when no provider is present) are defined here.
// For example, DEFAULT_REQUESTS_PER_SECOND provides a value for the GRPC rate limiter.
//
// -- Section.go ---
//
// Implements [config.Manager] configuration subset. Implements Defaults(), FromResolver, Validate etc.
package grpc
