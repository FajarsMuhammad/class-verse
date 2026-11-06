package voice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Presence tells whether a user is connected to a classroom/garden right now
// (implemented in cmd/server over realtime.Hub). It returns ErrNotPresent
// when not, so door, lock, ban and schedule rules stay with WS admission.
type Presence interface {
	VoiceMember(ctx context.Context, roomID, userID uuid.UUID) (Member, error)
}

// Member is an actor-issued admission. Identity changes after every revocation.
type Member struct {
	Name         string
	Muted        bool
	Identity     string
	Channel      string
	Broadcasting bool
}

// Issuer signs participant tokens (implemented by LiveKit).
type Issuer interface {
	ParticipantToken(room, identity, name string, canPublish bool) (string, error)
}

// Domain errors.
var (
	ErrNotPresent = errors.New("voice: not in room")
	ErrDisabled   = errors.New("voice: not configured")
	ErrPending    = errors.New("voice: admission pending")
)

// Token is what the browser needs to join a room's voice.
type Token struct {
	URL            string
	Token          string
	BroadcastToken string
	Identity       string
	Channel        string
}

func broadcastRoom(channel string) string {
	base, _, _ := strings.Cut(channel, ":zone:")
	return base + ":broadcast"
}

// Service issues voice tokens.
type Service struct {
	presence  Presence
	issuer    Issuer // nil when voice is off
	signalURL string // public admission gateway, never LiveKit's private URL
}

// NewService builds the service; issuer may be nil (voice disabled).
func NewService(presence Presence, issuer Issuer, signalURL string) *Service {
	return &Service{presence: presence, issuer: issuer, signalURL: signalURL}
}

// Token issues a short-lived LiveKit token for roomID to a connected member.
func (s *Service) Token(ctx context.Context, roomID, userID uuid.UUID) (Token, error) {
	if s.issuer == nil {
		return Token{}, ErrDisabled
	}
	m, err := s.presence.VoiceMember(ctx, roomID, userID)
	if err != nil {
		return Token{}, err
	}
	if m.Identity == "" || m.Channel == "" {
		return Token{}, ErrPending
	}
	tok, err := s.issuer.ParticipantToken(m.Channel, m.Identity, m.Name, !m.Muted && !m.Broadcasting)
	if err != nil {
		return Token{}, fmt.Errorf("voice token: %w", err)
	}
	broadcast, err := s.issuer.ParticipantToken(broadcastRoom(m.Channel), m.Identity, m.Name, !m.Muted && m.Broadcasting)
	if err != nil {
		return Token{}, fmt.Errorf("broadcast token: %w", err)
	}
	return Token{URL: s.signalURL, Token: tok, BroadcastToken: broadcast, Identity: m.Identity, Channel: m.Channel}, nil
}
