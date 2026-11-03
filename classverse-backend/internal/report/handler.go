package report

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

// Handler serves report and moderation-log endpoints.
type Handler struct {
	svc *Service
	log *slog.Logger
}

// NewHandler wires the HTTP layer.
func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// Routes mounts the endpoints; callers must be authenticated.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/reports", h.handle(h.create))
	r.Get("/orgs/{org_id}/reports", h.handle(h.list))
	r.Patch("/reports/{report_id}", h.handle(h.review))
	r.Get("/orgs/{org_id}/moderation-log", h.handle(h.moderationLog))
}

func (h *Handler) handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		return toAPIError(fn(w, r))
	})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	var req api.CreateReportRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	rep, err := h.svc.Create(r.Context(), userID, CreateInput{
		TargetID: req.TargetUserId, MessageID: req.MessageId, RoomID: req.RoomId, Reason: req.Reason,
	})
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, toAPI(rep))
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	limit, err := parseLimit(r)
	if err != nil {
		return err
	}
	items, next, err := h.svc.List(r.Context(), userID, orgID, r.URL.Query().Get("status"), limit, r.URL.Query().Get("cursor"))
	if err != nil {
		return err
	}
	resp := api.ReportList{Items: make([]api.Report, 0, len(items))}
	for _, it := range items {
		resp.Items = append(resp.Items, toAPI(it))
	}
	if next != "" {
		resp.NextCursor = &next
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	id, err := org.PathUUID(r, "report_id")
	if err != nil {
		return err
	}
	var req api.ReviewReportRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	rep, err := h.svc.Review(r.Context(), userID, id, string(req.Status))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, toAPI(rep))
	return nil
}

func (h *Handler) moderationLog(w http.ResponseWriter, r *http.Request) error {
	userID, _ := auth.UserID(r.Context())
	orgID, err := org.PathUUID(r, "org_id")
	if err != nil {
		return err
	}
	limit, err := parseLimit(r)
	if err != nil {
		return err
	}
	items, next, err := h.svc.Log(r.Context(), userID, orgID, limit, r.URL.Query().Get("cursor"))
	if err != nil {
		return err
	}
	resp := api.ModerationLog{Items: make([]api.ModerationAction, 0, len(items))}
	for _, a := range items {
		it := api.ModerationAction{
			Id: a.ID, RoomId: a.RoomID, RoomName: a.RoomName, Action: api.ModerationActionAction(a.Action),
			Actor: api.UserRef{Id: a.ActorID, Name: a.ActorName}, CreatedAt: a.CreatedAt,
		}
		if a.TargetID != nil {
			it.Target = &api.UserRef{Id: *a.TargetID, Name: a.TargetName}
		}
		resp.Items = append(resp.Items, it)
	}
	if next != "" {
		resp.NextCursor = &next
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

func parseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxLimit {
		return 0, httpx.ValidationFailed(map[string]string{"limit": "Limit 1–100"})
	}
	return n, nil
}

func toAPI(r Report) api.Report {
	return api.Report{
		Id: r.ID, OrgId: r.OrgID, RoomId: r.RoomID, RoomName: r.RoomName,
		Reporter:  api.UserRef{Id: r.ReporterID, Name: r.ReporterName},
		Target:    api.UserRef{Id: r.TargetID, Name: r.TargetName},
		MessageId: r.MessageID, MessageText: r.MessageText, Reason: r.Reason, Status: api.ReportStatus(r.Status),
		ReviewedBy: r.ReviewedBy, ReviewedAt: r.ReviewedAt, CreatedAt: r.CreatedAt,
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
		return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Laporan, pengguna, atau pesan tidak ditemukan"}
	case errors.Is(err, ErrSelfReport):
		return httpx.ValidationFailed(map[string]string{"target_user_id": "Kamu tidak bisa melaporkan diri sendiri"})
	case errors.Is(err, ErrInvalidCursor):
		return httpx.ValidationFailed(map[string]string{"cursor": "Cursor tidak valid"})
	}
	var apiErr *httpx.Error
	if mapped := org.ToAPIError(err); errors.As(mapped, &apiErr) {
		return mapped
	}
	return fmt.Errorf("report: %w", err)
}
