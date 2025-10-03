// Package base defines functions and types
// that can be usefully employed by all components of the configuration system
//
// --- TYPES ---
// Validator interface implemented here (use to validate your config values)
//
// Various SI and CS constants that help with readability are defined here, e.g. MEGA prefix as in Megabyte
// = 1_000_000
//
// --- FUNCTIONS ---
// Configuration validation functions are defined here, e.g GreaterThanZero for settings that must be...
// greater than zero.
//
// String map manipulation functions are defined here e.g. Merge() to merge maps from different sources
//
// You'll also finding hashing functions and various general utilities here
package base
