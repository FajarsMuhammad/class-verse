package voice

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Signaling exposes only browser signaling, never RoomService. LiveKit's own
// signaling port must remain private; media ports do not use this gateway.
type Signaling struct {
	livekit  *LiveKit
	presence Presence
	origin   string
	proxy    *httputil.ReverseProxy
}

// NewSignaling checks membership again on joins, reconnects and validation.
// It accepts refreshed LiveKit tokens only while their admission is still valid.
func NewSignaling(lk *LiveKit, presence Presence, origin string) (*Signaling, error) {
	target, err := url.Parse(lk.api)
	if err != nil || target.Host == "" {
		return nil, fmt.Errorf("voice signaling: invalid upstream URL")
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL.Path = strings.TrimPrefix(p.In.URL.Path, "/voice")
			p.Out.URL.RawPath = ""
			p.SetURL(target)
			p.Out.Header.Del("Authorization")
			p.Out.Header.Del("Cookie")
			// ReverseProxy otherwise forwards attacker-supplied forwarding headers.
			p.Out.Header.Del("Forwarded")
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Voice upstream unavailable", http.StatusBadGateway)
		},
		// Transport errors can contain the query token. Never log them verbatim.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	s := &Signaling{livekit: lk, presence: presence, origin: strings.TrimRight(origin, "/"), proxy: proxy}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 5 * time.Second
	proxy.Transport = admissionTransport{signaling: s, upstream: transport}
	return s, nil
}

func (s *Signaling) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.URL.Path {
	case "/voice/rtc", "/voice/rtc/v1", "/voice/rtc/validate", "/voice/rtc/v1/validate":
	default:
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	validation := strings.HasSuffix(r.URL.Path, "/validate")
	origin := r.Header.Get("Origin")
	if origin != s.origin && (!validation || origin != "") {
		http.Error(w, "Voice admission denied", http.StatusForbidden)
		return
	}
	if err := s.authorize(r); err != nil {
		http.Error(w, "Voice admission denied", http.StatusForbidden)
		return
	}
	s.proxy.ServeHTTP(w, r)
}

// Only the HTTP handshake holds the lock, not the long-lived WebSocket stream.
type admissionTransport struct {
	signaling *Signaling
	upstream  http.RoundTripper
}

func (t admissionTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.signaling.livekit.admission.RLock()
	defer t.signaling.livekit.admission.RUnlock()
	if err := t.signaling.authorize(r); err != nil {
		return nil, ErrNotPresent
	}
	return t.upstream.RoundTrip(r)
}

func (s *Signaling) authorize(r *http.Request) error {
	values := r.URL.Query()["access_token"]
	if len(values) != 1 || values[0] == "" {
		return ErrNotPresent
	}
	var c claims
	_, err := jwt.ParseWithClaims(values[0], &c, func(*jwt.Token) (any, error) { return s.livekit.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(s.livekit.key),
		jwt.WithExpirationRequired(), jwt.WithTimeFunc(s.livekit.now))
	if err != nil {
		return ErrNotPresent
	}
	g := c.Video
	if g == nil || !g.RoomJoin || g.RoomAdmin {
		return ErrNotPresent
	}
	roomRoot, _, ok := strings.Cut(g.Room, ":")
	if !ok {
		return ErrNotPresent
	}
	roomID, err := uuid.Parse(roomRoot)
	if err != nil {
		return ErrNotPresent
	}
	user, _, ok := strings.Cut(c.Subject, ":")
	if !ok {
		return ErrNotPresent
	}
	userID, err := uuid.Parse(user)
	if err != nil {
		return ErrNotPresent
	}
	m, err := s.presence.VoiceMember(r.Context(), roomID, userID)
	if err != nil {
		return err
	}
	if m.Identity == "" || c.Subject != m.Identity {
		return ErrNotPresent
	}
	canPublish := !m.Muted && !m.Broadcasting
	switch g.Room {
	case m.Channel:
	case broadcastRoom(m.Channel):
		canPublish = !m.Muted && m.Broadcasting
	default:
		return ErrNotPresent
	}
	if (g.CanPublish == nil || *g.CanPublish) && !canPublish {
		return errors.New("voice: stale publish permission")
	}
	return nil
}
