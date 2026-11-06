package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEvictionRequiresConfirmedAbsence(t *testing.T) {
	for _, tt := range []struct {
		name          string
		remove, probe int
		code          string
		wantError     bool
	}{
		{"confirmed", 200, 404, "not_found", false},
		{"still present", 200, 200, "", true},
		{"remove outage", 503, 404, "unavailable", true},
		{"probe outage", 200, 503, "unavailable", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				status := tt.remove
				if strings.HasSuffix(r.URL.Path, "GetParticipant") {
					status = tt.probe
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"code":"` + tt.code + `"}`))
			}))
			defer server.Close()
			lk, _ := NewLiveKit("ws"+strings.TrimPrefix(server.URL, "http"), "key", "secret", clock)
			err := lk.Remove(context.Background(), "room", "identity")
			if (err != nil) != tt.wantError {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
