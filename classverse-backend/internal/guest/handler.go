package guest

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves host and guest HTTP endpoints.
type Handler struct {
	svc         *Service
	frontendURL string
	log         *slog.Logger
	previewRate *ratelimit.Limiter
	joinRate    *ratelimit.Limiter
	pollRate    *ratelimit.Limiter
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, frontendURL string, now func() time.Time, log *slog.Logger) *Handler {
	return &Handler{svc: svc, frontendURL: frontendURL, log: log,
		previewRate: ratelimit.New(30, time.Minute, now), joinRate: ratelimit.New(10, time.Minute, now),
		pollRate: ratelimit.New(30, time.Minute, now)}
}

// PublicRoutes mounts anonymous link preview and join request endpoints.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/guest/links/{code}", h.handle(h.preview))
	r.Post("/guest/links/{code}/requests", h.handle(h.request))
}

// GuestRoutes mounts guest-only endpoints; use auth.RequireGuest.
func (h *Handler) GuestRoutes(r chi.Router) {
	r.Get("/guest/me", h.handle(h.status))
}

// HostRoutes mounts staff endpoints; use auth.RequireUser.
func (h *Handler) HostRoutes(r chi.Router) {
	r.Get("/rooms/{room_id}/guest-links", h.handle(h.links))
	r.Post("/rooms/{room_id}/guest-links", h.handle(h.create))
	r.Delete("/rooms/{room_id}/guest-links/{link_id}", h.handle(h.revoke))
	r.Get("/rooms/{room_id}/guest-requests", h.handle(h.requests))
	r.Post("/rooms/{room_id}/guest-requests/{request_id}", h.handle(h.decide))
}

func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		return toAPIError(fn(w, r))
	})
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) error {
	if err := httpx.RateLimit(w, h.previewRate, clientIP(r)); err != nil {
		return err
	}
	p, err := h.svc.Preview(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, api.GuestLinkPreview{OrgName: p.OrgName, RoomName: p.RoomName})
	return nil
}

func (h *Handler) request(w http.ResponseWriter, r *http.Request) error {
	if err := httpx.RateLimit(w, h.joinRate, clientIP(r)); err != nil {
		return err
	}
	var in api.RequestGuestAccessJSONBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	s, err := h.svc.Request(r.Context(), chi.URLParam(r, "code"), in.DisplayName, in.RequestKey)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, sessionJSON(s))
	return nil
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) error {
	id, _ := auth.GuestID(r.Context())
	if err := httpx.RateLimit(w, h.pollRate, id.String()); err != nil {
		return err
	}
	s, err := h.svc.Status(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, sessionJSON(s))
	return nil
}

func (h *Handler) links(w http.ResponseWriter, r *http.Request) error {
	id, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	ls, err := h.svc.Links(r.Context(), id, roomID)
	if err != nil {
		return err
	}
	out := make([]api.GuestLink, 0, len(ls))
	for _, l := range ls {
		out = append(out, h.linkJSON(l))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	id, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	var in api.CreateGuestLinkRequest
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	l, err := h.svc.CreateLink(r.Context(), id, roomID, in.ExpiresInHours, in.MaxGuests)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, h.linkJSON(l))
	return nil
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) error {
	id, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	linkID, err := org.PathUUID(r, "link_id")
	if err != nil {
		return err
	}
	if err := h.svc.RevokeLink(r.Context(), id, roomID, linkID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) requests(w http.ResponseWriter, r *http.Request) error {
	id, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	rs, err := h.svc.Requests(r.Context(), id, roomID)
	if err != nil {
		return err
	}
	out := make([]api.GuestRequest, 0, len(rs))
	for _, rq := range rs {
		out = append(out, requestJSON(rq))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": out})
	return nil
}

func (h *Handler) decide(w http.ResponseWriter, r *http.Request) error {
	id, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	rqID, err := org.PathUUID(r, "request_id")
	if err != nil {
		return err
	}
	var in api.DecideGuestRequestJSONBody
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	if !in.Decision.Valid() {
		return httpx.ValidationFailed(map[string]string{"decision": "Pilih izinkan atau tolak"})
	}
	rq, err := h.svc.Decide(r.Context(), id, roomID, rqID, in.Decision == api.Approve)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, requestJSON(rq))
	return nil
}

func (h *Handler) linkJSON(l Link) api.GuestLink {
	return api.GuestLink{Id: l.ID, Code: l.Code, Link: h.frontendURL + "/guest/" + l.Code, MaxGuests: l.MaxGuests,
		UsedCount: l.UsedCount, ExpiresAt: l.ExpiresAt, CreatedAt: l.CreatedAt}
}

func requestJSON(r Request) api.GuestRequest {
	return api.GuestRequest{Id: r.ID, UserId: r.UserID, DisplayName: r.Name, Status: api.GuestRequestStatus(r.Status), CreatedAt: r.CreatedAt}
}

func sessionJSON(s Status) api.GuestSession {
	return api.GuestSession{Token: s.Token, Status: api.GuestSessionStatus(s.Status), DisplayName: s.Name,
		RoomId: s.RoomID, RoomName: s.RoomName, MapKey: api.RoomMap(s.MapKey), OrgName: s.OrgName}
}

func clientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func toAPIError(err error) error {
	var v *ValidationError
	switch {
	case errors.As(err, &v):
		return httpx.ValidationFailed(v.Fields)
	case errors.Is(err, ErrNotFound), errors.Is(err, org.ErrNotMember):
		return httpx.ErrNotFound
	case errors.Is(err, org.ErrForbidden):
		return httpx.ErrForbidden
	case errors.Is(err, ErrKicked):
		return &httpx.Error{Status: http.StatusForbidden, Code: "forbidden", Message: "Kamu dikeluarkan dari sesi ini"}
	case errors.Is(err, ErrExpired):
		return &httpx.Error{Status: http.StatusGone, Code: "guest_expired", Message: "Link tamu sudah berakhir, dicabut, atau kuotanya habis"}
	default:
		return err
	}
}
