package module

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

// Handler serves module endpoints; mount inside RequireUser.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes mounts module endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/modules", h.handle(h.list))
	r.Post("/orgs/{org_id}/modules", h.handle(h.create))
	r.Put("/orgs/{org_id}/modules/{module_id}", h.handle(h.update))
	r.Delete("/orgs/{org_id}/modules/{module_id}", h.handle(h.archive))
	r.Post("/orgs/{org_id}/modules/{module_id}/steps", h.handle(h.addStep))
	r.Put("/orgs/{org_id}/modules/{module_id}/steps/{step_id}", h.handle(h.updateStep))
	r.Delete("/orgs/{org_id}/modules/{module_id}/steps/{step_id}", h.handle(h.archiveStep))
	r.Put("/orgs/{org_id}/modules/{module_id}/order", h.handle(h.reorder))
}

func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		err := fn(w, r)
		var verr *ValidationError
		switch {
		case err == nil:
			return nil
		case errors.As(err, &verr):
			return httpx.ValidationFailed(verr.Fields)
		case errors.Is(err, ErrNotFound):
			return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Modul atau langkah tidak ditemukan"}
		case errors.Is(err, ErrStepLimit):
			return httpx.ValidationFailed(map[string]string{"steps": fmt.Sprintf("Maksimal %d langkah per modul", maxSteps)})
		case errors.Is(err, ErrBadOrdering):
			return httpx.ValidationFailed(map[string]string{"step_ids": "Urutan harus memuat semua langkah tepat sekali"})
		}
		var apiErr *httpx.Error
		if mapped := org.ToAPIError(err); errors.As(mapped, &apiErr) {
			return mapped
		}
		return fmt.Errorf("module: %w", err)
	})
}

// StepResponse is the public view of a step.
type StepResponse struct {
	ID             uuid.UUID  `json:"id"`
	Kind           string     `json:"kind"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	FileID         *uuid.UUID `json:"file_id"`
	FileKind       *string    `json:"file_kind"`
	VideoID        *string    `json:"video_id"`
	Minutes        int32      `json:"minutes"`
	ReaderPosition int32      `json:"reader_position"`
}

// Response is the public view of a module with its steps in order.
type Response struct {
	ID          uuid.UUID      `json:"id"`
	BuildingKey string         `json:"building_key"`
	Level       string         `json:"level"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Published   bool           `json:"published"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Steps       []StepResponse `json:"steps"`
}

func toStep(s Step) StepResponse {
	out := StepResponse{ID: s.ID, Kind: s.Kind, Title: s.Title, Body: s.Body, Minutes: s.Minutes, ReaderPosition: s.ReaderPosition}
	if s.FileID.Valid {
		out.FileID = &s.FileID.UUID
	}
	if s.FileKind != "" {
		out.FileKind = &s.FileKind
	}
	if s.VideoID.Valid {
		out.VideoID = &s.VideoID.String
	}
	return out
}

func toModule(m Module) Response {
	steps := make([]StepResponse, 0, len(m.Steps))
	for _, s := range m.Steps {
		steps = append(steps, toStep(s))
	}
	return Response{
		ID: m.ID, BuildingKey: m.BuildingKey, Level: m.Level, Title: m.Title, Description: m.Description,
		Published: m.PublishedAt.Valid, UpdatedAt: m.UpdatedAt.Time, Steps: steps,
	}
}

type moduleRequest struct {
	BuildingKey string `json:"building_key"`
	Level       string `json:"level"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Published   bool   `json:"published"`
}

type stepRequest struct {
	Kind    string     `json:"kind"`
	Title   string     `json:"title"`
	Body    string     `json:"body"`
	FileID  *uuid.UUID `json:"file_id"`
	VideoID string     `json:"video_id"`
	Minutes int        `json:"minutes"`
}

// ids reads path UUIDs in order; malformed IDs are treated as not found (org.PathUUID).
func ids(r *http.Request, names ...string) (uuid.UUID, []uuid.UUID, error) {
	user, _ := auth.UserID(r.Context())
	out := make([]uuid.UUID, 0, len(names))
	for _, n := range names {
		id, err := org.PathUUID(r, n)
		if err != nil {
			return user, nil, err
		}
		out = append(out, id)
	}
	return user, out, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	mods, err := h.svc.List(r.Context(), user, p[0], Filter{BuildingKey: q.Get("building_key"), Level: q.Get("level"), Drafts: q.Get("drafts") == "true"})
	if err != nil {
		return err
	}
	items := make([]Response, 0, len(mods))
	for _, m := range mods {
		items = append(items, toModule(m))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	var req moduleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	m, err := h.svc.Create(r.Context(), user, p[0], Input(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toModule(Module{Module: m}))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id")
	if err != nil {
		return err
	}
	var req moduleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	m, err := h.svc.Update(r.Context(), user, p[0], p[1], Input(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toModule(Module{Module: m}))
	return nil
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id")
	if err != nil {
		return err
	}
	if err := h.svc.Archive(r.Context(), user, p[0], p[1]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) addStep(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id")
	if err != nil {
		return err
	}
	var req stepRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	s, err := h.svc.AddStep(r.Context(), user, p[0], p[1], StepInput(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toStep(Step{ModuleStep: s}))
	return nil
}

func (h *Handler) updateStep(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id", "step_id")
	if err != nil {
		return err
	}
	var req stepRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	s, err := h.svc.UpdateStep(r.Context(), user, p[0], p[1], p[2], StepInput(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toStep(Step{ModuleStep: s}))
	return nil
}

func (h *Handler) archiveStep(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id", "step_id")
	if err != nil {
		return err
	}
	if err := h.svc.ArchiveStep(r.Context(), user, p[0], p[1], p[2]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) reorder(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id")
	if err != nil {
		return err
	}
	var req struct {
		StepIDs []uuid.UUID `json:"step_ids"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.svc.Reorder(r.Context(), user, p[0], p[1], req.StepIDs); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
