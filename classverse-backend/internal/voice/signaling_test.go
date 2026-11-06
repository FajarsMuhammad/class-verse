package voice

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSignalingAdmission(t *testing.T) {
	roomID, userID := uuid.New(), uuid.New()
	tests := []struct {
		name, path, origin      string
		present, publish, muted bool
		want                    int
	}{
		{name: "connected member", path: "/voice/rtc", origin: "http://app.test", present: true, publish: true, want: 204},
		{name: "v1 join", path: "/voice/rtc/v1", origin: "http://app.test", present: true, publish: true, want: 204},
		{name: "validation", path: "/voice/rtc/v1/validate", present: true, publish: true, want: 204},
		{name: "old token after kick", path: "/voice/rtc", origin: "http://app.test", publish: true, want: 403},
		{name: "old publishing token after mute", path: "/voice/rtc", origin: "http://app.test", present: true, publish: true, muted: true, want: 403},
		{name: "foreign origin", path: "/voice/rtc", origin: "http://evil.test", present: true, publish: true, want: 403},
		{name: "admin API blocked", path: "/voice/twirp/livekit.RoomService/ListRooms", present: true, publish: true, want: 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/voice") {
					t.Error("proxy prefix not stripped")
				}
				if r.URL.Query().Get("access_token") == "" {
					t.Error("missing token upstream")
				}
				if r.Header.Get("Authorization") != "" {
					t.Error("forwarded alternate authorization")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer upstream.Close()
			lk, err := NewLiveKit("ws"+strings.TrimPrefix(upstream.URL, "http"), "key", "secret", clock)
			if err != nil {
				t.Fatal(err)
			}
			token, err := lk.ParticipantToken(roomID.String()+":zone:general", userID.String()+":admission", "Ani", tt.publish)
			if err != nil {
				t.Fatal(err)
			}
			p := fakePresence{name: "Ani", muted: tt.muted}
			if !tt.present {
				p.err = ErrNotPresent
			}
			gateway, err := NewSignaling(lk, p, "http://app.test")
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, tt.path+"?access_token="+token, nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Authorization", "Bearer alternate")
			w := httptest.NewRecorder()
			gateway.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

func TestSignalingRejectsInvalidTokens(t *testing.T) {
	lk, _ := NewLiveKit("ws://127.0.0.1:1", "key", "secret", clock)
	gateway, err := NewSignaling(lk, fakePresence{name: "Ani"}, "http://app.test")
	if err != nil {
		t.Fatal(err)
	}
	admin, _ := lk.sign("", "", adminTTL, &videoGrant{RoomAdmin: true, Room: uuid.NewString()})
	expired, _ := lk.sign(uuid.NewString(), "Ani", -tokenTTL, &videoGrant{RoomJoin: true, Room: uuid.NewString()})
	other, _ := NewLiveKit("ws://127.0.0.1:1", "key", "wrong", clock)
	forged, _ := other.ParticipantToken(uuid.NewString(), uuid.NewString(), "Ani", true)
	for _, token := range []string{"", "broken", admin, expired, forged} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/voice/rtc/validate?access_token="+token, nil)
		gateway.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("status=%d", w.Code)
		}
	}
}

var _ Presence = fakePresence{}
