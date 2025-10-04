// Package data defines the needed configuration for data processing pipelines
//
// -- Constants.go --
//
// Helpful constants related to config instantiation and readability.
// For example, the Data Prefix used to query the Config Manager for JUST Data config values
// is defined here. Default config values (used when no provider is present) are defined here.
// For example, DEFAULT_BATCH_SIZE provides a value for batch processing.
//
// -- Section.go ---
//
// Implements [config.Manager] configuration subset. Implements Defaults(), FromResolver, Validate etc.
package data
