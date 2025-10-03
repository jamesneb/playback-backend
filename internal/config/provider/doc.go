// Package provider defines an interface for configuration value providers.
//
// Configuration values could come from environment variables, external files, secrets managers etc..
// specific "types" of providers should implement the Provider interface and hide their implementation details
//
// For example, an environment variable provider could implement env var parsing using [caarlos0/env] and hide
// that dependency behind the Load function
//
// [caarlos0/env]: https://github.com/caarlos0/env

package provider
