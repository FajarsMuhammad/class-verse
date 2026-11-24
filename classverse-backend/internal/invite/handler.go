package invite

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves invite endpoints.
type Handler struct {
	svc         *Service
	frontendURL string
	log         *slog.Logger
	codeLimit   *ratelimit.Limiter
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, frontendURL string, now func() time.Time, log *slog.Logger) *Handler {
	return &Handler{
		svc: svc, frontendURL: frontendURL, log: log,
		// Preview and accept share one budget so codes cannot be guessed quickly.
		codeLimit: ratelimit.New(10, time.Minute, now),
	}
}

// Routes mounts invite endpoints; callers must be authenticated.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/orgs/{org_id}/invites", h.handle(h.create))
	r.Get("/orgs/{org_id}/invites", h.handle(h.list))
	r.Delete("/orgs/{org_id}/invites/{invite_id}", h.handle(h.revoke))
	r.Get("/invites/{code}", h.handle(h.preview))
	r.Post("/invites/{code}/accept", h.handle(h.accept))
}

func (h *Handler) handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		return toAPIError(fn(w, r))
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	var req api.CreateInviteRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	inv, err := h.svc.Create(r.Context(), userID, orgID, CreateInput{
		Role: org.Role(req.Role), ExpiresInDays: req.ExpiresInDays, MaxUses: req.MaxUses, ClassID: req.ClassId,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, h.toAPI(inv))
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if limit, err = strconv.Atoi(v); err != nil || limit < 1 {
			return httpx.ValidationFailed(map[string]string{"limit": "limit harus 1–100"})
		}
	}
	var classID *uuid.UUID
	if raw := r.URL.Query().Get("class_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return httpx.ValidationFailed(map[string]string{"class_id": "ID kelas tidak valid"})
		}
		classID = &id
	}
	page, err := h.svc.List(r.Context(), userID, orgID, limit, r.URL.Query().Get("cursor"), classID)
	if err != nil {
		return err
	}
	resp := api.InviteList{Items: make([]api.Invite, 0, len(page.Items))}
	for _, inv := range page.Items {
		resp.Items = append(resp.Items, h.toAPI(inv))
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	inviteID, err := org.PathUUID(r, "invite_id")
	if err != nil {
		return err
	}
	if err := h.svc.Revoke(r.Context(), userID, orgID, inviteID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	if err := httpx.RateLimit(w, h.codeLimit, userID.String()); err != nil {
		return err
	}
	p, err := h.svc.Preview(r.Context(), chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	resp := api.InvitePreview{OrgName: p.OrgName, Role: api.InviteRole(p.Role)}
	if p.ClassName != "" {
		resp.ClassName = &p.ClassName
		resp.YearLabel = &p.YearLabel
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	if err := httpx.RateLimit(w, h.codeLimit, userID.String()); err != nil {
		return err
	}
	m, err := h.svc.Accept(r.Context(), userID, chi.URLParam(r, "code"))
	if err != nil {
		return err
	}
	resp := api.Membership{OrgId: m.OrgID, UserId: m.UserID, Role: api.Role(m.Role), JoinedAt: m.JoinedAt.UTC()}
	if m.PlacementStatus != "" {
		status := api.MembershipPlacementStatus(m.PlacementStatus)
		resp.PlacementStatus = &status
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) toAPI(inv Invite) api.Invite {
	return api.Invite{
		Id: inv.ID, ClassId: inv.ClassID, Code: inv.Code, Link: h.frontendURL + "/gabung/" + inv.Code,
		Role: api.InviteRole(inv.Role), MaxUses: inv.MaxUses, UsedCount: inv.UsedCount,
		ExpiresAt: inv.ExpiresAt.UTC(), CreatedBy: inv.CreatedBy, CreatedAt: inv.CreatedAt.UTC(),
	}
}

func toAPIError(err error) error {
	if err == nil {
		return nil
	}
	var verr *ValidationError
	switch {
	case errors.As(err, &verr):
		return httpx.ValidationFailed(verr.Fields)
	case errors.Is(err, ErrNotFound):
		return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Kode undangan tidak ditemukan"}
	case errors.Is(err, ErrExpired):
		return &httpx.Error{Status: http.StatusGone, Code: "invite_expired", Message: "Kode undangan sudah tidak berlaku"}
	case errors.Is(err, ErrAlreadyMember):
		return &httpx.Error{Status: http.StatusConflict, Code: "conflict", Message: "Kamu sudah menjadi anggota sekolah ini"}
	case errors.Is(err, ErrProfileIncomplete):
		return &httpx.Error{Status: http.StatusForbidden, Code: "profile_incomplete", Message: "Lengkapi profil terlebih dahulu"}
	case errors.Is(err, ErrForbidden):
		return httpx.ErrForbidden
	case errors.Is(err, ErrInvalidCursor):
		return httpx.ValidationFailed(map[string]string{"cursor": "Cursor tidak valid"})
	}
	if mapped := org.ToAPIError(err); !isWrapped(mapped) {
		return mapped
	}
	return fmt.Errorf("invite: %w", err)
}

// isWrapped reports whether org.ToAPIError left err unmapped.
func isWrapped(err error) bool {
	var apiErr *httpx.Error
	return !errors.As(err, &apiErr)
}
