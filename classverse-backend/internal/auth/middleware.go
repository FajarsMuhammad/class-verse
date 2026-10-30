package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
)

type ctxKey struct{}

var errUnauthenticated = &httpx.Error{Status: http.StatusUnauthorized, Code: "unauthenticated", Message: "Sesi berakhir. Silakan masuk lagi."}

// RequireUser rejects requests without a valid Bearer access token and stores
// the user id in the request context (read it with UserID).
func RequireUser(tokens *Tokens, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				httpx.WriteError(w, r, log, errUnauthenticated)
				return
			}
			id, err := tokens.Verify(strings.TrimSpace(raw))
			if err != nil {
				httpx.WriteError(w, r, log, errUnauthenticated)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
		})
	}
}

// UserID returns the authenticated user id set by RequireUser.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return id, ok
}
