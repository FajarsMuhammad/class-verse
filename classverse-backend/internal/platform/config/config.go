// Package config loads application settings from environment variables.
package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Environments accepted in APP_ENV.
const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

const minOTPKeyLen = 32

// Google holds the OAuth client settings. Login with Google is disabled when
// all three are empty (for example on a laptop without a Google client).
type Google struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// Enabled reports whether Google login is configured.
func (g Google) Enabled() bool {
	return g.ClientID != ""
}

// Config holds all settings the server needs at startup.
type Config struct {
	AppEnv        string
	HTTPAddr      string
	DatabaseURL   string
	FrontendURL   string
	SMTPAddr      string
	SMTPFrom      string
	JWTPrivateKey ed25519.PrivateKey
	OTPHMACKey    []byte
	Google        Google
	LiveKit       LiveKit
	Storage       Storage
}

// Storage is optional; credentials are never logged.
type Storage struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Secure    bool
}

// LiveKit configures voice (slice 9). All empty disables voice; validation
// of the combination happens in voice.NewLiveKit.
type LiveKit struct {
	URL       string
	APIKey    string
	APISecret string
}

// IsProduction reports whether the server runs in production mode.
func (c Config) IsProduction() bool {
	return c.AppEnv == EnvProduction
}

// Load reads the configuration using getenv (usually os.Getenv).
// All missing or invalid variables are reported together in one error.
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
		FrontendURL: strings.TrimRight(required("FRONTEND_URL"), "/"),
		SMTPAddr:    required("SMTP_ADDR"),
		SMTPFrom:    required("SMTP_FROM"),
		Google: Google{
			ClientID:     optional("GOOGLE_CLIENT_ID", ""),
			ClientSecret: optional("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  optional("GOOGLE_REDIRECT_URL", ""),
		},
		LiveKit: LiveKit{
			URL:       optional("LIVEKIT_URL", ""),
			APIKey:    optional("LIVEKIT_API_KEY", ""),
			APISecret: optional("LIVEKIT_API_SECRET", ""),
		},
	}
	jwtSeed := required("JWT_PRIVATE_KEY")
	otpKey := required("OTP_HMAC_KEY")

	var errs []error
	if len(missing) > 0 {
		errs = append(errs, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", ")))
	}
	if cfg.AppEnv != EnvDevelopment && cfg.AppEnv != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, cfg.AppEnv))
	}
	if jwtSeed != "" {
		seed, err := base64.StdEncoding.DecodeString(jwtSeed)
		if err != nil || len(seed) != ed25519.SeedSize {
			errs = append(errs, fmt.Errorf("JWT_PRIVATE_KEY must be a base64 Ed25519 seed of %d bytes", ed25519.SeedSize))
		} else {
			cfg.JWTPrivateKey = ed25519.NewKeyFromSeed(seed)
		}
	}
	if otpKey != "" {
		key, err := base64.StdEncoding.DecodeString(otpKey)
		if err != nil || len(key) < minOTPKeyLen {
			errs = append(errs, fmt.Errorf("OTP_HMAC_KEY must be base64 of at least %d bytes", minOTPKeyLen))
		} else {
			cfg.OTPHMACKey = key
		}
	}
	cfg.Storage = Storage{Endpoint: optional("STORAGE_ENDPOINT", ""), Bucket: optional("STORAGE_BUCKET", ""), AccessKey: optional("STORAGE_ACCESS_KEY", ""), SecretKey: optional("STORAGE_SECRET_KEY", ""), Secure: optional("STORAGE_SECURE", "true") != "false"}
	if cfg.Storage.Endpoint == "" && cfg.Storage.Bucket == "" && cfg.Storage.AccessKey == "" && cfg.Storage.SecretKey == "" {
		cfg.Storage = Storage{}
	} else if countSet(cfg.Storage.Endpoint, cfg.Storage.Bucket, cfg.Storage.AccessKey, cfg.Storage.SecretKey) != 4 {
		errs = append(errs, errors.New("STORAGE_ENDPOINT, STORAGE_BUCKET, STORAGE_ACCESS_KEY, and STORAGE_SECRET_KEY must be set together"))
	}
	if raw := optional("STORAGE_SECURE", "true"); raw != "true" && raw != "false" {
		errs = append(errs, errors.New("STORAGE_SECURE must be true or false"))
	}
	if cfg.IsProduction() && cfg.Storage.Endpoint != "" && !cfg.Storage.Secure {
		errs = append(errs, errors.New("production storage requires TLS"))
	}
	g := cfg.Google
	if set := countSet(g.ClientID, g.ClientSecret, g.RedirectURL); set != 0 && set != 3 {
		errs = append(errs, errors.New("GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and GOOGLE_REDIRECT_URL must be set together"))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func countSet(values ...string) int {
	n := 0
	for _, v := range values {
		if v != "" {
			n++
		}
	}
	return n
}
