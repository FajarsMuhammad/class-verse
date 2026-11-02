package room

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

// Handler serves room endpoints.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts room endpoints; callers must be authenticated.
func (h *Handler) Routes(r chi.Router) {
	r.Get("/orgs/{org_id}/rooms", h.handle(h.list))
	r.Post("/orgs/{org_id}/rooms", h.handle(h.create))
	r.Get("/rooms/{room_id}", h.handle(h.get))
	r.Patch("/rooms/{room_id}", h.handle(h.update))
	r.Delete("/rooms/{room_id}", h.handle(h.archive))
}

func (h *Handler) handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		return toAPIError(fn(w, r))
	})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if limit, err = strconv.Atoi(raw); err != nil || limit < 1 || limit > maxLimit {
			return httpx.ValidationFailed(map[string]string{"limit": "Limit 1–100"})
		}
	}
	page, err := h.svc.List(r.Context(), userID, orgID, r.URL.Query().Get("building"), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		return err
	}
	resp := api.RoomList{Items: make([]api.Room, 0, len(page.Items))}
	for _, it := range page.Items {
		resp.Items = append(resp.Items, ToAPI(it))
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	var req api.CreateRoomRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	rm, err := h.svc.Create(r.Context(), userID, orgID, CreateInput{
		Name: req.Name, MapKey: string(req.MapKey), BuildingKey: req.BuildingKey, ScheduledAt: req.ScheduledAt,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, ToAPI(rm))
	return nil
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	rm, err := h.svc.Get(r.Context(), userID, roomID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, ToAPI(rm))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := httpx.DecodeJSON(r, &fields); err != nil {
		return err
	}
	var in UpdateInput
	for key, raw := range fields {
		switch key {
		case "name":
			var s string
			if json.Unmarshal(raw, &s) != nil {
				return httpx.ValidationFailed(map[string]string{"name": "Nama ruang tidak valid"})
			}
			in.Name = &s
		case "scheduled_at":
			in.SetSchedule = true
			if string(raw) != "null" {
				var t time.Time
				if json.Unmarshal(raw, &t) != nil {
					return httpx.ValidationFailed(map[string]string{"scheduled_at": "Format jadwal tidak valid"})
				}
				in.ScheduledAt = &t
			}
		default:
			return httpx.ValidationFailed(map[string]string{key: "Field tidak bisa diubah"})
		}
	}
	rm, err := h.svc.Update(r.Context(), userID, roomID, in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, ToAPI(rm))
	return nil
}

func (h *Handler) archive(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	roomID, err := org.PathUUID(r, "room_id")
	if err != nil {
		return err
	}
	if err := h.svc.Archive(r.Context(), userID, roomID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ToAPI converts a room to the API model.
func ToAPI(r Room) api.Room {
	return api.Room{
		Id: r.ID, OrgId: r.OrgID, Name: r.Name, MapKey: api.RoomMap(r.MapKey), BuildingKey: r.BuildingKey,
		Capacity: r.Capacity, ScheduledAt: r.ScheduledAt, CreatedAt: r.CreatedAt.UTC(), People: r.People, Live: r.Live,
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
		return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Ruang tidak ditemukan"}
	case errors.Is(err, ErrBusy):
		return &httpx.Error{Status: http.StatusConflict, Code: "room_busy", Message: "Ruang masih dipakai. Arsipkan setelah kosong 5 menit."}
	case errors.Is(err, ErrInvalidCursor):
		return httpx.ValidationFailed(map[string]string{"cursor": "Cursor tidak valid"})
	}
	var apiErr *httpx.Error
	if mapped := org.ToAPIError(err); errors.As(mapped, &apiErr) {
		return mapped
	}
	return fmt.Errorf("room: %w", err)
}
