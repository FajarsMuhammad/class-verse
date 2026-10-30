package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/mail"
)

const (
	otpTTL          = 10 * time.Minute
	otpCooldown     = 60 * time.Second
	otpMaxAttempts  = 5
	refreshTokenTTL = 30 * 24 * time.Hour
	refreshTokenLen = 32
)

// Mailer sends transactional email.
type Mailer interface {
	Send(ctx context.Context, msg mail.Message) error
}

// Session is the result of a successful login: a short-lived access token
// for the frontend and an opaque refresh token for the cookie.
type Session struct {
	AccessToken  string
	RefreshToken string
}

// Profile is the current user as returned by GET /api/me.
type Profile struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	BirthDate    *time.Time
	AvatarKey    string
	NeedsProfile bool
}

// RegisterInput is the body of POST /api/auth/register.
type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
	BirthDate   string
}

// ProfileInput holds optional profile changes; nil fields stay unchanged.
type ProfileInput struct {
	DisplayName *string
	BirthDate   *string
}

// GoogleIdentity is the verified subset of a Google ID token.
type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

// Service implements registration, login, sessions, and profile rules.
type Service struct {
	store     *Store
	tokens    *Tokens
	mailer    Mailer
	otpKey    []byte
	now       func() time.Time
	log       *slog.Logger
	dummyHash string
}

// NewService wires the auth service.
func NewService(store *Store, tokens *Tokens, mailer Mailer, otpKey []byte, now func() time.Time, log *slog.Logger) (*Service, error) {
	// Login for unknown emails verifies against this hash so the response
	// time does not reveal whether the email exists.
	dummy, err := hashPassword("kelasverse-dummy-password")
	if err != nil {
		return nil, fmt.Errorf("create dummy hash: %w", err)
	}
	return &Service{store: store, tokens: tokens, mailer: mailer, otpKey: otpKey, now: now, log: log, dummyHash: dummy}, nil
}

// Register creates or refreshes an unverified account and emails an OTP.
// It never reveals whether the email is already registered.
func (s *Service) Register(ctx context.Context, in RegisterInput) error {
	var v ValidationError
	email, ok := normalizeEmail(in.Email)
	if !ok {
		v.add("email", "Email tidak valid")
	}
	validatePassword(&v, in.Password)
	name := validateDisplayName(&v, in.DisplayName)
	birth := validateBirthDate(&v, in.BirthDate, s.now())
	if err := v.orNil(); err != nil {
		return err
	}

	hash, err := hashPassword(in.Password)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}

	var (
		msg       *mail.Message
		newUserID uuid.UUID
	)
	err = s.store.InTx(ctx, func(q *query.Queries) error {
		u, err := q.GetUserByEmail(ctx, email)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			u, err = q.CreateUser(ctx, query.CreateUserParams{
				Email:        email,
				PasswordHash: pgtype.Text{String: hash, Valid: true},
				DisplayName:  name,
				BirthDate:    pgDate(birth),
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}
			newUserID = u.ID
		case err != nil:
			return fmt.Errorf("get user: %w", err)
		case u.EmailVerifiedAt.Valid:
			m := mail.AccountExistsMessage(email)
			msg = &m
			return nil
		default:
			err = q.UpdatePendingUser(ctx, query.UpdatePendingUserParams{
				PasswordHash: pgtype.Text{String: hash, Valid: true},
				DisplayName:  name,
				BirthDate:    pgDate(birth),
				Now:          s.pgNow(),
				ID:           u.ID,
			})
			if err != nil {
				return fmt.Errorf("update pending user: %w", err)
			}
		}
		msg, err = s.issueOTP(ctx, q, u.ID, email)
		return err
	})
	if isUniqueViolation(err) {
		// A concurrent registration for the same email won; it sends the OTP.
		return nil
	}
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if newUserID != uuid.Nil {
		s.log.InfoContext(ctx, "user registered", slog.String("user_id", newUserID.String()))
	}
	return s.send(ctx, msg)
}

// ResendOTP sends a fresh code to an unverified account. Like Register it
// gives the same answer whether or not the email exists.
func (s *Service) ResendOTP(ctx context.Context, rawEmail string) error {
	email, ok := normalizeEmail(rawEmail)
	if !ok {
		return &ValidationError{Fields: map[string]string{"email": "Email tidak valid"}}
	}
	var msg *mail.Message
	err := s.store.InTx(ctx, func(q *query.Queries) error {
		u, err := q.GetUserByEmail(ctx, email)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil
		case err != nil:
			return fmt.Errorf("get user: %w", err)
		case u.EmailVerifiedAt.Valid:
			m := mail.AccountExistsMessage(email)
			msg = &m
			return nil
		}
		msg, err = s.issueOTP(ctx, q, u.ID, email)
		return err
	})
	if err != nil {
		return fmt.Errorf("resend otp: %w", err)
	}
	return s.send(ctx, msg)
}

// issueOTP replaces any active code with a new one, unless the last code was
// sent less than otpCooldown ago. It returns the email to send, if any.
func (s *Service) issueOTP(ctx context.Context, q *query.Queries, userID uuid.UUID, email string) (*mail.Message, error) {
	now := s.now()
	last, err := q.GetLatestOTP(ctx, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get latest otp: %w", err)
	}
	if err == nil {
		sentAt := last.ExpiresAt.Time.Add(-otpTTL)
		if now.Sub(sentAt) < otpCooldown {
			return nil, nil
		}
	}
	code, err := generateOTP()
	if err != nil {
		return nil, err
	}
	if err := q.ConsumeActiveOTPs(ctx, query.ConsumeActiveOTPsParams{Now: s.pgNow(), UserID: userID}); err != nil {
		return nil, fmt.Errorf("consume old otps: %w", err)
	}
	err = q.CreateOTP(ctx, query.CreateOTPParams{
		UserID:    userID,
		CodeHash:  hashOTP(s.otpKey, userID, code),
		ExpiresAt: pgTime(now.Add(otpTTL)),
	})
	if err != nil {
		return nil, fmt.Errorf("create otp: %w", err)
	}
	m := mail.OTPMessage(email, code)
	return &m, nil
}

func (s *Service) send(ctx context.Context, msg *mail.Message) error {
	if msg == nil {
		return nil
	}
	if err := s.mailer.Send(ctx, *msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

// VerifyEmail checks the OTP, marks the email verified, and logs the user in.
func (s *Service) VerifyEmail(ctx context.Context, rawEmail, code string) (Session, error) {
	email, ok := normalizeEmail(rawEmail)
	code = strings.TrimSpace(code)
	if !ok || utf8.RuneCountInString(code) != 6 {
		return Session{}, ErrInvalidOTP
	}

	var (
		outcome error
		refresh string
		userID  uuid.UUID
	)
	err := s.store.InTx(ctx, func(q *query.Queries) error {
		u, err := q.GetUserByEmail(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = ErrInvalidOTP
			return nil
		}
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if u.EmailVerifiedAt.Valid {
			outcome = ErrInvalidOTP
			return nil
		}
		otp, err := q.GetActiveOTPForUpdate(ctx, u.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = ErrInvalidOTP
			return nil
		}
		if err != nil {
			return fmt.Errorf("get otp: %w", err)
		}
		switch {
		case !s.now().Before(otp.ExpiresAt.Time):
			outcome = ErrOTPExpired
			return nil
		case otp.Attempts >= otpMaxAttempts:
			outcome = ErrTooManyAttempts
			return nil
		}
		if subtle.ConstantTimeCompare(hashOTP(s.otpKey, u.ID, code), otp.CodeHash) != 1 {
			attempts, err := q.IncrementOTPAttempts(ctx, otp.ID)
			if err != nil {
				return fmt.Errorf("increment otp attempts: %w", err)
			}
			outcome = ErrInvalidOTP
			if attempts >= otpMaxAttempts {
				outcome = ErrTooManyAttempts
			}
			return nil // commit the attempt counter
		}
		if err := q.ConsumeOTP(ctx, query.ConsumeOTPParams{Now: s.pgNow(), ID: otp.ID}); err != nil {
			return fmt.Errorf("consume otp: %w", err)
		}
		if err := q.MarkEmailVerified(ctx, query.MarkEmailVerifiedParams{Now: s.pgNow(), ID: u.ID}); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		userID = u.ID
		refresh, err = s.createRefreshToken(ctx, q, u.ID, uuid.New())
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("verify email: %w", err)
	}
	if outcome != nil {
		return Session{}, outcome
	}
	s.log.InfoContext(ctx, "email verified", slog.String("user_id", userID.String()))
	return s.session(userID, refresh)
}

// Login checks email and password. Unknown emails and wrong passwords give
// the same error and take the same time.
func (s *Service) Login(ctx context.Context, rawEmail, password string) (Session, error) {
	email, _ := normalizeEmail(rawEmail)
	u, err := s.store.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, fmt.Errorf("login: get user: %w", err)
	}
	if err != nil || !u.PasswordHash.Valid {
		_, _ = verifyPassword(password, s.dummyHash)
		return Session{}, ErrInvalidCredentials
	}
	ok, err := verifyPassword(password, u.PasswordHash.String)
	if err != nil {
		return Session{}, fmt.Errorf("login: verify password: %w", err)
	}
	if !ok {
		s.log.InfoContext(ctx, "login failed", slog.String("user_id", u.ID.String()))
		return Session{}, ErrInvalidCredentials
	}
	if !u.EmailVerifiedAt.Valid {
		return Session{}, ErrEmailNotVerified
	}
	return s.startSession(ctx, u.ID)
}

// Refresh rotates a refresh token. Reusing a token that was already rotated
// revokes the whole family: someone else may hold a copy.
func (s *Service) Refresh(ctx context.Context, raw string) (Session, error) {
	if raw == "" {
		return Session{}, ErrUnauthenticated
	}
	var (
		outcome error
		next    string
		userID  uuid.UUID
	)
	err := s.store.InTx(ctx, func(q *query.Queries) error {
		t, err := q.GetRefreshTokenForUpdate(ctx, hashToken(raw))
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = ErrUnauthenticated
			return nil
		}
		if err != nil {
			return fmt.Errorf("get refresh token: %w", err)
		}
		if t.UsedAt.Valid || t.RevokedAt.Valid {
			if err := q.RevokeRefreshFamily(ctx, query.RevokeRefreshFamilyParams{Now: s.pgNow(), FamilyID: t.FamilyID}); err != nil {
				return fmt.Errorf("revoke family: %w", err)
			}
			if !t.RevokedAt.Valid {
				s.log.WarnContext(ctx, "refresh reuse detected", slog.String("user_id", t.UserID.String()))
			}
			outcome = ErrSessionRevoked
			return nil
		}
		if !s.now().Before(t.ExpiresAt.Time) {
			outcome = ErrUnauthenticated
			return nil
		}
		if err := q.MarkRefreshTokenUsed(ctx, query.MarkRefreshTokenUsedParams{Now: s.pgNow(), ID: t.ID}); err != nil {
			return fmt.Errorf("mark used: %w", err)
		}
		userID = t.UserID
		next, err = s.createRefreshToken(ctx, q, t.UserID, t.FamilyID)
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("refresh: %w", err)
	}
	if outcome != nil {
		return Session{}, outcome
	}
	return s.session(userID, next)
}

// Logout revokes the family of the given refresh token. Unknown tokens are ignored.
func (s *Service) Logout(ctx context.Context, raw string) error {
	if raw == "" {
		return nil
	}
	t, err := s.store.GetRefreshToken(ctx, hashToken(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("logout: get token: %w", err)
	}
	if err := s.store.RevokeRefreshFamily(ctx, query.RevokeRefreshFamilyParams{Now: s.pgNow(), FamilyID: t.FamilyID}); err != nil {
		return fmt.Errorf("logout: revoke family: %w", err)
	}
	return nil
}

// GoogleLogin finds, links, or creates the account for a Google identity.
func (s *Service) GoogleLogin(ctx context.Context, id GoogleIdentity) (Session, error) {
	email, ok := normalizeEmail(id.Email)
	if !id.EmailVerified || !ok || id.Subject == "" {
		return Session{}, ErrGoogleUnverified
	}
	sub := pgtype.Text{String: id.Subject, Valid: true}

	var (
		userID  uuid.UUID
		outcome error
		created bool
	)
	err := s.store.InTx(ctx, func(q *query.Queries) error {
		u, err := q.GetUserByGoogleSub(ctx, sub)
		if err == nil {
			userID = u.ID
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("get user by google sub: %w", err)
		}

		u, err = q.GetUserByEmail(ctx, email)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			u, err = q.CreateGoogleUser(ctx, query.CreateGoogleUserParams{
				Email:       email,
				Now:         s.pgNow(),
				GoogleSub:   sub,
				DisplayName: googleDisplayName(id.Name, email),
			})
			if err != nil {
				return fmt.Errorf("create google user: %w", err)
			}
			created = true
		case err != nil:
			return fmt.Errorf("get user by email: %w", err)
		case u.GoogleSub.Valid:
			outcome = ErrAccountConflict
			return nil
		default:
			if !u.EmailVerifiedAt.Valid {
				// The pending registration never proved the email; drop its OTPs.
				if err := q.ConsumeActiveOTPs(ctx, query.ConsumeActiveOTPsParams{Now: s.pgNow(), UserID: u.ID}); err != nil {
					return fmt.Errorf("consume otps: %w", err)
				}
			}
			u, err = q.LinkGoogleAccount(ctx, query.LinkGoogleAccountParams{GoogleSub: sub, Now: s.pgNow(), ID: u.ID})
			if err != nil {
				return fmt.Errorf("link google: %w", err)
			}
		}
		userID = u.ID
		return nil
	})
	if isUniqueViolation(err) {
		return Session{}, ErrAccountConflict
	}
	if err != nil {
		return Session{}, fmt.Errorf("google login: %w", err)
	}
	if outcome != nil {
		return Session{}, outcome
	}
	if created {
		s.log.InfoContext(ctx, "user registered", slog.String("user_id", userID.String()), slog.String("method", "google"))
	}
	return s.startSession(ctx, userID)
}

// Me returns the profile of userID.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (Profile, error) {
	u, err := s.store.GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrUserNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("get me: %w", err)
	}
	return toProfile(u), nil
}

// UpdateProfile changes the display name and/or birth date of userID.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, in ProfileInput) (Profile, error) {
	var (
		v      ValidationError
		params = query.UpdateProfileParams{Now: s.pgNow(), ID: userID}
	)
	if in.DisplayName != nil {
		name := validateDisplayName(&v, *in.DisplayName)
		params.DisplayName = pgtype.Text{String: name, Valid: true}
	}
	if in.BirthDate != nil {
		params.BirthDate = pgDate(validateBirthDate(&v, *in.BirthDate, s.now()))
	}
	if err := v.orNil(); err != nil {
		return Profile{}, err
	}
	u, err := s.store.UpdateProfile(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrUserNotFound
	}
	if err != nil {
		return Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return toProfile(u), nil
}

func (s *Service) startSession(ctx context.Context, userID uuid.UUID) (Session, error) {
	refresh, err := s.createRefreshToken(ctx, s.store.Queries, userID, uuid.New())
	if err != nil {
		return Session{}, err
	}
	return s.session(userID, refresh)
}

func (s *Service) session(userID uuid.UUID, refresh string) (Session, error) {
	access, err := s.tokens.Issue(userID)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *Service) createRefreshToken(ctx context.Context, q *query.Queries, userID, familyID uuid.UUID) (string, error) {
	b := make([]byte, refreshTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read refresh token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	now := s.now()
	err := q.CreateRefreshToken(ctx, query.CreateRefreshTokenParams{
		UserID:    userID,
		FamilyID:  familyID,
		TokenHash: hashToken(raw),
		ExpiresAt: pgTime(now.Add(refreshTokenTTL)),
		CreatedAt: pgTime(now),
	})
	if err != nil {
		return "", fmt.Errorf("create refresh token: %w", err)
	}
	return raw, nil
}

func (s *Service) pgNow() pgtype.Timestamptz {
	return pgTime(s.now())
}

func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func googleDisplayName(name, email string) string {
	name = truncateRunes(strings.TrimSpace(name), maxDisplayNameLen)
	if utf8.RuneCountInString(name) >= minDisplayNameLen {
		return name
	}
	local := truncateRunes(email[:strings.IndexByte(email, '@')], maxDisplayNameLen)
	if utf8.RuneCountInString(local) >= minDisplayNameLen {
		return local
	}
	return "Pengguna"
}

func toProfile(u query.User) Profile {
	p := Profile{
		ID:           u.ID,
		Email:        u.Email,
		DisplayName:  u.DisplayName,
		AvatarKey:    u.AvatarKey,
		NeedsProfile: !u.BirthDate.Valid,
	}
	if u.BirthDate.Valid {
		d := u.BirthDate.Time
		p.BirthDate = &d
	}
	return p
}

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func pgDate(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
