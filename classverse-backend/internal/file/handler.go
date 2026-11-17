package file

import (
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

// Handler serves authenticated resumable-upload endpoints.
type Handler struct {
	uploads *Uploader
	log     *slog.Logger
	limit   *ratelimit.Limiter
	reads   *ratelimit.Limiter
}

// NewHandler applies per-user limits in addition to global API limits.
func NewHandler(uploads *Uploader, log *slog.Logger, now func() time.Time) *Handler {
	// Reads get their own bucket: PDF.js issues many range requests per document.
	return &Handler{uploads: uploads, log: log, limit: ratelimit.New(120, time.Minute, now), reads: ratelimit.New(600, time.Minute, now)}
}

// Routes must be mounted inside RequireUser.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/orgs/{org_id}/uploads", h.handle(h.start))
	r.Get("/orgs/{org_id}/uploads/{file_id}", h.handle(h.status))
	r.Put("/orgs/{org_id}/uploads/{file_id}/parts/{part}", h.handle(h.part))
	r.Post("/orgs/{org_id}/uploads/{file_id}/complete", h.handle(h.complete))
	r.Delete("/orgs/{org_id}/uploads/{file_id}", h.handle(h.cancel))
	r.Get("/orgs/{org_id}/files", h.read(h.list))
	r.Get("/orgs/{org_id}/files/{file_id}/content", h.read(h.content))
	r.Put("/orgs/{org_id}/files/{file_id}/position", h.read(h.position))
	r.Delete("/orgs/{org_id}/files/{file_id}", h.handle(h.remove))
}

func (h *Handler) read(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return h.limited(h.reads, fn)
}
func (h *Handler) handle(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return h.limited(h.limit, fn)
}

func (h *Handler) limited(limit *ratelimit.Limiter, fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		user, ok := auth.UserID(r.Context())
		if !ok {
			return &httpx.Error{Status: 401, Code: "unauthenticated", Message: "Silakan masuk"}
		}
		if err := httpx.RateLimit(w, limit, user.String()); err != nil {
			return err
		}
		err := fn(w, r)
		switch {
		case errors.Is(err, ErrNotFound):
			return httpx.ErrNotFound
		case errors.Is(err, org.ErrForbidden), errors.Is(err, org.ErrNotMember):
			return httpx.ErrForbidden
		case errors.Is(err, ErrEncryptedPDF):
			return &httpx.Error{Status: 400, Code: "pdf_encrypted", Message: "PDF berpassword tidak didukung. Hapus password lalu unggah lagi."}
		case errors.Is(err, ErrRange):
			return &httpx.Error{Status: http.StatusRequestedRangeNotSatisfiable, Code: "range_not_satisfiable", Message: "Rentang file tidak valid"}
		case errors.Is(err, ErrInvalidUpload):
			return httpx.ValidationFailed(map[string]string{"file": "File bukan PDF/MP3/M4A yang valid, atau ukuran/chunk/checksum tidak cocok"})
		case errors.Is(err, ErrInUse):
			return &httpx.Error{Status: 409, Code: "file_in_use", Message: "File masih dipakai buku katalog atau langkah modul. Arsipkan dulu yang memakainya."}
		case errors.Is(err, ErrState):
			return &httpx.Error{Status: 409, Code: "conflict", Message: "Upload sedang diproses, belum lengkap, atau sudah kedaluwarsa"}
		case errors.Is(err, ErrQuota):
			return &httpx.Error{Status: 409, Code: "quota_exceeded", Message: "Kuota penyimpanan sekolah tidak cukup"}
		default:
			return err
		}
	})
}
func uploadIDs(r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return user, orgID, uuid.Nil, err
	}
	id, err := org.PathUUID(r, "file_id")
	return user, orgID, id, err
}
func (h *Handler) start(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	var req struct {
		ID     uuid.UUID `json:"id"`
		Name   string    `json:"name"`
		Size   int64     `json:"size"`
		Kind   string    `json:"kind"`
		SHA256 string    `json:"sha256"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	digest, err := hex.DecodeString(req.SHA256)
	if err != nil {
		return ErrInvalidUpload
	}
	if req.ID == uuid.Nil {
		return ErrInvalidUpload
	}
	result, err := h.uploads.StartWithID(r.Context(), user, orgID, req.ID, req.Name, req.Size, req.Kind, digest)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, result)
	return nil
}
func (h *Handler) status(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	result, err := h.uploads.Status(r.Context(), user, orgID, id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, result)
	return nil
}
func (h *Handler) part(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	part, err := strconv.Atoi(chi.URLParam(r, "part"))
	if err != nil {
		return ErrInvalidUpload
	}
	r.Body = http.MaxBytesReader(w, r.Body, ChunkBytes)
	if err := h.uploads.Part(r.Context(), user, orgID, id, part, r.Body); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
func (h *Handler) complete(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	result, err := h.uploads.Assemble(r.Context(), user, orgID, id)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, result)
	return nil
}
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	if err := h.uploads.Cancel(r.Context(), user, orgID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Item is the safe public view of a ready file.
type Item struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	UploadedBy  uuid.UUID `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
	Position    int32     `json:"position"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	user, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	rows, err := h.uploads.meta.List(r.Context(), user, orgID)
	if err != nil {
		return err
	}
	usage, err := h.uploads.meta.Usage(r.Context(), user, orgID)
	if err != nil {
		return err
	}
	items := make([]Item, 0, len(rows))
	for _, f := range rows {
		items = append(items, Item{ID: f.ID, Name: f.Name, Kind: f.Kind, ContentType: f.ContentType.String, Size: f.SizeBytes, UploadedBy: f.UploadedBy, CreatedAt: f.CreatedAt.Time, Position: f.Position})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "used_bytes": usage.UsedBytes, "quota_bytes": usage.QuotaBytes})
	return nil
}

// parseRange accepts one "bytes=a-b", "bytes=a-" or "bytes=-n" range.
func parseRange(raw string, size int64) (int64, int64, error) {
	if raw == "" {
		return 0, -1, nil
	}
	spec, ok := strings.CutPrefix(raw, "bytes=")
	first, last, found := strings.Cut(spec, "-")
	if !ok || !found || strings.Contains(spec, ",") {
		return 0, 0, ErrRange
	}
	if first == "" {
		n, err := strconv.ParseInt(last, 10, 64)
		if err != nil || n < 1 {
			return 0, 0, ErrRange
		}
		return max(0, size-n), size - 1, nil
	}
	start, err := strconv.ParseInt(first, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, ErrRange
	}
	if last == "" {
		return start, size - 1, nil
	}
	end, err := strconv.ParseInt(last, 10, 64)
	if err != nil || end < start {
		return 0, 0, ErrRange
	}
	return start, min(end, size-1), nil
}

func (h *Handler) content(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	// Authorize before parsing Range so a 416 never reveals file size to non-members.
	f, err := h.uploads.meta.readable(r.Context(), user, orgID, id)
	if err != nil {
		return err
	}
	start, end, err := parseRange(r.Header.Get("Range"), f.SizeBytes)
	if err != nil {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(f.SizeBytes, 10))
		return err
	}
	if r.Header.Get("Range") == "" {
		end = f.SizeBytes - 1
	}
	c, err := h.uploads.Open(r.Context(), f, start, end)
	if err != nil {
		return err
	}
	defer c.Body.Close()
	hd := w.Header()
	hd.Set("Content-Type", f.ContentType.String)
	hd.Set("Accept-Ranges", "bytes")
	hd.Set("Cache-Control", "private, no-store")
	hd.Set("X-Content-Type-Options", "nosniff")
	hd.Set("Content-Security-Policy", "sandbox")
	status := http.StatusOK
	if c.Partial {
		status = http.StatusPartialContent
		hd.Set("Content-Range", c.Header.Get("Content-Range"))
	}
	if n := c.Header.Get("Content-Length"); n != "" {
		hd.Set("Content-Length", n)
	}
	w.WriteHeader(status)
	// Headers are sent; a broken client connection here is not an API error.
	_, _ = io.Copy(w, c.Body)
	return nil
}

func (h *Handler) position(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	var req struct {
		Position int32 `json:"position"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.uploads.meta.SavePosition(r.Context(), user, orgID, id, req.Position); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) error {
	user, orgID, id, err := uploadIDs(r)
	if err != nil {
		return err
	}
	if err := h.uploads.Delete(r.Context(), user, orgID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
