package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

var trustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "fc00::/7",
}

type pinger interface {
	Ping(ctx context.Context) error
}

func newRouter(log *slog.Logger, db pinger, api, ws func(chi.Router)) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// Replaces the deprecated middleware.RealIP, which trusts client-supplied
	// headers. X-Forwarded-For is honored only from proxies on loopback or
	// private networks (Vite dev proxy, Caddy on the same VPS or Docker
	// network). Read the result with middleware.GetClientIP.
	r.Use(middleware.ClientIPFromXFF(trustedProxies...))
	r.Use(httpx.RequestLogger(log))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", httpx.Handle(log, func(w http.ResponseWriter, r *http.Request) error {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			log.WarnContext(ctx, "readiness check failed", slog.Any("err", fmt.Errorf("ping database: %w", err)))
			return &httpx.Error{Status: http.StatusServiceUnavailable, Code: "unavailable", Message: "Database belum siap"}
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return nil
	}))

	if api != nil {
		r.Route("/api", api)
	}
	if ws != nil {
		r.Group(ws)
	}
	return r
}
