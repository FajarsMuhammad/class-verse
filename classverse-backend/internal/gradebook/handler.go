package gradebook

import (
	"bytes"
	"errors"
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

// Handler serves read-only academic reports inside RequireUser.
type Handler struct {
	svc            *Service
	log            *slog.Logger
	reads, exports *ratelimit.Limiter
}

// NewHandler applies separate bounded read/export budgets.
func NewHandler(s *Service, log *slog.Logger, now func() time.Time) *Handler {
	return &Handler{svc: s, log: log, reads: ratelimit.New(120, time.Minute, now), exports: ratelimit.New(6, time.Minute, now)}
}

// Routes mounts academic paths distinct from moderation reports.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/gradebook", h.handle(h.roster, false))
	r.Get("/orgs/{org_id}/gradebook/csv", h.handle(h.export, true))
	r.Get("/orgs/{org_id}/gradebook/me", h.handle(h.student, false))
	r.Get("/orgs/{org_id}/gradebook/students/{user_id}", h.handle(h.student, false))
}
func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request, uuid.UUID, uuid.UUID, Filter) error, export bool) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		user, _ := auth.UserID(r.Context())
		limit := h.reads
		if export {
			limit = h.exports
		}
		if err := httpx.RateLimit(w, limit, user.String()); err != nil {
			return err
		}
		orgID, err := org.PathUUID(r, "org_id")
		if err != nil {
			return err
		}
		f, err := parseFilter(r)
		if err != nil {
			return httpx.ValidationFailed(map[string]string{"filter": "Filter rapor tidak valid"})
		}
		err = fn(w, r, user, orgID, f)
		if errors.Is(err, ErrNotFound) {
			return &httpx.Error{Status: 404, Code: "not_found", Message: "Siswa atau filter rapor tidak ditemukan"}
		}
		if errors.Is(err, ErrInvalid) {
			return httpx.ValidationFailed(map[string]string{"filter": "Pilih tahun ajaran untuk filter kelas; periksa batas halaman"})
		}
		return org.ToAPIError(err)
	})
}
func parseFilter(r *http.Request) (Filter, error) {
	f := Filter{Building: r.URL.Query().Get("building_key")}
	for _, field := range []struct {
		name string
		dest **uuid.UUID
	}{{"year_id", &f.YearID}, {"class_id", &f.ClassID}, {"module_id", &f.ModuleID}, {"quiz_id", &f.QuizID}} {
		if value := r.URL.Query().Get(field.name); value != "" {
			id, err := uuid.Parse(value)
			if err != nil || id == uuid.Nil {
				return f, ErrInvalid
			}
			*field.dest = &id
		}
	}
	return f, nil
}
func (h *Handler) student(w http.ResponseWriter, r *http.Request, user, orgID uuid.UUID, f Filter) error {
	target := user
	if chi.URLParam(r, "user_id") != "" {
		id, err := org.PathUUID(r, "user_id")
		if err != nil {
			return err
		}
		target = id
	}
	report, err := h.svc.Student(r.Context(), user, orgID, target, f)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, report)
	return nil
}
func (h *Handler) roster(w http.ResponseWriter, r *http.Request, user, orgID uuid.UUID, f Filter) error {
	offset, limit := 0, 20
	var err error
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 || offset > 1000000 {
			return ErrInvalid
		}
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 50 {
			return ErrInvalid
		}
	}
	page, err := h.svc.Roster(r.Context(), user, orgID, f, int32(offset), int32(limit))
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, page)
	return nil
}
func (h *Handler) export(w http.ResponseWriter, r *http.Request, user, orgID uuid.UUID, f Filter) error {
	page, err := h.svc.Roster(r.Context(), user, orgID, f, 0, 10001)
	if err != nil {
		return err
	}
	if page.Total > 10000 {
		return &httpx.Error{Status: 400, Code: "export_limit", Message: "Ekspor maksimal 10.000 siswa. Persempit filter kelas."}
	}
	var buf bytes.Buffer
	if err := WriteCSV(&buf, page.Items); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="rapor.csv"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, err = w.Write(buf.Bytes())
	return err
}
