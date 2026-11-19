package module

import (
	"encoding/json"
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
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves module endpoints; mount inside RequireUser.
type Handler struct {
	svc    *Service
	log    *slog.Logger
	reads  *ratelimit.Limiter
	writes *ratelimit.Limiter
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, reads: ratelimit.New(600, time.Minute, svc.now), writes: ratelimit.New(120, time.Minute, svc.now)}
}

// Routes mounts module endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/modules", h.handle(h.list))
	r.Post("/orgs/{org_id}/modules", h.handle(h.create))
	r.Post("/orgs/{org_id}/modules/{module_id}/publish", h.handle(h.publish))
	r.Get("/orgs/{org_id}/modules/{module_id}/revisions", h.handle(h.history))
	r.Put("/orgs/{org_id}/modules/{module_id}", h.handle(h.update))
	r.Delete("/orgs/{org_id}/modules/{module_id}", h.handle(h.archive))
	r.Post("/orgs/{org_id}/modules/{module_id}/steps", h.handle(h.addStep))
	r.Put("/orgs/{org_id}/modules/{module_id}/steps/{step_id}", h.handle(h.updateStep))
	r.Delete("/orgs/{org_id}/modules/{module_id}/steps/{step_id}", h.handle(h.archiveStep))
	r.Put("/orgs/{org_id}/modules/{module_id}/order", h.handle(h.reorder))
	r.Put("/orgs/{org_id}/modules/{module_id}/steps/{step_id}/done", h.handle(h.done(true)))
	r.Delete("/orgs/{org_id}/modules/{module_id}/steps/{step_id}/done", h.handle(h.done(false)))
	r.Get("/orgs/{org_id}/modules/resume", h.handle(h.resume))
}

func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		user, ok := auth.UserID(r.Context())
		if !ok {
			return &httpx.Error{Status: 401, Code: "unauthenticated", Message: "Silakan masuk"}
		}
		limit := h.writes
		if r.Method == http.MethodGet {
			limit = h.reads
		}
		if err := httpx.RateLimit(w, limit, user.String()); err != nil {
			return err
		}
		err := fn(w, r)
		var verr *ValidationError
		switch {
		case err == nil:
			return nil
		case errors.As(err, &verr):
			return httpx.ValidationFailed(verr.Fields)
		case errors.Is(err, ErrConflict):
			return &httpx.Error{Status: 409, Code: "draft_conflict", Message: "Draf berubah di tab lain. Muat versi server atau salin perubahan lokal."}
		case errors.Is(err, ErrNotFound):
			return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Modul atau langkah tidak ditemukan"}
		case errors.Is(err, ErrStepLimit):
			return httpx.ValidationFailed(map[string]string{"steps": fmt.Sprintf("Maksimal %d langkah per modul", maxSteps)})
		case errors.Is(err, ErrQuizStep):
			return &httpx.Error{Status: http.StatusConflict, Code: "quiz_step", Message: "Langkah kuis selesai otomatis saat kamu lulus kuisnya"}
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
	RichContent    json.RawMessage `json:"rich_content"`
	ID             uuid.UUID       `json:"id"`
	Kind           string          `json:"kind"`
	Title          string          `json:"title"`
	Body           string          `json:"body"`
	FileID         *uuid.UUID      `json:"file_id"`
	FileKind       *string         `json:"file_kind"`
	VideoID        *string         `json:"video_id"`
	QuizID         *uuid.UUID      `json:"quiz_id"`
	Minutes        int32           `json:"minutes"`
	ReaderPosition int32           `json:"reader_position"`
	Done           bool            `json:"done"`
}

// Response is the public view of a module with its steps in order.
type Response struct {
	DraftVersion        int64          `json:"draft_version"`
	PublishedRevisionID *uuid.UUID     `json:"published_revision_id"`
	ID                  uuid.UUID      `json:"id"`
	BuildingKey         string         `json:"building_key"`
	Level               string         `json:"level"`
	Title               string         `json:"title"`
	Description         string         `json:"description"`
	Published           bool           `json:"published"`
	UpdatedAt           time.Time      `json:"updated_at"`
	Steps               []StepResponse `json:"steps"`
}

func toStep(s Step) StepResponse {
	out := StepResponse{RichContent: s.RichContent, ID: s.ID, Kind: s.Kind, Title: s.Title, Body: s.Body, Minutes: s.Minutes, ReaderPosition: s.ReaderPosition, Done: s.Done}
	if s.FileID.Valid {
		out.FileID = &s.FileID.UUID
	}
	if s.FileKind != "" {
		out.FileKind = &s.FileKind
	}
	if s.VideoID.Valid {
		out.VideoID = &s.VideoID.String
	}
	if s.QuizID.Valid {
		out.QuizID = &s.QuizID.UUID
	}
	return out
}

func toModule(m Module) Response {
	steps := make([]StepResponse, 0, len(m.Steps))
	for _, s := range m.Steps {
		steps = append(steps, toStep(s))
	}
	return Response{
		DraftVersion: m.DraftVersion, PublishedRevisionID: func() *uuid.UUID {
			if m.PublishedRevisionID.Valid {
				return &m.PublishedRevisionID.UUID
			}
			return nil
		}(), ID: m.ID, BuildingKey: m.BuildingKey, Level: m.Level, Title: m.Title, Description: m.Description,
		Published: m.PublishedAt.Valid, UpdatedAt: m.UpdatedAt.Time, Steps: steps,
	}
}

type moduleRequest struct {
	ExpectedVersion int64  `json:"expected_version"`
	BuildingKey     string `json:"building_key"`
	Level           string `json:"level"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Published       bool   `json:"published"`
}

type stepRequest struct {
	ExpectedVersion int64           `json:"expected_version"`
	RichContent     json.RawMessage `json:"rich_content"`
	Kind            string          `json:"kind"`
	Title           string          `json:"title"`
	Body            string          `json:"body"`
	FileID          *uuid.UUID      `json:"file_id"`
	VideoID         string          `json:"video_id"`
	QuizID          *uuid.UUID      `json:"quiz_id"`
	Minutes         int             `json:"minutes"`
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
	offset := int64(0)
	if raw := q.Get("offset"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 32)
		if parseErr != nil || parsed < 0 {
			return httpx.ValidationFailed(map[string]string{"offset": "Offset tidak valid"})
		}
		offset = parsed
	}
	limit := int64(100)
	if raw := q.Get("limit"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 32)
		if parseErr != nil || parsed < 1 || parsed > 100 {
			return httpx.ValidationFailed(map[string]string{"limit": "Limit harus 1–100"})
		}
		limit = parsed
	}
	if status := q.Get("status"); status != "" && status != "draft" && status != "published" {
		return httpx.ValidationFailed(map[string]string{"status": "Status tidak valid"})
	}
	mods, err := h.svc.List(r.Context(), user, p[0], Filter{Limit: int32(limit), Search: q.Get("search"), Status: q.Get("status"), Offset: int32(offset), BuildingKey: q.Get("building_key"), Level: q.Get("level"), Drafts: q.Get("drafts") == "true", Archived: q.Get("archived") == "true"})
	if err != nil {
		return err
	}
	items := make([]Response, 0, len(mods))
	for _, m := range mods {
		items = append(items, toModule(m))
	}
	total := int64(0)
	if len(mods) > 0 {
		total = mods[0].TotalCount
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
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
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
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
	if err := httpx.DecodeJSONLimit(r, &req, maxRichBytes+16*1024); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
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
	if err := httpx.DecodeJSONLimit(r, &req, maxRichBytes+16*1024); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
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
	version, err := strconv.ParseInt(r.URL.Query().Get("expected_version"), 10, 64)
	if err != nil || version < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	if err := h.svc.ArchiveStep(r.Context(), user, p[0], p[1], p[2], version); err != nil {
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
		StepIDs         []uuid.UUID `json:"step_ids"`
		ExpectedVersion int64       `json:"expected_version"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.ExpectedVersion < 1 {
		return httpx.ValidationFailed(map[string]string{"expected_version": "Versi draf wajib"})
	}
	if err := h.svc.Reorder(r.Context(), user, p[0], p[1], req.StepIDs, req.ExpectedVersion); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) done(done bool) func(http.ResponseWriter, *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		user, p, err := ids(r, "org_id", "module_id", "step_id")
		if err != nil {
			return err
		}
		if err := h.svc.SetDone(r.Context(), user, p[0], p[1], p[2], done); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
}

// ResumeResponse is the "Lanjutkan belajar" card; null when there is nothing to continue.
type ResumeResponse struct {
	ModuleID    uuid.UUID `json:"module_id"`
	BuildingKey string    `json:"building_key"`
	Level       string    `json:"level"`
	Title       string    `json:"title"`
	Done        int32     `json:"done"`
	Total       int32     `json:"total"`
	NextTitle   string    `json:"next_title"`
}

func (h *Handler) resume(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	res, err := h.svc.ResumeFor(r.Context(), user, p[0])
	if err != nil {
		return err
	}
	var out *ResumeResponse
	if res != nil {
		out = &ResumeResponse{ModuleID: res.ModuleID, BuildingKey: res.BuildingKey, Level: res.Level, Title: res.Title,
			Done: res.Done, Total: res.Total, NextTitle: res.NextTitle}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"resume": out})
	return nil
}

func (h *Handler) publish(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id")
	if err != nil {
		return err
	}
	var req struct {
		ExpectedVersion int64 `json:"expected_version"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	m, err := h.svc.Publish(r.Context(), user, p[0], p[1], req.ExpectedVersion)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, toModule(Module{Module: m}))
	return nil
}
func (h *Handler) history(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "module_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.History(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row.ID, "revision": row.Revision, "draft_version": row.DraftVersion, "published_at": row.PublishedAt.Time})
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
	return nil
}
