package httpx

import (
	"net/http"
	"strconv"
)

// Pagination validates bounded offset pagination at the HTTP boundary.
func Pagination(r *http.Request) (int32, int32, error) {
	limit, offset := int64(20), int64(0)
	for key, target := range map[string]*int64{"limit": &limit, "offset": &offset} {
		if raw := r.URL.Query().Get(key); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 32)
			if err != nil || value < 0 {
				return 0, 0, ValidationFailed(map[string]string{key: "Pagination tidak valid"})
			}
			*target = value
		}
	}
	if limit < 1 || limit > 100 {
		return 0, 0, ValidationFailed(map[string]string{"limit": "Limit harus 1–100"})
	}
	return int32(limit), int32(offset), nil
}
