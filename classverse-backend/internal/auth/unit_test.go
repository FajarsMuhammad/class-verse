package auth

import (
	"crypto/ed25519"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordHash(t *testing.T) {
	h, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash = %q, want PHC argon2id with OWASP params", h)
	}
	tests := []struct {
		name     string
		password string
		hash     string
		want     bool
		wantErr  bool
	}{
		{"correct", "correct horse battery", h, true, false},
		{"wrong", "wrong horse battery", h, false, false},
		{"malformed", "x", "$bcrypt$abc", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := verifyPassword(tt.password, tt.hash)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Errorf("verifyPassword() = %v, %v; want %v, err=%v", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestTokens(t *testing.T) {
	now := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	key := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	tokens := NewTokens(key, clock)
	user := uuid.New()

	raw, err := tokens.Issue(user)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tokens.Verify(raw)
	if err != nil || got != user {
		t.Fatalf("Verify() = %v, %v; want %v", got, err, user)
	}

	otherKey := ed25519.NewKeyFromSeed([]byte(strings.Repeat("x", ed25519.SeedSize)))
	forged, _ := NewTokens(otherKey, clock).Issue(user)
	if _, err := tokens.Verify(forged); err == nil {
		t.Error("Verify() accepted token signed with another key")
	}

	now = now.Add(accessTokenTTL + time.Second)
	if _, err := tokens.Verify(raw); err == nil {
		t.Error("Verify() accepted expired token")
	}
}

func TestOTPHash(t *testing.T) {
	code, err := generateOTP()
	if err != nil || len(code) != 6 {
		t.Fatalf("generateOTP() = %q, %v", code, err)
	}
	key := []byte("k")
	a, b := uuid.New(), uuid.New()
	if string(hashOTP(key, a, "123456")) == string(hashOTP(key, b, "123456")) {
		t.Error("hash must depend on user id")
	}
}

func TestValidation(t *testing.T) {
	now := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		email     string
		pass      string
		display   string
		birth     string
		wantField []string
	}{
		{"valid", "Ani@Example.com", "abcdefghij", " Ani ", "2009-02-28", nil},
		{"all invalid", "not-an-email", "short", "ab", "", []string{"email", "password", "display_name", "birth_date"}},
		{"future birth date", "a@b.co", "abcdefghij", "Ani", "2026-03-02", []string{"birth_date"}},
		{"bad date", "a@b.co", "abcdefghij", "Ani", "28-02-2009", []string{"birth_date"}},
		{"name too long", "a@b.co", "abcdefghij", strings.Repeat("é", 25), "2009-02-28", []string{"display_name"}},
		{"name 24 runes ok", "a@b.co", "abcdefghij", strings.Repeat("é", 24), "2009-02-28", nil},
		{"email with display name", "Ani <a@b.co>", "abcdefghij", "Ani", "2009-02-28", []string{"email"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v ValidationError
			if _, ok := normalizeEmail(tt.email); !ok {
				v.add("email", "x")
			}
			validatePassword(&v, tt.pass)
			validateDisplayName(&v, tt.display)
			validateBirthDate(&v, tt.birth, now)
			if len(v.Fields) != len(tt.wantField) {
				t.Fatalf("fields = %v, want %v", v.Fields, tt.wantField)
			}
			for _, f := range tt.wantField {
				if _, ok := v.Fields[f]; !ok {
					t.Errorf("missing field %q in %v", f, v.Fields)
				}
			}
		})
	}
	if e, _ := normalizeEmail(" Ani@Example.COM "); e != "ani@example.com" {
		t.Errorf("normalizeEmail = %q", e)
	}
}

func TestGoogleDisplayName(t *testing.T) {
	tests := []struct{ name, email, want string }{
		{"Ani Rahma", "ani@x.id", "Ani Rahma"},
		{strings.Repeat("a", 30), "ani@x.id", strings.Repeat("a", 24)},
		{"", "budi@x.id", "budi"},
		{"Al", "jo@x.id", "Pengguna"},
	}
	for _, tt := range tests {
		if got := googleDisplayName(tt.name, tt.email); got != tt.want {
			t.Errorf("googleDisplayName(%q, %q) = %q, want %q", tt.name, tt.email, got, tt.want)
		}
	}
}
