package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
)

var (
	testSeed   = make([]byte, ed25519.SeedSize)
	testOTPKey = []byte(strings.Repeat("k", 32))
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":    "postgres://u:p@localhost:5432/db",
		"FRONTEND_URL":    "http://localhost:5173",
		"SMTP_ADDR":       "localhost:1025",
		"SMTP_FROM":       "no-reply@kelasverse.test",
		"JWT_PRIVATE_KEY": base64.StdEncoding.EncodeToString(testSeed),
		"OTP_HMAC_KEY":    base64.StdEncoding.EncodeToString(testOTPKey),
	}
}

func baseConfig() Config {
	return Config{
		AppEnv:        EnvDevelopment,
		HTTPAddr:      ":8080",
		DatabaseURL:   "postgres://u:p@localhost:5432/db",
		FrontendURL:   "http://localhost:5173",
		SMTPAddr:      "localhost:1025",
		SMTPFrom:      "no-reply@kelasverse.test",
		JWTPrivateKey: ed25519.NewKeyFromSeed(testSeed),
		OTPHMACKey:    testOTPKey,
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(m map[string]string)
		want    func() Config
		wantErr []string
	}{
		{
			name:   "defaults for optional variables",
			modify: func(map[string]string) {},
			want:   baseConfig,
		},
		{
			name: "explicit optional variables",
			modify: func(m map[string]string) {
				m["APP_ENV"] = "production"
				m["HTTP_ADDR"] = ":9000"
				m["FRONTEND_URL"] = "http://localhost:5173/"
				m["GOOGLE_CLIENT_ID"] = "id"
				m["GOOGLE_CLIENT_SECRET"] = "secret"
				m["GOOGLE_REDIRECT_URL"] = "http://localhost:5173/api/auth/google/callback"
			},
			want: func() Config {
				c := baseConfig()
				c.AppEnv = EnvProduction
				c.HTTPAddr = ":9000"
				c.Google = Google{ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost:5173/api/auth/google/callback"}
				return c
			},
		},
		{
			name: "all missing variables reported together",
			modify: func(m map[string]string) {
				delete(m, "DATABASE_URL")
				delete(m, "SMTP_FROM")
				delete(m, "JWT_PRIVATE_KEY")
				m["FRONTEND_URL"] = "   "
			},
			wantErr: []string{"DATABASE_URL", "FRONTEND_URL", "SMTP_FROM", "JWT_PRIVATE_KEY"},
		},
		{
			name:    "invalid APP_ENV",
			modify:  func(m map[string]string) { m["APP_ENV"] = "staging" },
			wantErr: []string{"APP_ENV", "staging"},
		},
		{
			name: "invalid keys",
			modify: func(m map[string]string) {
				m["JWT_PRIVATE_KEY"] = base64.StdEncoding.EncodeToString([]byte("short"))
				m["OTP_HMAC_KEY"] = "not base64!"
			},
			wantErr: []string{"JWT_PRIVATE_KEY", "OTP_HMAC_KEY"},
		},
		{
			name:    "partial google config",
			modify:  func(m map[string]string) { m["GOOGLE_CLIENT_ID"] = "id" },
			wantErr: []string{"GOOGLE_CLIENT_SECRET"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validEnv()
			tt.modify(m)

			got, err := Load(env(m))

			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatalf("Load() error = nil, want error mentioning %v", tt.wantErr)
				}
				for _, s := range tt.wantErr {
					if !strings.Contains(err.Error(), s) {
						t.Errorf("Load() error = %q, want it to mention %q", err, s)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if want := tt.want(); !reflect.DeepEqual(got, want) {
				t.Errorf("Load() = %+v, want %+v", got, want)
			}
		})
	}
}
