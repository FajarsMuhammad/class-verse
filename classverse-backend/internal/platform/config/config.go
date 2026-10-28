// Package config loads application settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"strings"
)

// Environments accepted in APP_ENV.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

// Config holds all settings the server needs at startup.
type Config struct {
	AppEnv      string
	HTTPAddr    string
	DatabaseURL string
	FrontendURL string
	SMTPAddr    string
	SMTPFrom    string
}

// IsProduction reports whether the server runs in production mode.
func (c Config) IsProduction() bool {
	return c.AppEnv == EnvProduction
}

// Load reads the configuration using getenv (usually os.Getenv).
// All missing required variables are reported together in one error.
func Load(getenv func(string) string) (Config, error) {
	var missing []string
	required := func(key string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	optional := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	cfg := Config{
		AppEnv:      optional("APP_ENV", EnvDevelopment),
		HTTPAddr:    optional("HTTP_ADDR", ":8080"),
		DatabaseURL: required("DATABASE_URL"),
		FrontendURL: required("FRONTEND_URL"),
		SMTPAddr:    required("SMTP_ADDR"),
		SMTPFrom:    required("SMTP_FROM"),
	}

	var errs []error
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", ")))
	}
	if cfg.AppEnv != EnvDevelopment && cfg.AppEnv != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, cfg.AppEnv))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
