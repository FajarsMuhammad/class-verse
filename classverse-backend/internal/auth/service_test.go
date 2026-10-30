package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/mail"
)

type fakeMailer struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (m *fakeMailer) Send(_ context.Context, msg mail.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

var codeRe = regexp.MustCompile(`\b(\d{6})\b`)

// lastCode returns the OTP from the most recent email, or "".
func (m *fakeMailer) lastCode() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return ""
	}
	return codeRe.FindString(m.sent[len(m.sent)-1].Body)
}

func (m *fakeMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type env struct {
	svc    *Service
	store  *Store
	tokens *Tokens
	mailer *fakeMailer
	clock  *clock
}

func newEnv(t *testing.T) *env {
	t.Helper()
	pool := dbtest.New(t)
	c := &clock{t: time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)}
	tokens := NewTokens(ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)), c.Now)
	m := &fakeMailer{}
	store := NewStore(pool)
	svc, err := NewService(store, tokens, m, []byte(strings.Repeat("k", 32)), c.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return &env{svc: svc, store: store, tokens: tokens, mailer: m, clock: c}
}

func validRegister(email string) RegisterInput {
	return RegisterInput{Email: email, Password: "rahasia-123", DisplayName: "Ani", BirthDate: "2008-05-17"}
}

// registerVerified creates a verified user and returns its session.
func (e *env) registerVerified(t *testing.T, email string) Session {
	t.Helper()
	ctx := context.Background()
	if err := e.svc.Register(ctx, validRegister(email)); err != nil {
		t.Fatal(err)
	}
	sess, err := e.svc.VerifyEmail(ctx, email, e.mailer.lastCode())
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestRegisterAndVerify(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	if err := e.svc.Register(ctx, validRegister("Ani@Example.com")); err != nil {
		t.Fatalf("Register() = %v", err)
	}
	code := e.mailer.lastCode()
	if code == "" {
		t.Fatal("no OTP sent")
	}
	if _, err := e.svc.Login(ctx, "ani@example.com", "rahasia-123"); !errors.Is(err, ErrEmailNotVerified) {
		t.Errorf("Login() before verify = %v, want ErrEmailNotVerified", err)
	}
	sess, err := e.svc.VerifyEmail(ctx, "ANI@example.com", code)
	if err != nil {
		t.Fatalf("VerifyEmail() = %v", err)
	}
	if sess.AccessToken == "" || sess.RefreshToken == "" {
		t.Fatal("empty session")
	}
	if _, err := e.svc.VerifyEmail(ctx, "ani@example.com", code); !errors.Is(err, ErrInvalidOTP) {
		t.Errorf("reuse OTP = %v, want ErrInvalidOTP", err)
	}

	// Registering a verified email sends a notice, not a code, and still succeeds.
	before := e.mailer.count()
	if err := e.svc.Register(ctx, validRegister("ani@example.com")); err != nil {
		t.Fatalf("Register() existing = %v", err)
	}
	if e.mailer.count() != before+1 || e.mailer.lastCode() != "" {
		t.Error("existing account should get one notice without a code")
	}
}

func TestRegisterValidation(t *testing.T) {
	e := newEnv(t)
	in := validRegister("x")
	in.Password = "short"
	err := e.svc.Register(context.Background(), in)
	var verr *ValidationError
	if !errors.As(err, &verr) || verr.Fields["email"] == "" || verr.Fields["password"] == "" {
		t.Fatalf("Register() = %v, want validation for email and password", err)
	}
}

func TestVerifyEmailOTPRules(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, e *env, code string) error
		want error
	}{
		{
			name: "expired",
			run: func(_ *testing.T, e *env, code string) error {
				e.clock.Advance(otpTTL)
				_, err := e.svc.VerifyEmail(context.Background(), "a@b.co", code)
				return err
			},
			want: ErrOTPExpired,
		},
		{
			name: "five wrong attempts lock the code",
			run: func(t *testing.T, e *env, code string) error {
				wrong := "000000"
				if code == wrong {
					wrong = "111111"
				}
				for i := range otpMaxAttempts {
					_, err := e.svc.VerifyEmail(context.Background(), "a@b.co", wrong)
					want := ErrInvalidOTP
					if i == otpMaxAttempts-1 {
						want = ErrTooManyAttempts
					}
					if !errors.Is(err, want) {
						t.Fatalf("attempt %d = %v, want %v", i+1, err, want)
					}
				}
				_, err := e.svc.VerifyEmail(context.Background(), "a@b.co", code)
				return err
			},
			want: ErrTooManyAttempts,
		},
		{
			name: "unknown email",
			run: func(_ *testing.T, e *env, code string) error {
				_, err := e.svc.VerifyEmail(context.Background(), "nobody@b.co", code)
				return err
			},
			want: ErrInvalidOTP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			if err := e.svc.Register(context.Background(), validRegister("a@b.co")); err != nil {
				t.Fatal(err)
			}
			if err := tt.run(t, e, e.mailer.lastCode()); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}

func TestResendOTPCooldown(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.svc.Register(ctx, validRegister("a@b.co")); err != nil {
		t.Fatal(err)
	}
	first := e.mailer.lastCode()

	if err := e.svc.ResendOTP(ctx, "a@b.co"); err != nil {
		t.Fatal(err)
	}
	if e.mailer.count() != 1 {
		t.Errorf("resend within cooldown sent %d emails, want 1", e.mailer.count())
	}

	e.clock.Advance(otpCooldown)
	if err := e.svc.ResendOTP(ctx, "a@b.co"); err != nil {
		t.Fatal(err)
	}
	if e.mailer.count() != 2 {
		t.Fatalf("resend after cooldown sent %d emails, want 2", e.mailer.count())
	}
	second := e.mailer.lastCode()
	if first != second {
		if _, err := e.svc.VerifyEmail(ctx, "a@b.co", first); !errors.Is(err, ErrInvalidOTP) {
			t.Errorf("old OTP = %v, want ErrInvalidOTP", err)
		}
	}
	if _, err := e.svc.VerifyEmail(ctx, "a@b.co", second); err != nil {
		t.Errorf("new OTP = %v", err)
	}

	if err := e.svc.ResendOTP(ctx, "nobody@b.co"); err != nil {
		t.Errorf("ResendOTP(unknown) = %v, want nil", err)
	}
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	e.registerVerified(t, "a@b.co")
	tests := []struct {
		name, email, pass string
		want              error
	}{
		{"correct", "A@B.co", "rahasia-123", nil},
		{"wrong password", "a@b.co", "rahasia-124", ErrInvalidCredentials},
		{"unknown email", "x@b.co", "rahasia-123", ErrInvalidCredentials},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.svc.Login(context.Background(), tt.email, tt.pass)
			if !errors.Is(err, tt.want) {
				t.Errorf("Login() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRefreshRotation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s0 := e.registerVerified(t, "a@b.co")
	other, err := e.svc.Login(ctx, "a@b.co", "rahasia-123") // separate family
	if err != nil {
		t.Fatal(err)
	}

	s1, err := e.svc.Refresh(ctx, s0.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh = %v", err)
	}
	if s1.RefreshToken == s0.RefreshToken {
		t.Fatal("refresh token not rotated")
	}

	// Reusing the rotated token revokes the family, including s1.
	if _, err := e.svc.Refresh(ctx, s0.RefreshToken); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("reuse = %v, want ErrSessionRevoked", err)
	}
	if _, err := e.svc.Refresh(ctx, s1.RefreshToken); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("descendant after reuse = %v, want ErrSessionRevoked", err)
	}
	// Other login families are untouched.
	if _, err := e.svc.Refresh(ctx, other.RefreshToken); err != nil {
		t.Errorf("other family = %v, want nil", err)
	}
	if _, err := e.svc.Refresh(ctx, "nonsense"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("unknown token = %v, want ErrUnauthenticated", err)
	}
}

func TestRefreshExpiredAndLogout(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.registerVerified(t, "a@b.co")

	if err := e.svc.Logout(ctx, s.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Refresh(ctx, s.RefreshToken); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("after logout = %v, want ErrSessionRevoked", err)
	}

	s2, err := e.svc.Login(ctx, "a@b.co", "rahasia-123")
	if err != nil {
		t.Fatal(err)
	}
	e.clock.Advance(refreshTokenTTL)
	if _, err := e.svc.Refresh(ctx, s2.RefreshToken); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("expired = %v, want ErrUnauthenticated", err)
	}
}

func TestRefreshConcurrentRotation(t *testing.T) {
	e := newEnv(t)
	s := e.registerVerified(t, "a@b.co")

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Go(func() {
			_, results[i] = e.svc.Refresh(context.Background(), s.RefreshToken)
		})
	}
	wg.Wait()
	ok := 0
	for _, err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Errorf("concurrent refresh succeeded %d times, want exactly 1 (%v)", ok, results)
	}
}

func TestGoogleLinking(t *testing.T) {
	ctx := context.Background()
	gid := func(sub, email string) GoogleIdentity {
		return GoogleIdentity{Subject: sub, Email: email, EmailVerified: true, Name: "Ani Google"}
	}

	t.Run("same google sub logs in", func(t *testing.T) {
		e := newEnv(t)
		s1, err := e.svc.GoogleLogin(ctx, gid("g1", "a@b.co"))
		if err != nil {
			t.Fatal(err)
		}
		s2, err := e.svc.GoogleLogin(ctx, gid("g1", "a@b.co"))
		if err != nil {
			t.Fatal(err)
		}
		u1, _ := e.tokens.Verify(s1.AccessToken)
		u2, _ := e.tokens.Verify(s2.AccessToken)
		if u1 != u2 {
			t.Error("same google sub created two users")
		}
	})

	t.Run("verified email account is linked", func(t *testing.T) {
		e := newEnv(t)
		local := e.registerVerified(t, "a@b.co")
		g, err := e.svc.GoogleLogin(ctx, gid("g1", "A@b.co"))
		if err != nil {
			t.Fatal(err)
		}
		u1, _ := e.tokens.Verify(local.AccessToken)
		u2, _ := e.tokens.Verify(g.AccessToken)
		if u1 != u2 {
			t.Fatal("verified account not linked")
		}
		if _, err := e.svc.Login(ctx, "a@b.co", "rahasia-123"); err != nil {
			t.Errorf("password login after link = %v, want nil", err)
		}
	})

	t.Run("unverified email account loses password", func(t *testing.T) {
		e := newEnv(t)
		if err := e.svc.Register(ctx, validRegister("a@b.co")); err != nil {
			t.Fatal(err)
		}
		code := e.mailer.lastCode()
		g, err := e.svc.GoogleLogin(ctx, gid("g1", "a@b.co"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Login(ctx, "a@b.co", "rahasia-123"); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("squatter password = %v, want ErrInvalidCredentials", err)
		}
		if _, err := e.svc.VerifyEmail(ctx, "a@b.co", code); !errors.Is(err, ErrInvalidOTP) {
			t.Errorf("squatter OTP = %v, want ErrInvalidOTP", err)
		}
		id, _ := e.tokens.Verify(g.AccessToken)
		p, err := e.svc.Me(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if p.Email != "a@b.co" {
			t.Errorf("email = %q", p.Email)
		}
	})

	t.Run("new user needs profile", func(t *testing.T) {
		e := newEnv(t)
		g, err := e.svc.GoogleLogin(ctx, gid("g1", "new@b.co"))
		if err != nil {
			t.Fatal(err)
		}
		id, _ := e.tokens.Verify(g.AccessToken)
		p, err := e.svc.Me(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if !p.NeedsProfile || p.DisplayName != "Ani Google" || p.AvatarKey != "default" {
			t.Errorf("profile = %+v", p)
		}
	})

	t.Run("unverified google email rejected", func(t *testing.T) {
		e := newEnv(t)
		id := gid("g1", "a@b.co")
		id.EmailVerified = false
		if _, err := e.svc.GoogleLogin(ctx, id); !errors.Is(err, ErrGoogleUnverified) {
			t.Errorf("got %v, want ErrGoogleUnverified", err)
		}
	})

	t.Run("email already linked to another google account", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.svc.GoogleLogin(ctx, gid("g1", "a@b.co")); err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.GoogleLogin(ctx, gid("g2", "a@b.co")); !errors.Is(err, ErrAccountConflict) {
			t.Errorf("got %v, want ErrAccountConflict", err)
		}
	})
}

func TestUpdateProfile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	g, err := e.svc.GoogleLogin(ctx, GoogleIdentity{Subject: "g1", Email: "a@b.co", EmailVerified: true, Name: "Ani"})
	if err != nil {
		t.Fatal(err)
	}
	id, _ := e.tokens.Verify(g.AccessToken)

	birth := "2009-01-31"
	p, err := e.svc.UpdateProfile(ctx, id, ProfileInput{BirthDate: &birth})
	if err != nil {
		t.Fatal(err)
	}
	if p.NeedsProfile || p.BirthDate.Format(time.DateOnly) != birth || p.DisplayName != "Ani" {
		t.Errorf("profile = %+v", p)
	}

	short := "Al"
	if _, err := e.svc.UpdateProfile(ctx, id, ProfileInput{DisplayName: &short}); !errors.As(err, new(*ValidationError)) {
		t.Errorf("short name = %v, want ValidationError", err)
	}
	if _, err := e.svc.Me(ctx, uuid.New()); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("Me(unknown) = %v", err)
	}
}
