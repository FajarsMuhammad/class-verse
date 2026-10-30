// Package httpx holds the shared HTTP plumbing: JSON responses, the error
// shape from the API contract, and request middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// Error is an API error that is safe to show to the client.
// It is rendered as {"error": {"code", "message", "fields"}}.
type Error struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string // per-field validation messages, optional
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

var errInternal = &Error{
	Status:  http.StatusInternalServerError,
	Code:    "internal_error",
	Message: "Terjadi kesalahan pada server. Coba lagi nanti.",
}

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	// The header is already sent; an encode failure here means the client went away.
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError renders err in the API error shape. Errors that are not an
// *Error are logged and hidden behind a generic 500 internal_error.
func WriteError(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		log.ErrorContext(r.Context(), "unhandled error",
			slog.String("request_id", middleware.GetReqID(r.Context())),
			slog.String("method", r.Method),
			slog.String("route", routePattern(r)),
			slog.Any("err", err),
		)
		apiErr = errInternal
	}
	JSON(w, apiErr.Status, errorBody{Error: errorPayload{
		Code:    apiErr.Code,
		Message: apiErr.Message,
		Fields:  apiErr.Fields,
	}})
}

// Handle adapts a handler that returns an error into an http.HandlerFunc,
// so every handler shares the same error flow.
func Handle(log *slog.Logger, h func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteError(w, r, log, err)
		}
	}
}
