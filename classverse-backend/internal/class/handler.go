package class

import (
	"encoding/json"
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
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves class endpoints; mount inside RequireUser. All endpoints are staff-only.
type Handler struct {
	svc    *Service
	log    *slog.Logger
	reads  *ratelimit.Limiter
	writes *ratelimit.Limiter
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log, reads: ratelimit.New(600, time.Minute, svc.now), writes: ratelimit.New(240, time.Minute, svc.now)}
}

// Routes mounts class endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/academic-years", h.handle(h.years))
	r.Post("/orgs/{org_id}/academic-years", h.handle(h.createYear))
	r.Put("/orgs/{org_id}/academic-years/{year_id}/active", h.handle(h.activate))
	r.Get("/orgs/{org_id}/academic-years/{year_id}/classes", h.handle(h.classes))
	r.Post("/orgs/{org_id}/academic-years/{year_id}/classes", h.handle(h.createClass))
	r.Get("/orgs/{org_id}/academic-years/{year_id}/roster", h.handle(h.roster))
	r.Put("/orgs/{org_id}/academic-years/{year_id}/roster/{user_id}", h.handle(h.place))
	r.Put("/orgs/{org_id}/classes/{class_id}", h.handle(h.rename))
	r.Delete("/orgs/{org_id}/classes/{class_id}", h.handle(h.archive))
	r.Get("/orgs/{org_id}/members/{user_id}/placements", h.handle(h.history))
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
		case errors.Is(err, ErrNotFound):
			return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Tahun ajaran atau kelas tidak ditemukan"}
		case errors.Is(err, ErrNotStudent):
			return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Peserta tidak ditemukan di sekolah ini"}
		case errors.Is(err, ErrClassNotEmpty):
			return &httpx.Error{Status: http.StatusConflict, Code: "class_not_empty", Message: "Kelas masih punya siswa. Pindahkan atau keluarkan dulu."}
		}
		var apiErr *httpx.Error
		if mapped := org.ToAPIError(err); errors.As(mapped, &apiErr) {
			return mapped
		}
		return fmt.Errorf("class: %w", err)
	})
}

// YearResponse is an academic year.
type YearResponse struct {
	ID     uuid.UUID `json:"id"`
	Label  string    `json:"label"`
	Active bool      `json:"active"`
}

// Response is a class with its student count.
type Response struct {
	ID       uuid.UUID `json:"id"`
	YearID   uuid.UUID `json:"academic_year_id"`
	Name     string    `json:"name"`
	Students int32     `json:"students"`
}

// RosterResponse is a participant with their class in the year (null: not placed).
type RosterResponse struct {
	UserID      uuid.UUID  `json:"user_id"`
	DisplayName string     `json:"display_name"`
	AvatarKey   string     `json:"avatar_key"`
	ClassID     *uuid.UUID `json:"class_id"`
}

// PlacementResponse is one row of class history.
type PlacementResponse struct {
	ClassName string     `json:"class_name"`
	YearLabel string     `json:"year_label"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

func toYear(y Year) YearResponse { return YearResponse(y) }

func toClass(c Class) Response { return Response(c) }

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

func (h *Handler) years(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	years, err := h.svc.Years(r.Context(), user, p[0])
	if err != nil {
		return err
	}
	items := make([]YearResponse, len(years))
	for i, y := range years {
		items[i] = toYear(y)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (h *Handler) createYear(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id")
	if err != nil {
		return err
	}
	var req struct {
		Label string `json:"label"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	y, err := h.svc.CreateYear(r.Context(), user, p[0], req.Label)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toYear(y))
	return nil
}

func (h *Handler) activate(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "year_id")
	if err != nil {
		return err
	}
	if err := h.svc.Activate(r.Context(), user, p[0], p[1]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) classes(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "year_id")
	if err != nil {
		return err
	}
	classes, err := h.svc.Classes(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	items := make([]Response, len(classes))
	for i, c := range classes {
		items[i] = toClass(c)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

type classRequest struct {
	Name string `json:"name"`
}

func (h *Handler) createClass(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "year_id")
	if err != nil {
		return err
	}
	var req classRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	c, err := h.svc.CreateClass(r.Context(), user, p[0], p[1], req.Name)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toClass(c))
	return nil
}

func (h *Handler) rename(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "class_id")
	if err != nil {
		return err
	}
	var req classRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	c, err := h.svc.RenameClass(r.Context(), user, p[0], p[1], req.Name)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toClass(c))
	return nil
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "class_id")
	if err != nil {
		return err
	}
	if err := h.svc.ArchiveClass(r.Context(), user, p[0], p[1]); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) roster(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "year_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.Roster(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	items := make([]RosterResponse, len(rows))
	for i, e := range rows {
		items[i] = RosterResponse(e)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}

func (h *Handler) place(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "year_id", "user_id")
	if err != nil {
		return err
	}
	var req struct {
		ClassID json.RawMessage `json:"class_id"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	var classID *uuid.UUID
	if len(req.ClassID) == 0 || json.Unmarshal(req.ClassID, &classID) != nil {
		return httpx.ValidationFailed(map[string]string{"class_id": "Pilih kelas atau null untuk mengeluarkan siswa"})
	}
	if err := h.svc.Place(r.Context(), user, p[0], p[1], p[2], classID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) error {
	user, p, err := ids(r, "org_id", "user_id")
	if err != nil {
		return err
	}
	rows, err := h.svc.History(r.Context(), user, p[0], p[1])
	if err != nil {
		return err
	}
	items := make([]PlacementResponse, len(rows))
	for i, e := range rows {
		items[i] = PlacementResponse(e)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
	return nil
}
