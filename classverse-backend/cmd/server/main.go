// Command server runs the Kelasverse HTTP and WebSocket server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/invite"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/config"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/logx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/mail"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := logx.New(os.Stdout, cfg.IsProduction())

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	api, err := newAPI(ctx, cfg, pool, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newRouter(log, pool, api),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No global WriteTimeout: WebSocket connections are long-lived.
		// Limit write time per handler instead.
	}

	serveErr := make(chan error, 1)
	go func() {
		log.Info("server listening", slog.String("addr", cfg.HTTPAddr), slog.String("env", cfg.AppEnv))
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	log.Info("server stopped")
	return nil
}

// newAPI wires every feature and returns the /api route tree.
func newAPI(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (func(chi.Router), error) {
	now := func() time.Time { return time.Now().UTC() }

	orgSvc := org.NewService(org.NewStore(pool), now, log)
	inviteSvc := invite.NewService(invite.NewStore(pool), orgSvc, now, log)

	authHandler, tokens, err := newAuth(ctx, cfg, pool, orgSvc, now, log)
	if err != nil {
		return nil, err
	}
	orgHandler := org.NewHandler(orgSvc, log)
	inviteHandler := invite.NewHandler(inviteSvc, cfg.FrontendURL, now, log)

	return func(r chi.Router) {
		authHandler.Routes(r)
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireUser(tokens, log))
			orgHandler.Routes(r)
			inviteHandler.Routes(r)
		})
	}, nil
}

func newAuth(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, orgs auth.OrgLister, now func() time.Time, log *slog.Logger) (*auth.Handler, *auth.Tokens, error) {
	mailer, err := mail.NewSMTPSender(cfg.SMTPAddr, cfg.SMTPFrom)
	if err != nil {
		return nil, nil, err
	}
	tokens := auth.NewTokens(cfg.JWTPrivateKey, now)
	svc, err := auth.NewService(auth.NewStore(pool), tokens, mailer, cfg.OTPHMACKey, now, log)
	if err != nil {
		return nil, nil, err
	}
	var google auth.Google
	if cfg.Google.Enabled() {
		gc, err := auth.NewGoogleClientForGoogle(ctx, cfg.Google.ClientID, cfg.Google.ClientSecret, cfg.Google.RedirectURL)
		if err != nil {
			return nil, nil, fmt.Errorf("google login: %w", err)
		}
		google = gc
	} else {
		log.Warn("google login disabled: GOOGLE_* not set")
	}
	return auth.NewHandler(svc, tokens, google, orgs, cfg.FrontendURL, now, log), tokens, nil
}
