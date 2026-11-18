package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGuestTokenAudienceIsolation(t *testing.T) {
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	tokens := NewTokens(key, func() time.Time { return now })
	id := uuid.New()
	guest, err := tokens.IssueGuest(id, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Verify(guest); err == nil {
		t.Fatal("account verifier accepted guest token")
	}
	if got, err := tokens.VerifyGuest(guest); err != nil || got != id {
		t.Fatalf("guest verification: %v %v", got, err)
	}
	account, err := tokens.Issue(id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.VerifyGuest(account); err == nil {
		t.Fatal("guest verifier accepted account token")
	}
	expired, err := tokens.IssueGuest(id, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.VerifyGuest(expired); err == nil {
		t.Fatal("expired guest token accepted")
	}
}

func TestGuestTokenLifetimeBound(t *testing.T) {
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	now := time.Now().UTC()
	tokens := NewTokens(key, func() time.Time { return now })
	raw, err := tokens.IssueGuest(uuid.New(), now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(16 * time.Minute)
	if _, err := tokens.VerifyGuest(raw); err == nil {
		t.Fatal("guest issuer exceeded 15-minute lifetime")
	}
}
