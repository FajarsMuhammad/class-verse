package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestRouterHealth(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		pingErr    error
		wantStatus int
		wantBody   string
	}{
		{"healthz ignores database", "/healthz", errors.New("down"), http.StatusOK, `{"status":"ok"}`},
		{"readyz ok", "/readyz", nil, http.StatusOK, `{"status":"ok"}`},
		{"readyz database down", "/readyz", errors.New("down"), http.StatusServiceUnavailable, `"code":"unavailable"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := slog.New(slog.NewTextHandler(io.Discard, nil))
			h := newRouter(log, fakePinger{err: tt.pingErr}, nil, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, http.NoBody))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
