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
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/invite"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/config"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/logx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/mail"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/realtime"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/report"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/room"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/voice"
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

	a, err := newApp(ctx, cfg, pool, log)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           newRouter(log, pool, a.api, a.ws),
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
	// Shutdown does not track hijacked WebSocket connections; the hub closes
	// them (1001) and stops the rooms.
	if err := a.hub.Shutdown(shutdownCtx); err != nil {
		log.Warn("realtime shutdown incomplete", slog.Any("err", err))
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	log.Info("server stopped")
	return nil
}

// app holds the wired features: the /api and /ws route trees and the
// realtime hub that must be shut down explicitly.
type app struct {
	api func(chi.Router)
	ws  func(chi.Router)
	hub *realtime.Hub
}

// campusBuildings adapts the campus map to room.Buildings.
type campusBuildings struct{ m *realtime.MapData }

func (c campusBuildings) Building(key string) (staffOnly, ok bool) {
	b, ok := c.m.Building(key)
	return b.StaffOnly, ok
}

// roomOccupancy adapts the hub to room.Occupancy, translating the hub's busy
// error into the room domain error so the API answers 409, not 500.
type roomOccupancy struct{ *realtime.Hub }

func (o roomOccupancy) WhileEmpty(ctx context.Context, roomID uuid.UUID, fn func() error) error {
	err := o.Hub.WhileEmpty(ctx, roomID, fn)
	if errors.Is(err, realtime.ErrRoomBusy) {
		return room.ErrBusy
	}
	return err
}

// voicePresence adapts the hub to voice.Presence.
type voicePresence struct{ *realtime.Hub }

func (p voicePresence) VoiceMember(ctx context.Context, roomID, userID uuid.UUID) (voice.Member, error) {
	m, err := p.Hub.VoiceMember(ctx, roomID, userID)
	if errors.Is(err, realtime.ErrVoicePending) {
		return voice.Member{}, voice.ErrPending
	}
	if errors.Is(err, realtime.ErrNotInRoom) {
		return voice.Member{}, voice.ErrNotPresent
	}
	return voice.Member{Name: m.Name, Muted: m.Muted, Identity: m.Identity, Channel: m.Channel, Broadcasting: m.Broadcasting}, err
}

// newApp wires every feature.
func newApp(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (*app, error) {
	now := func() time.Time { return time.Now().UTC() }

	orgSvc := org.NewService(org.NewStore(pool), now, log)
	inviteSvc := invite.NewService(invite.NewStore(pool), orgSvc, now, log)

	authHandler, tokens, err := newAuth(ctx, cfg, pool, orgSvc, now, log)
	if err != nil {
		return nil, err
	}
	orgHandler := org.NewHandler(orgSvc, log)
	inviteHandler := invite.NewHandler(inviteSvc, cfg.FrontendURL, now, log)
	mapHandler := realtime.NewMapHandler(log)

	mapData, err := realtime.LoadMaps("campus", "classroom", "garden", "lab", "baca")
	if err != nil {
		return nil, fmt.Errorf("load maps: %w", err)
	}
	rtStore := realtime.NewStore(pool)
	// Sessions left open by a crash cannot be resumed; end them before serving.
	recovered, err := rtStore.RecoverSessions(ctx, now())
	if err != nil {
		return nil, err
	}
	hub := realtime.NewHub(mapData, rtStore, now, log)
	if err := hub.RestoreSessions(recovered); err != nil {
		_ = hub.Shutdown(ctx)
		return nil, err
	}
	go runChatRetention(ctx, rtStore, now, purgeInterval, log)
	authHandler.SetProfilePublisher(hub)
	gateway, err := realtime.NewGateway(hub, tokens, rtStore, cfg.FrontendURL, log)
	if err != nil {
		return nil, err
	}
	roomSvc := room.NewService(room.NewStore(pool), orgSvc, campusBuildings{mapData["campus"]}, roomOccupancy{hub}, now, log)
	roomHandler := room.NewHandler(roomSvc, log)
	reportHandler := report.NewHandler(report.NewService(report.NewStore(pool), orgSvc, now), log)
	lk, err := voice.NewLiveKit(cfg.LiveKit.URL, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, now)
	if err != nil {
		_ = hub.Shutdown(ctx)
		return nil, err
	}
	var issuer voice.Issuer
	if lk != nil {
		hub.SetVoice(lk)
		issuer = lk
	} else {
		log.Info("voice disabled: LIVEKIT_URL not set")
	}
	signalURL := "ws" + strings.TrimPrefix(cfg.FrontendURL, "http") + "/voice"
	voiceHandler := voice.NewHandler(voice.NewService(voicePresence{hub}, issuer, signalURL), log)
	var signaling http.Handler
	if lk != nil {
		signaling, err = voice.NewSignaling(lk, voicePresence{hub}, cfg.FrontendURL)
		if err != nil {
			_ = hub.Shutdown(ctx)
			return nil, err
		}
	}

	return &app{
		api: func(r chi.Router) {
			authHandler.Routes(r)
			mapHandler.Routes(r)
			r.Group(func(r chi.Router) {
				r.Use(auth.RequireUser(tokens, log))
				orgHandler.Routes(r)
				inviteHandler.Routes(r)
				roomHandler.Routes(r)
				reportHandler.Routes(r)
				voiceHandler.Routes(r)
			})
		},
		ws: func(r chi.Router) {
			gateway.Routes(r)
			if signaling != nil {
				r.Handle("/voice/*", signaling)
			}
		},
		hub: hub,
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
