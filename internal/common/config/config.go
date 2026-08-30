// Package config loads configuration from the environment.
//
// Everything deployment-specific arrives through the environment, so no
// credential is ever committed. The Java service this replaces kept its
// database password in application.properties, in the repository.
package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// validater is implemented by configuration types that need a check the struct
// tags cannot express.
type validater interface {
	Validate() error
}

// Load populates an instance of T from the environment.
//
// Fields are bound by their `env` struct tag and checked against their
// `validate` tag, so a setting that is missing or unusable fails at startup
// with the name of the variable rather than at the first request that needs it.
func Load[T any]() (T, error) {
	var cfg T

	if err := env.Parse(&cfg); err != nil {
		return cfg, fmt.Errorf("load config from the environment: %w", err)
	}

	if err := validate.Struct(cfg); err != nil {
		return cfg, fmt.Errorf("invalid config: %w", err)
	}

	if v, ok := any(cfg).(validater); ok {
		if err := v.Validate(); err != nil {
			return cfg, fmt.Errorf("invalid config: %w", err)
		}
	}

	return cfg, nil
}
