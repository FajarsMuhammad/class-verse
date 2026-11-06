package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

type admissionPresence struct{ allowed atomic.Bool }

func (p *admissionPresence) VoiceMember(_ context.Context, roomID, userID uuid.UUID) (Member, error) {
	if !p.allowed.Load() {
		return Member{}, ErrNotPresent
	}
	return Member{Name: "Ani", Identity: userID.String() + ":admission", Channel: roomID.String() + ":zone:general"}, nil
}

func TestSignalingWebSocketAndRevokedReplay(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rtc/v1" {
			t.Errorf("path=%s", r.URL.Path)
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"app.test"}})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		kind, data, err := c.Read(r.Context())
		if err != nil {
			return
		}
		_ = c.Write(r.Context(), kind, data)
	}))
	defer upstream.Close()
	p := &admissionPresence{}
	p.allowed.Store(true)
	lk, _ := NewLiveKit("ws"+strings.TrimPrefix(upstream.URL, "http"), "key", "secret", clock)
	token, _ := lk.ParticipantToken(uuid.NewString()+":zone:general", uuid.NewString()+":admission", "Ani", true)
	gateway, err := NewSignaling(lk, p, "http://app.test")
	if err != nil {
		t.Fatal(err)
	}
	public := httptest.NewServer(gateway)
	defer public.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	address := "ws" + strings.TrimPrefix(public.URL, "http") + "/voice/rtc/v1?access_token=" + token
	options := &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://app.test"}}}
	c, _, err := websocket.Dial(ctx, address, options)
	if err != nil {
		t.Fatal("admitted WebSocket failed")
	}
	defer func() { _ = c.CloseNow() }()
	if err = c.Write(ctx, websocket.MessageBinary, []byte("signal")); err != nil {
		t.Fatal(err)
	}
	kind, data, err := c.Read(ctx)
	if err != nil || kind != websocket.MessageBinary || string(data) != "signal" {
		t.Fatalf("echo kind=%v data=%q err=%v", kind, data, err)
	}
	_ = c.CloseNow()
	p.allowed.Store(false)
	stale, res, err := websocket.Dial(ctx, address, options)
	if stale != nil {
		_ = stale.CloseNow()
	}
	if err == nil || res == nil || res.StatusCode != http.StatusForbidden {
		t.Fatal("revoked admission accepted old token")
	}
}
