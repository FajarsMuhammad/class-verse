package realtime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/maps"
)

// MapHandler serves GET /api/maps/{map_key}. Maps are not organization data,
// so the endpoint is public; ETag lets browsers revalidate cheaply.
type MapHandler struct {
	log *slog.Logger
}

// NewMapHandler returns the map file handler.
func NewMapHandler(log *slog.Logger) *MapHandler {
	return &MapHandler{log: log}
}

// Routes mounts the handler under /api.
func (h *MapHandler) Routes(r chi.Router) {
	r.Get("/maps/{map_key}", httpx.Handle(h.log, h.get))
}

func (h *MapHandler) get(w http.ResponseWriter, r *http.Request) error {
	raw, err := maps.Read(chi.URLParam(r, "map_key"))
	if errors.Is(err, maps.ErrUnknown) {
		return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Peta tidak ditemukan"}
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(raw)
	return nil
}

// LoadMaps parses every map the server knows.
func LoadMaps(keys ...string) (map[string]*MapData, error) {
	out := make(map[string]*MapData, len(keys))
	for _, k := range keys {
		raw, err := maps.Read(k)
		if err != nil {
			return nil, err
		}
		m, err := ParseMap(k, raw)
		if err != nil {
			return nil, err
		}
		out[k] = m
	}
	return out, nil
}
