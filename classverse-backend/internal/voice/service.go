package voice

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Presence tells whether a user is connected to a classroom/garden right now
// (implemented in cmd/server over realtime.Hub). It returns ErrNotPresent
// when not, so door, lock, ban and schedule rules stay with WS admission.
type Presence interface {
	VoiceMember(ctx context.Context, roomID, userID uuid.UUID) (name string, muted bool, err error)
}

// Issuer signs participant tokens (implemented by LiveKit).
type Issuer interface {
	URL() string
	ParticipantToken(room, identity, name string, canPublish bool) (string, error)
}

// Domain errors.
var (
	ErrNotPresent = errors.New("voice: not in room")
	ErrDisabled   = errors.New("voice: not configured")
)

// Token is what the browser needs to join a room's voice.
type Token struct {
	URL   string
	Token string
}

// Service issues voice tokens.
type Service struct {
	presence Presence
	issuer   Issuer // nil when voice is off
}

// NewService builds the service; issuer may be nil (voice disabled).
func NewService(presence Presence, issuer Issuer) *Service {
	return &Service{presence: presence, issuer: issuer}
}

// Token issues a short-lived LiveKit token for roomID to a connected member.
func (s *Service) Token(ctx context.Context, roomID, userID uuid.UUID) (Token, error) {
	if s.issuer == nil {
		return Token{}, ErrDisabled
	}
	name, muted, err := s.presence.VoiceMember(ctx, roomID, userID)
	if err != nil {
		return Token{}, err
	}
	tok, err := s.issuer.ParticipantToken(roomID.String(), userID.String(), name, !muted)
	if err != nil {
		return Token{}, fmt.Errorf("voice token: %w", err)
	}
	return Token{URL: s.issuer.URL(), Token: tok}, nil
}
