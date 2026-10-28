package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://u:p@localhost:5432/db",
		"FRONTEND_URL": "http://localhost:5173",
		"SMTP_ADDR":    "localhost:1025",
		"SMTP_FROM":    "no-reply@kelasverse.test",
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(m map[string]string)
		want    Config
		wantErr []string
	}{
		{
			name:   "defaults for optional variables",
			modify: func(map[string]string) {},
			want: Config{
				AppEnv:      EnvDevelopment,
				HTTPAddr:    ":8080",
				DatabaseURL: "postgres://u:p@localhost:5432/db",
				FrontendURL: "http://localhost:5173",
				SMTPAddr:    "localhost:1025",
				SMTPFrom:    "no-reply@kelasverse.test",
			},
		},
		{
			name: "explicit optional variables",
			modify: func(m map[string]string) {
				m["APP_ENV"] = "production"
				m["HTTP_ADDR"] = ":9000"
			},
			want: Config{
				AppEnv:      EnvProduction,
				HTTPAddr:    ":9000",
				DatabaseURL: "postgres://u:p@localhost:5432/db",
				FrontendURL: "http://localhost:5173",
				SMTPAddr:    "localhost:1025",
				SMTPFrom:    "no-reply@kelasverse.test",
			},
		},
		{
			name: "all missing variables reported together",
			modify: func(m map[string]string) {
				delete(m, "DATABASE_URL")
				delete(m, "SMTP_FROM")
				m["FRONTEND_URL"] = "   "
			},
			wantErr: []string{"DATABASE_URL", "FRONTEND_URL", "SMTP_FROM"},
		},
		{
			name:    "invalid APP_ENV",
			modify:  func(m map[string]string) { m["APP_ENV"] = "staging" },
			wantErr: []string{"APP_ENV", "staging"},
		},
		{
			name: "missing and invalid reported together",
			modify: func(m map[string]string) {
				delete(m, "SMTP_ADDR")
				m["APP_ENV"] = "prod"
			},
			wantErr: []string{"SMTP_ADDR", "APP_ENV"},
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
			if got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
