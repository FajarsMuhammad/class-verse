package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type channelPresence struct{ member Member }

func (p channelPresence) VoiceMember(context.Context, uuid.UUID, uuid.UUID) (Member, error) {
	return p.member, nil
}

func TestChannelAdmissionRejectsOldAndForeignGrants(t *testing.T) {
	root, user := uuid.New(), uuid.New()
	identity := user.String() + ":" + uuid.NewString()
	channel := root.String() + ":zone:one"
	member := Member{Name: "Ani", Identity: identity, Channel: channel}
	lk, _ := NewLiveKit("ws://127.0.0.1:1", "key", "secret", clock)
	gateway, _ := NewSignaling(lk, channelPresence{member}, "http://app.test")
	for _, tt := range []struct {
		name, room, identity string
		publish              bool
	}{
		{"other zone", root.String() + ":zone:two", identity, true},
		{"old admission", channel, user.String() + ":" + uuid.NewString(), true},
		{"student broadcast", root.String() + ":broadcast", identity, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			token, _ := lk.ParticipantToken(tt.room, tt.identity, "Ani", tt.publish)
			req := httptest.NewRequest(http.MethodGet, "/voice/rtc/validate?access_token="+token, nil)
			w := httptest.NewRecorder()
			gateway.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d", w.Code)
			}
		})
	}
	token, err := NewService(channelPresence{member}, lk, "ws://app.test/voice").Token(context.Background(), root, user)
	if err != nil {
		t.Fatal(err)
	}
	zone, broadcast := parse(t, token.Token, "secret"), parse(t, token.BroadcastToken, "secret")
	if zone.Video.Room != channel || zone.Subject != identity || !*zone.Video.CanPublish {
		t.Fatal("wrong zone grant")
	}
	if broadcast.Video.Room != root.String()+":broadcast" || *broadcast.Video.CanPublish {
		t.Fatal("student can broadcast")
	}
}
