package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestHandle(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(w http.ResponseWriter, r *http.Request) error
		wantStatus int
		wantBody   string
		wantLogged bool
	}{
		{
			name: "success writes handler response",
			handler: func(w http.ResponseWriter, _ *http.Request) error {
				JSON(w, http.StatusOK, map[string]string{"status": "ok"})
				return nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"status":"ok"}`,
		},
		{
			name: "api error uses its status and code",
			handler: func(http.ResponseWriter, *http.Request) error {
				return &Error{Status: http.StatusGone, Code: "invite_expired", Message: "Kode undangan sudah kedaluwarsa"}
			},
			wantStatus: http.StatusGone,
			wantBody:   `{"error":{"code":"invite_expired","message":"Kode undangan sudah kedaluwarsa"}}`,
		},
		{
			name: "wrapped api error is unwrapped",
			handler: func(http.ResponseWriter, *http.Request) error {
				return fmt.Errorf("accept invite: %w", &Error{Status: http.StatusConflict, Code: "conflict", Message: "Sudah menjadi anggota"})
			},
			wantStatus: http.StatusConflict,
			wantBody:   `{"error":{"code":"conflict","message":"Sudah menjadi anggota"}}`,
		},
		{
			name: "validation error includes fields",
			handler: func(http.ResponseWriter, *http.Request) error {
				return &Error{
					Status:  http.StatusBadRequest,
					Code:    "validation_failed",
					Message: "Input tidak valid",
					Fields:  map[string]string{"display_name": "Minimal 3 karakter"},
				}
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":{"code":"validation_failed","message":"Input tidak valid","fields":{"display_name":"Minimal 3 karakter"}}}`,
		},
		{
			name: "unknown error becomes 500 and is logged",
			handler: func(http.ResponseWriter, *http.Request) error {
				return errors.New("db exploded")
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   `{"error":{"code":"internal_error","message":"Terjadi kesalahan pada server. Coba lagi nanti."}}`,
			wantLogged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logBuf bytes.Buffer
			log := slog.New(slog.NewTextHandler(&logBuf, nil))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", http.NoBody)

			Handle(log, tt.handler)(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			assertJSONEqual(t, rec.Body.Bytes(), tt.wantBody)

			logged := strings.Contains(logBuf.String(), "db exploded")
			if logged != tt.wantLogged {
				t.Errorf("logged = %v, want %v (log: %q)", logged, tt.wantLogged, logBuf.String())
			}
		})
	}
}

func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("response is not JSON: %v (%q)", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad want JSON: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("body = %s, want %s", got, want)
	}
}
