// Package voice issues LiveKit tokens for classroom/garden voice and calls the
// LiveKit RoomService for teacher mute and eviction (docs/design.md, slice 9).
// Audio flows between browsers and LiveKit; nothing is recorded here.
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// tokenTTL is short because a self-hosted LiveKit cannot revoke tokens;
	// it only gates the initial connect (LiveKit refreshes it while connected).
	tokenTTL     = 2 * time.Minute
	adminTTL     = time.Minute
	sourceMic    = "microphone"
	callTimeout  = 5 * time.Second
	responseCap  = 64 << 10
	twirpService = "/twirp/livekit.RoomService/"
)

// LiveKit signs participant and admin tokens and calls the RoomService
// (Twirp JSON over HTTP). Claims follow github.com/livekit/protocol/auth.
type LiveKit struct {
	api       string // RoomService base URL, http(s)://
	key       string
	secret    []byte
	http      *http.Client
	now       func() time.Time
	admission sync.RWMutex // serialize revocation against in-flight signaling handshakes
}

// NewLiveKit returns nil when url, key and secret are all empty (voice off).
func NewLiveKit(url, key, secret string, now func() time.Time) (*LiveKit, error) {
	if url == "" && key == "" && secret == "" {
		return nil, nil
	}
	if url == "" || key == "" || secret == "" {
		return nil, fmt.Errorf("voice: LIVEKIT_URL, LIVEKIT_API_KEY and LIVEKIT_API_SECRET must be set together")
	}
	api, ok := strings.CutPrefix(url, "ws")
	if !ok || (!strings.HasPrefix(api, "://") && !strings.HasPrefix(api, "s://")) {
		return nil, fmt.Errorf("voice: LIVEKIT_URL must start with ws:// or wss://")
	}
	return &LiveKit{
		api: "http" + strings.TrimRight(api, "/"), key: key, secret: []byte(secret),
		http: &http.Client{Timeout: callTimeout}, now: now,
	}, nil
}

type videoGrant struct {
	RoomJoin             bool     `json:"roomJoin,omitempty"`
	RoomAdmin            bool     `json:"roomAdmin,omitempty"`
	Room                 string   `json:"room,omitempty"`
	CanPublish           *bool    `json:"canPublish,omitempty"`
	CanSubscribe         *bool    `json:"canSubscribe,omitempty"`
	CanPublishData       *bool    `json:"canPublishData,omitempty"`
	CanPublishSources    []string `json:"canPublishSources,omitempty"`
	CanUpdateOwnMetadata *bool    `json:"canUpdateOwnMetadata,omitempty"`
}

type claims struct {
	jwt.RegisteredClaims
	Name  string      `json:"name,omitempty"`
	Video *videoGrant `json:"video"`
}

func (l *LiveKit) sign(identity, name string, ttl time.Duration, g *videoGrant) (string, error) {
	now := l.now()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: l.key, Subject: identity,
			IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Name: name, Video: g,
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(l.secret)
	if err != nil {
		return "", fmt.Errorf("sign livekit token: %w", err)
	}
	return s, nil
}

// ParticipantToken lets identity join room with audio only; publish is
// denied while the user is muted by a teacher.
func (l *LiveKit) ParticipantToken(room, identity, name string, canPublish bool) (string, error) {
	f := false
	g := &videoGrant{
		RoomJoin: true, Room: room,
		CanPublish: &canPublish, CanSubscribe: new(true), CanPublishData: &f, CanUpdateOwnMetadata: &f,
	}
	if canPublish {
		g.CanPublishSources = []string{sourceMic}
	}
	return l.sign(identity, name, tokenTTL, g)
}

// SetCanPublish grants or revokes microphone publishing for a connected
// participant (teacher mute). A participant that is not connected is not an error.
func (l *LiveKit) SetCanPublish(ctx context.Context, room, identity string, canPublish bool) error {
	perm := map[string]any{"can_subscribe": true, "can_publish": canPublish, "can_publish_data": false}
	if canPublish {
		perm["can_publish_sources"] = []string{"MICROPHONE"}
	}
	return l.call(ctx, room, "UpdateParticipant", map[string]any{"room": room, "identity": identity, "permission": perm})
}

// Remove disconnects identity from room (kick or leaving the room).
func (l *LiveKit) Remove(ctx context.Context, room, identity string) error {
	// Handshakes must finish before eviction, otherwise a delayed join can land
	// after GetParticipant confirmed absence. This lock never runs on room actors.
	l.admission.Lock()
	defer l.admission.Unlock()
	body := map[string]any{"room": room, "identity": identity}
	if err := l.call(ctx, room, "RemoveParticipant", body); err != nil {
		return err
	}
	err := l.call(ctx, room, "GetParticipant", body)
	if errors.Is(err, errParticipantAbsent) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("livekit removal not yet confirmed")
}

var errParticipantAbsent = errors.New("livekit participant absent")

func (l *LiveKit) call(ctx context.Context, room, method string, body any) error {
	tok, err := l.sign("", "", adminTTL, &videoGrant{RoomAdmin: true, Room: room})
	if err != nil {
		return err
	}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("livekit %s: %w", method, err)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.api+twirpService+method, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("livekit %s: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	res, err := l.http.Do(req)
	if err != nil {
		return fmt.Errorf("livekit %s: %w", method, err)
	}
	defer func() { _ = res.Body.Close() }()
	msg, _ := io.ReadAll(io.LimitReader(res.Body, responseCap))
	if res.StatusCode == http.StatusOK {
		return nil
	}
	var te struct {
		Code string `json:"code"`
	}
	// The participant already left: nothing to mute or remove.
	if json.Unmarshal(msg, &te) == nil && te.Code == "not_found" {
		if method == "GetParticipant" {
			return errParticipantAbsent
		}
		return nil
	}
	return fmt.Errorf("livekit %s: status %d code %q", method, res.StatusCode, te.Code)
}
