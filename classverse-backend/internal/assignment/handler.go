package assignment

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves school-scoped assignments inside RequireUser.
type Handler struct {
	svc           *Service
	log           *slog.Logger
	reads, writes *ratelimit.Limiter
}

// NewHandler wires request limits and existing JSON error handling.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, reads: ratelimit.New(600, time.Minute, svc.now), writes: ratelimit.New(120, time.Minute, svc.now)}
}

// Routes mounts assignment endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/assignments", h.handle(h.list))
	r.Post("/orgs/{org_id}/assignments", h.handle(h.issue))
	r.Delete("/orgs/{org_id}/assignments/{assignment_id}", h.handle(h.cancel))
	r.Get("/orgs/{org_id}/assignments/{assignment_id}/steps", h.handle(h.steps))
	r.Get("/orgs/{org_id}/assignments/{assignment_id}/roster", h.handle(h.roster))
	r.Put("/orgs/{org_id}/assignments/{assignment_id}/steps/{step_id}/done", h.handle(h.complete))
}
func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		user, _ := auth.UserID(r.Context())
		limit := h.writes
		if r.Method == http.MethodGet {
			limit = h.reads
		}
		if err := httpx.RateLimit(w, limit, user.String()); err != nil {
			return err
		}
		err := fn(w, r)
		if err == nil {
			return nil
		}
		switch {
		case errors.Is(err, ErrNotFound):
			return &httpx.Error{Status: 404, Code: "not_found", Message: "Tugas atau konten tidak ditemukan"}
		case errors.Is(err, ErrInvalid):
			return httpx.ValidationFailed(map[string]string{"assignment": "ID tugas, kelas dan modul wajib diisi"})
		case errors.Is(err, ErrEmpty):
			return httpx.ValidationFailed(map[string]string{"assignment": "Pilih kelas berisi siswa dan modul berisi langkah"})
		case errors.Is(err, ErrLocked):
			return &httpx.Error{Status: 409, Code: "step_locked", Message: "Selesaikan langkah sebelumnya dulu"}
		case errors.Is(err, ErrCancelled):
			return &httpx.Error{Status: 409, Code: "assignment_cancelled", Message: "Tugas sudah dibatalkan"}
		case errors.Is(err, ErrConflict):
			return &httpx.Error{Status: 409, Code: "assignment_conflict", Message: "ID tugas sudah dipakai untuk tugas berbeda"}
		case errors.Is(err, ErrQuizStep):
			return &httpx.Error{Status: 409, Code: "quiz_step", Message: "Lulus kuis dari tugas ini untuk menyelesaikan langkah"}
		}
		return org.ToAPIError(err)
	})
}
func ids(r *http.Request, names ...string) (uuid.UUID, []uuid.UUID, error) {
	user, _ := auth.UserID(r.Context())
	out := make([]uuid.UUID, len(names))
	for i, n := range names {
		id, err := org.PathUUID(r, n)
		if err != nil {
			return user, nil, err
		}
		out[i] = id
	}
	return user, out, nil
}

// Response is a task summary, never a catalog revision.
type Response struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	ClassName   string     `json:"class_name"`
	YearLabel   string     `json:"year_label"`
	BuildingKey string     `json:"building_key"`
	Level       string     `json:"level"`
	DueAt       *time.Time `json:"due_at"`
	Cancelled   bool       `json:"cancelled"`
	Done        int32      `json:"done"`
	Total       int32      `json:"total"`
}

// StepResponse hides locked content and retains only navigational metadata.
type StepResponse struct {
	RichContent json.RawMessage `json:"rich_content"`
	ID          uuid.UUID       `json:"id"`
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	FileID      *uuid.UUID      `json:"file_id"`
	FileKind    *string         `json:"file_kind"`
	VideoID     *string         `json:"video_id"`
	QuizID      *uuid.UUID      `json:"quiz_id"`
	Minutes     int32           `json:"minutes"`
	Done        bool            `json:"done"`
	Unlocked    bool            `json:"unlocked"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.List(r.Context(), user, p[0])
	if err != nil {
		return err
	}
	items := make([]Response, len(rows))
	for i, a := range rows {
		items[i] = Response{ID: a.ID, Title: a.Title, ClassName: a.ClassName, YearLabel: a.YearLabel, BuildingKey: a.BuildingKey, Level: a.Level, Cancelled: a.CancelledAt.Valid, Done: a.Done, Total: a.Total}
		if a.DueAt.Valid {
			d := a.DueAt.Time
			items[i].DueAt = &d
		}
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
	return nil
}
func (h *Handler) issue(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	var in struct {
		ID       uuid.UUID  `json:"id"`
		ClassID  uuid.UUID  `json:"class_id"`
		ModuleID uuid.UUID  `json:"module_id"`
		DueAt    *time.Time `json:"due_at"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		return err
	}
	a, err := h.svc.Issue(r.Context(), user, p[0], Input(in))
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, map[string]any{"id": a.ID})
	return nil
}
func (h *Handler) steps(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "assignment_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.Steps(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	items := make([]StepResponse, len(rows))
	for i, s := range rows {
		items[i] = StepResponse{RichContent: s.RichContent, ID: s.ID, Kind: s.Kind, Title: s.Title, Body: s.Body, Minutes: s.Minutes, Done: s.Done, Unlocked: s.Unlocked}
		if s.FileID.Valid {
			id := s.FileID.UUID
			items[i].FileID = &id
			if s.FileKind.Valid {
				k := s.FileKind.String
				items[i].FileKind = &k
			}
		}
		if s.VideoID.Valid {
			v := s.VideoID.String
			items[i].VideoID = &v
		}
		if s.QuizID.Valid {
			id := s.QuizID.UUID
			items[i].QuizID = &id
		}
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
	return nil
}
func (h *Handler) complete(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "assignment_id", "step_id")
	if err != nil {
		return err
	}
	if err := h.svc.Complete(r.Context(), user, p[0], p[1], p[2]); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "assignment_id")
	if err != nil {
		return err
	}
	if err := h.svc.Cancel(r.Context(), user, p[0], p[1]); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
func (h *Handler) roster(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "assignment_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.Roster(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	type entry struct {
		UserID      uuid.UUID `json:"user_id"`
		DisplayName string    `json:"display_name"`
		Done        int32     `json:"done"`
		Total       int32     `json:"total"`
	}
	items := make([]entry, len(rows))
	for i, v := range rows {
		items[i] = entry(v)
	}
	httpx.JSON(w, 200, map[string]any{"items": items})
	return nil
}
