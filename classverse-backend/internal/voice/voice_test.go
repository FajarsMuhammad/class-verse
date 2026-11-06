package voice

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
)

var clock = func() time.Time { return time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC) }

func parse(t *testing.T, tok, secret string) claims {
	t.Helper()
	var c claims
	_, err := jwt.ParseWithClaims(tok, &c, func(*jwt.Token) (any, error) { return []byte(secret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithTimeFunc(clock))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewLiveKitConfig(t *testing.T) {
	tests := []struct {
		name, url, key, secret string
		off, bad               bool
	}{
		{name: "all empty disables", off: true},
		{name: "partial", url: "ws://x", bad: true},
		{name: "http scheme", url: "http://x", key: "k", secret: "s", bad: true},
		{name: "ws", url: "ws://lk:7880", key: "k", secret: "s"},
		{name: "wss", url: "wss://lk.example/", key: "k", secret: "s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lk, err := NewLiveKit(tt.url, tt.key, tt.secret, clock)
			switch {
			case tt.bad && err == nil, !tt.bad && err != nil:
				t.Fatalf("err=%v", err)
			case tt.off && lk != nil:
				t.Fatal("voice should be off")
			}
		})
	}
	lk, _ := NewLiveKit("wss://lk.example/", "k", "s", clock)
	if lk.api != "https://lk.example" {
		t.Errorf("api url = %q", lk.api)
	}
}

func TestParticipantToken(t *testing.T) {
	lk, _ := NewLiveKit("ws://lk", "key", "secret", clock)
	for _, publish := range []bool{true, false} {
		tok, err := lk.ParticipantToken("room-1", "user-1", "Ani", publish)
		if err != nil {
			t.Fatal(err)
		}
		c := parse(t, tok, "secret")
		g := c.Video
		if c.Issuer != "key" || c.Subject != "user-1" || c.Name != "Ani" || !g.RoomJoin || g.Room != "room-1" || g.RoomAdmin {
			t.Fatalf("claims = %+v %+v", c, g)
		}
		if c.ExpiresAt.Sub(clock()) != tokenTTL {
			t.Errorf("ttl = %v", c.ExpiresAt.Sub(clock()))
		}
		if *g.CanPublish != publish || !*g.CanSubscribe || *g.CanPublishData || *g.CanUpdateOwnMetadata {
			t.Errorf("publish=%v grant=%+v", publish, g)
		}
		if publish && (len(g.CanPublishSources) != 1 || g.CanPublishSources[0] != "microphone") {
			t.Errorf("sources = %v", g.CanPublishSources)
		}
	}
}

func TestRoomServiceCalls(t *testing.T) {
	type call struct {
		path string
		body map[string]any
		auth claims
	}
	var got []call
	status, code := http.StatusOK, ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		got = append(got, call{r.URL.Path, body, parse(t, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), "secret")})
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"code":"`+code+`"}`)
	}))
	defer srv.Close()
	lk, _ := NewLiveKit("ws"+strings.TrimPrefix(srv.URL, "http"), "key", "secret", clock)
	ctx := context.Background()

	if err := lk.SetCanPublish(ctx, "r", "u", false); err != nil {
		t.Fatal(err)
	}
	if err := lk.Remove(ctx, "r", "u"); err != nil {
		t.Fatal(err)
	}
	if got[0].path != "/twirp/livekit.RoomService/UpdateParticipant" || got[1].path != "/twirp/livekit.RoomService/RemoveParticipant" {
		t.Fatalf("paths = %s %s", got[0].path, got[1].path)
	}
	perm := got[0].body["permission"].(map[string]any)
	if perm["can_publish"] != false || got[0].body["identity"] != "u" {
		t.Errorf("mute body = %v", got[0].body)
	}
	if a := got[0].auth.Video; !a.RoomAdmin || a.Room != "r" || a.RoomJoin {
		t.Errorf("admin grant = %+v", a)
	}

	status, code = http.StatusNotFound, "not_found"
	if err := lk.Remove(ctx, "r", "gone"); err != nil {
		t.Errorf("participant already gone should be ok: %v", err)
	}
	status, code = http.StatusUnauthorized, "unauthenticated"
	if err := lk.Remove(ctx, "r", "u"); err == nil {
		t.Error("401 accepted")
	}
}

type fakePresence struct {
	name  string
	muted bool
	err   error
}

func (f fakePresence) VoiceMember(context.Context, uuid.UUID, uuid.UUID) (string, bool, error) {
	return f.name, f.muted, f.err
}

func TestTokenEndpoint(t *testing.T) {
	lk, _ := NewLiveKit("ws://lk", "key", "secret", clock)
	tests := []struct {
		name   string
		issuer Issuer
		p      fakePresence
		room   string
		status int
		code   string
	}{
		{"connected member", lk, fakePresence{name: "Ani"}, uuid.NewString(), 200, ""},
		{"muted member cannot publish", lk, fakePresence{name: "Ani", muted: true}, uuid.NewString(), 200, ""},
		{"not in room", lk, fakePresence{err: ErrNotPresent}, uuid.NewString(), 404, "not_found"},
		{"bad id", lk, fakePresence{}, "nope", 404, "not_found"},
		{"voice off", nil, fakePresence{name: "Ani"}, uuid.NewString(), 503, "voice_unavailable"},
		{"hub error", lk, fakePresence{err: errors.New("boom")}, uuid.NewString(), 500, "internal_error"},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)), time.Now)
	access, err := tokens.Issue(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(auth.RequireUser(tokens, log))
			NewHandler(NewService(tt.p, tt.issuer), log).Routes(r)
			req := httptest.NewRequest(http.MethodPost, "/rooms/"+tt.room+"/voice-token", nil)
			req.Header.Set("Authorization", "Bearer "+access)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d body %s", rec.Code, rec.Body)
			}
			var body struct {
				URL   string                `json:"url"`
				Token string                `json:"token"`
				Error struct{ Code string } `json:"error"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if tt.status != 200 {
				if body.Error.Code != tt.code {
					t.Errorf("code = %q", body.Error.Code)
				}
				return
			}
			c := parse(t, body.Token, "secret")
			if body.URL != "ws://lk" || c.Name != "Ani" || *c.Video.CanPublish == tt.p.muted || c.Video.Room != tt.room {
				t.Errorf("url=%s claims=%+v", body.URL, c.Video)
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Error("token response cacheable")
			}
		})
	}
}
