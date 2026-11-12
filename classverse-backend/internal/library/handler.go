package library

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
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

// Handler serves catalog endpoints; mount inside RequireUser.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes mounts catalog endpoints.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/library/books", h.handle(h.list))
	r.Post("/orgs/{org_id}/library/books", h.handle(h.create))
	r.Put("/orgs/{org_id}/library/books/{book_id}", h.handle(h.update))
	r.Delete("/orgs/{org_id}/library/books/{book_id}", h.handle(h.archive))
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
			return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Buku tidak ditemukan"}
		case errors.Is(err, ErrInvalidCursor):
			return httpx.ValidationFailed(map[string]string{"cursor": "Cursor tidak valid"})
		}
		var apiErr *httpx.Error
		if mapped := org.ToAPIError(err); errors.As(mapped, &apiErr) {
			return mapped
		}
		return fmt.Errorf("library: %w", err)
	})
}

// BookResponse is the public view of a catalog entry.
type BookResponse struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	Author    string     `json:"author"`
	Shelf     string     `json:"shelf"`
	Level     string     `json:"level"`
	Format    string     `json:"format"`
	Copies    int32      `json:"copies"`
	FileID    *uuid.UUID `json:"file_id"`
	Cover     string     `json:"cover"`
	Sample    bool       `json:"sample"`
	Published bool       `json:"published"`
	Position  int32      `json:"position"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func toResponse(b Book) BookResponse {
	var file *uuid.UUID
	if b.FileID.Valid {
		file = &b.FileID.UUID
	}
	return BookResponse{
		ID: b.ID, Title: b.Title, Author: b.Author, Shelf: b.Shelf, Level: b.Level, Format: b.Format,
		Copies: b.Copies, FileID: file, Cover: b.Cover, Sample: b.Sample, Published: b.PublishedAt.Valid,
		Position: b.Position, UpdatedAt: b.UpdatedAt.Time,
	}
}

type bookRequest struct {
	Title     string     `json:"title"`
	Author    string     `json:"author"`
	Shelf     string     `json:"shelf"`
	Level     string     `json:"level"`
	Format    string     `json:"format"`
	Copies    int        `json:"copies"`
	FileID    *uuid.UUID `json:"file_id"`
	Cover     string     `json:"cover"`
	Published bool       `json:"published"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	q := r.URL.Query()
	f := Filter{Shelf: q.Get("shelf"), Query: q.Get("q"), Cursor: q.Get("cursor"), Drafts: q.Get("drafts") == "true"}
	if raw := q.Get("limit"); raw != "" {
		if f.Limit, err = strconv.Atoi(raw); err != nil || f.Limit < 1 || f.Limit > maxLimit {
			return httpx.ValidationFailed(map[string]string{"limit": "Limit 1–50"})
		}
	}
	page, err := h.svc.List(r.Context(), user, orgID, f)
	if err != nil {
		return err
	}
	items := make([]BookResponse, 0, len(page.Items))
	for _, b := range page.Items {
		items = append(items, toResponse(b))
	}
	var next *string
	if page.NextCursor != "" {
		next = &page.NextCursor
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	var req bookRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	b, err := h.svc.Create(r.Context(), user, orgID, Input(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toResponse(Book{LibraryBook: b}))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	id, err := org.PathUUID(r, "book_id")
	if err != nil {
		return err
	}
	var req bookRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	b, err := h.svc.Update(r.Context(), user, orgID, id, Input(req))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toResponse(Book{LibraryBook: b}))
	return nil
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	id, err := org.PathUUID(r, "book_id")
	if err != nil {
		return err
	}
	if err := h.svc.Archive(r.Context(), user, orgID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
