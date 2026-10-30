package org

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

// Handler serves /api/orgs.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the organization endpoints; callers must already be
// authenticated (auth.RequireUser).
func (h *Handler) Routes(r chi.Router) {
	r.Post("/orgs", h.handle(h.create))
	r.Get("/orgs/{org_id}", h.handle(h.get))
}

func (h *Handler) handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		return ToAPIError(fn(w, r))
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	var req api.CreateOrgRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	o, err := h.svc.Create(r.Context(), userID, CreateInput{Name: req.Name, Slug: req.Slug, LogoURL: req.LogoUrl})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, ToAPI(o))
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	o, err := h.svc.Get(r.Context(), userID, orgID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, ToAPI(o))
	return nil
}

// ToAPI converts an organization to the API model.
func ToAPI(o Organization) api.Organization {
	return api.Organization{
		Id: o.ID, Name: o.Name, Slug: o.Slug, LogoUrl: o.LogoURL,
		Role: api.Role(o.Role), CampusRoomId: o.CampusRoomID, CreatedAt: o.CreatedAt.UTC(),
	}
}

// PathUUID parses a UUID path parameter; malformed ids are reported as 404.
func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	return id, nil
}

// ToAPIError maps organization domain errors to API errors. Other features
// use it for errors returned by Authorize.
func ToAPIError(err error) error {
	if err == nil {
		return nil
	}
	var verr *ValidationError
	switch {
	case errors.As(err, &verr):
		return httpx.ValidationFailed(verr.Fields)
	case errors.Is(err, ErrNotMember):
		return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Organisasi tidak ditemukan"}
	case errors.Is(err, ErrForbidden):
		return httpx.ErrForbidden
	case errors.Is(err, ErrSlugTaken):
		return &httpx.Error{Status: http.StatusConflict, Code: "conflict", Message: "Slug sudah dipakai sekolah lain",
			Fields: map[string]string{"slug": "Slug sudah dipakai, pilih yang lain"}}
	case errors.Is(err, ErrNotAdult):
		return &httpx.Error{Status: http.StatusForbidden, Code: "forbidden",
			Message: "Membuat sekolah hanya untuk pengguna berusia 18 tahun ke atas. Minta kode undangan dari sekolahmu."}
	case errors.Is(err, ErrNotEligible):
		return &httpx.Error{Status: http.StatusForbidden, Code: "profile_incomplete", Message: "Lengkapi profil terlebih dahulu"}
	case errors.Is(err, ErrUserNotFound):
		return &httpx.Error{Status: http.StatusUnauthorized, Code: "unauthenticated", Message: "Sesi berakhir. Silakan masuk lagi."}
	}
	return fmt.Errorf("org: %w", err)
}
