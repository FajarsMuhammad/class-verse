package realtime

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMapHandler(t *testing.T) {
	r := chi.NewRouter()
	NewMapHandler(slog.New(slog.NewTextHandler(io.Discard, nil))).Routes(r)
	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/maps/campus", nil))
	if first.Code != http.StatusOK || first.Header().Get("Cache-Control") != "no-cache" || first.Body.Len() == 0 {
		t.Fatalf("map response: status=%d headers=%v", first.Code, first.Header())
	}
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	req := httptest.NewRequest(http.MethodGet, "/maps/campus", nil)
	req.Header.Set("If-None-Match", etag)
	cached := httptest.NewRecorder()
	r.ServeHTTP(cached, req)
	if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Fatalf("revalidate: %d", cached.Code)
	}
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/maps/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing map: %d", missing.Code)
	}
}
