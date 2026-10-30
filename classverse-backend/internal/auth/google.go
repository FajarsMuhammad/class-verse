package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const googleIssuer = "https://accounts.google.com"

// GoogleClient runs the Authorization Code + PKCE flow and verifies ID tokens.
type GoogleClient struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewGoogleClient discovers the provider at issuer (Google in production, a
// fake server in tests).
func NewGoogleClient(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*GoogleClient, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}
	return &GoogleClient{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		// Verifies iss, aud (client id), exp, and signature.
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// NewGoogleClientForGoogle discovers accounts.google.com.
func NewGoogleClientForGoogle(ctx context.Context, clientID, clientSecret, redirectURL string) (*GoogleClient, error) {
	return NewGoogleClient(ctx, googleIssuer, clientID, clientSecret, redirectURL)
}

// AuthURL returns the consent URL bound to state and the PKCE verifier.
func (g *GoogleClient) AuthURL(state, verifier string) string {
	return g.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
}

// Exchange trades the code for tokens and returns the verified identity.
func (g *GoogleClient) Exchange(ctx context.Context, code, verifier string) (GoogleIdentity, error) {
	tok, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("exchange code: %w", err)
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return GoogleIdentity{}, errors.New("token response has no id_token")
	}
	idToken, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return GoogleIdentity{}, fmt.Errorf("verify id token: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return GoogleIdentity{}, fmt.Errorf("decode id token claims: %w", err)
	}
	return GoogleIdentity{
		Subject:       idToken.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
	}, nil
}
