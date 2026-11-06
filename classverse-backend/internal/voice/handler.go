package voice

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

var errUnavailable = &httpx.Error{
	Status: http.StatusServiceUnavailable, Code: "voice_unavailable", Message: "Suara belum tersedia di server ini",
}

// Handler serves POST /api/rooms/{room_id}/voice-token.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes mounts the endpoint; callers must be authenticated.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/rooms/{room_id}/voice-token", httpx.Handle(h.log, h.token))
}

func (h *Handler) token(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "room_id"))
	if err != nil {
		return httpx.ErrNotFound
	}
	t, err := h.svc.Token(r.Context(), roomID, userID)
	switch {
	case errors.Is(err, ErrDisabled):
		return errUnavailable
	case errors.Is(err, ErrPending):
		return &httpx.Error{Status: http.StatusConflict, Code: "voice_pending", Message: "Pemutusan suara sebelumnya belum terkonfirmasi"}
	case errors.Is(err, ErrNotPresent):
		// Same answer for missing rooms, other orgs and not being inside.
		return httpx.ErrNotFound
	case err != nil:
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, api.VoiceToken{Url: t.URL, Token: t.Token, BroadcastToken: t.BroadcastToken, Identity: t.Identity, Channel: t.Channel})
	return nil
}
