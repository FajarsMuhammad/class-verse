package auth

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	accessTokenTTL = 15 * time.Minute
	tokenIssuer    = "kelasverse"
	tokenAudience  = "kelasverse-web"
	// guestAudience is accepted only by guest endpoints, the guest's room socket,
	// and its voice token (G1); RequireUser never accepts it.
	guestAudience = "kelasverse-guest"
)

var errInvalidToken = errors.New("invalid access token")

// Tokens issues and verifies EdDSA access tokens. Roles are deliberately not
// in the token (ADR-005).
type Tokens struct {
	key ed25519.PrivateKey
	now func() time.Time
}

// NewTokens returns a token issuer for the given Ed25519 key.
func NewTokens(key ed25519.PrivateKey, now func() time.Time) *Tokens {
	return &Tokens{key: key, now: now}
}

// Issue returns a signed access token for userID.
func (t *Tokens) Issue(userID uuid.UUID) (string, error) {
	return t.sign(userID, tokenAudience, t.now().Add(accessTokenTTL))
}

// IssueGuest returns a guest token valid until expires (the guest link's expiry).
func (t *Tokens) IssueGuest(userID uuid.UUID, expires time.Time) (string, error) {
	if limit := t.now().Add(accessTokenTTL); expires.After(limit) {
		expires = limit
	}
	return t.sign(userID, guestAudience, expires)
}

// VerifyGuest checks a guest token and returns the guest's user id.
func (t *Tokens) VerifyGuest(raw string) (uuid.UUID, error) {
	return t.verify(raw, guestAudience)
}

func (t *Tokens) sign(userID uuid.UUID, audience string, expires time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(t.now()),
		ExpiresAt: jwt.NewNumericDate(expires),
		ID:        uuid.NewString(),
		Issuer:    tokenIssuer,
		Audience:  jwt.ClaimStrings{audience},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(t.key)
	if err != nil {
		return "", fmt.Errorf("sign access token: %w", err)
	}
	return s, nil
}

// Verify checks signature, expiry, issuer, and audience and returns the user id.
func (t *Tokens) Verify(raw string) (uuid.UUID, error) {
	return t.verify(raw, tokenAudience)
}

func (t *Tokens) verify(raw, audience string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(raw, &claims,
		func(*jwt.Token) (any, error) { return t.key.Public(), nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(tokenIssuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(t.now),
	)
	if err != nil {
		return uuid.Nil, errInvalidToken
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, errInvalidToken
	}
	return id, nil
}
