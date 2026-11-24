package httpx

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

const maxBodyBytes = 16 << 10

// DecodeJSON reads a small JSON body into v. Unknown fields and malformed
// JSON become a 400 validation_failed.
func DecodeJSON(r *http.Request, v any) error {
	return DecodeJSONLimit(r, v, maxBodyBytes)
}

// DecodeJSONLimit bounds larger structured documents without widening unrelated endpoint limits.
func DecodeJSONLimit(r *http.Request, v any, maxBytes int64) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return &Error{Status: http.StatusBadRequest, Code: "validation_failed", Message: "Format permintaan tidak valid"}
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return &Error{Status: http.StatusBadRequest, Code: "validation_failed", Message: "Format permintaan tidak valid"}
	}
	return nil
}

// Limiter is satisfied by ratelimit.Limiter.
type Limiter interface {
	Allow(key string) (bool, time.Duration)
}

// RateLimit records one event for key and returns 429 rate_limited with a
// Retry-After header once the limit is exceeded.
func RateLimit(w http.ResponseWriter, l Limiter, key string) error {
	ok, retry := l.Allow(key)
	if ok {
		return nil
	}
	w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
	return &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Terlalu banyak percobaan. Coba lagi sebentar lagi."}
}

// Common client errors shared by features.
var (
	ErrNotFound  = &Error{Status: http.StatusNotFound, Code: "not_found", Message: "Tidak ditemukan"}
	ErrForbidden = &Error{Status: http.StatusForbidden, Code: "forbidden", Message: "Kamu tidak punya akses untuk ini"}
)

// ValidationFailed builds a 400 with per-field messages.
func ValidationFailed(fields map[string]string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: "validation_failed", Message: "Periksa kembali isian kamu", Fields: fields}
}
