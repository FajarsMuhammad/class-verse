// Package guest lets a host invite people into one meeting room without an
// account (G1, ADR-012). A guest is a users row flagged is_guest that never
// gets a membership; access comes only from an approved request of an active
// link to that room.
package guest

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/invite"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// Product limits approved for G1.
const (
	MaxHours     = 24
	MaxGuests    = 40
	defaultHours = 2
	// tokenTTL keeps guest tokens short; the guest page renews by polling Status.
	tokenTTL = 15 * time.Minute
)

// Domain errors.
var (
	ErrNotFound = errors.New("guest: not found")
	ErrExpired  = errors.New("guest: link inactive")
	ErrKicked   = errors.New("guest: kicked")
)

// ValidationError carries per-field messages for the client.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "guest: invalid fields" }

// Authorizer checks organization roles (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// Ender disconnects guests whose access ended (implemented by realtime.Hub).
type Ender interface {
	EndGuest(userID uuid.UUID)
}

// Issuer signs guest tokens (implemented by auth.Tokens).
type Issuer interface {
	IssueGuest(userID uuid.UUID, expires time.Time) (string, error)
}

// Link is a guest invitation to one room.
type Link struct {
	ID        uuid.UUID
	Code      string
	MaxGuests int
	UsedCount int
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Request is a guest waiting for, or decided by, a host.
type Request struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Status    string
	CreatedAt time.Time
}

// Preview is what an anonymous visitor may see about a link.
type Preview struct {
	OrgName, RoomName string
}

// Status is what a guest sees about their own request, with a fresh token.
type Status struct {
	Status, Name, RoomName, MapKey, OrgName string
	RoomID                                  uuid.UUID
	Token                                   string
}

// Service implements guest links, requests, and admission.
type Service struct {
	pool   *pgxpool.Pool
	q      *query.Queries
	auth   Authorizer
	tokens Issuer
	ender  Ender
	now    func() time.Time
	log    *slog.Logger
}

// NewService wires the service. ender may be set later with SetEnder.
func NewService(pool *pgxpool.Pool, a Authorizer, tokens Issuer, now func() time.Time, log *slog.Logger) *Service {
	return &Service{pool: pool, q: query.New(pool), auth: a, tokens: tokens, now: now, log: log}
}

// SetEnder wires the realtime hub before serving requests.
func (s *Service) SetEnder(e Ender) { s.ender = e }

// host loads a guest-capable room and checks the caller is its teacher/admin.
// Other organizations' and missing rooms look the same.
func (s *Service) host(ctx context.Context, userID, roomID uuid.UUID) (query.GetGuestRoomRow, error) {
	r, err := s.q.GetGuestRoom(ctx, roomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, fmt.Errorf("load room: %w", err)
	}
	if _, err := s.auth.Authorize(ctx, userID, r.OrgID, org.RoleTeacher); err != nil {
		if errors.Is(err, org.ErrNotMember) {
			return r, ErrNotFound
		}
		return r, err
	}
	return r, nil
}

// CreateLink makes a guest link for a room; hours 1–24 (default 2), maxGuests 1–40 (default 40).
func (s *Service) CreateLink(ctx context.Context, userID, roomID uuid.UUID, hours, maxGuests *int) (Link, error) {
	v := map[string]string{}
	h, n := defaultHours, MaxGuests
	if hours != nil {
		h = *hours
		if h < 1 || h > MaxHours {
			v["expires_in_hours"] = "Masa berlaku 1–24 jam"
		}
	}
	if maxGuests != nil {
		n = *maxGuests
		if n < 1 || n > MaxGuests {
			v["max_guests"] = "Batas tamu 1–40"
		}
	}
	if len(v) > 0 {
		return Link{}, &ValidationError{Fields: v}
	}
	room, err := s.host(ctx, userID, roomID)
	if err != nil {
		return Link{}, err
	}
	now := s.now().UTC()
	for range 5 {
		code, err := invite.NewCode()
		if err != nil {
			return Link{}, err
		}
		l, err := s.q.CreateGuestLink(ctx, query.CreateGuestLinkParams{
			ID: uuid.New(), OrgID: room.OrgID, RoomID: roomID, Code: code, MaxGuests: int32(n), //nolint:gosec // bounded above
			ExpiresAt: ts(now.Add(time.Duration(h) * time.Hour)), CreatedBy: userID, CreatedAt: ts(now),
		})
		if isUnique(err) {
			continue
		}
		if err != nil {
			return Link{}, fmt.Errorf("create guest link: %w", err)
		}
		s.log.InfoContext(ctx, "guest link created", slog.String("room_id", roomID.String()), slog.String("link_id", l.ID.String()))
		return toLink(l), nil
	}
	return Link{}, errors.New("create guest link: code collisions")
}

// Links lists active links of a room.
func (s *Service) Links(ctx context.Context, userID, roomID uuid.UUID) ([]Link, error) {
	if _, err := s.host(ctx, userID, roomID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListGuestLinks(ctx, query.ListGuestLinksParams{RoomID: roomID, Now: ts(s.now())})
	if err != nil {
		return nil, fmt.Errorf("list guest links: %w", err)
	}
	out := make([]Link, 0, len(rows))
	for _, r := range rows {
		out = append(out, toLink(r))
	}
	return out, nil
}

// RevokeLink ends a link and disconnects its approved guests.
func (s *Service) RevokeLink(ctx context.Context, userID, roomID, linkID uuid.UUID) error {
	if _, err := s.host(ctx, userID, roomID); err != nil {
		return err
	}
	var guests []uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if _, err := q.LockGuestLink(ctx, query.LockGuestLinkParams{RoomID: roomID, LinkID: linkID}); err != nil {
			return err
		}
		var err error
		guests, err = q.RevokeGuestLink(ctx, query.RevokeGuestLinkParams{Now: ts(s.now()), LinkID: linkID, RoomID: roomID})
		if err != nil {
			return err
		}
		return q.RecordGuestDecision(ctx, query.RecordGuestDecisionParams{RoomID: roomID, LinkID: linkID, ActorID: userID, Action: "revoke", CreatedAt: ts(s.now())})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("revoke guest link: %w", err)
	}
	for _, id := range guests {
		if s.ender != nil {
			s.ender.EndGuest(id)
		}
	}
	return nil
}

// Preview shows organization and room names of a usable link.
func (s *Service) Preview(ctx context.Context, rawCode string) (Preview, error) {
	code, ok := invite.NormalizeCode(rawCode)
	if !ok {
		return Preview{}, ErrNotFound
	}
	r, err := s.q.PreviewGuestLink(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return Preview{}, ErrNotFound
	}
	if err != nil {
		return Preview{}, fmt.Errorf("preview guest link: %w", err)
	}
	if r.RevokedAt.Valid || !r.ExpiresAt.Time.After(s.now()) || r.UsedCount >= r.MaxGuests {
		return Preview{}, ErrExpired
	}
	return Preview{OrgName: r.OrgName, RoomName: r.RoomName}, nil
}

// Request creates a guest identity and a pending request.
func (s *Service) Request(ctx context.Context, rawCode, rawName string, requestKey uuid.UUID) (Status, error) {
	if requestKey.Version() != 4 || requestKey.Variant() != uuid.RFC4122 {
		return Status{}, &ValidationError{Fields: map[string]string{"request_key": "Kunci permintaan tidak valid"}}
	}
	key := sha256.Sum256(requestKey[:])
	name, msg := auth.NormalizeDisplayName(rawName)
	if msg != "" {
		return Status{}, &ValidationError{Fields: map[string]string{"display_name": msg}}
	}
	code, ok := invite.NormalizeCode(rawCode)
	if !ok {
		return Status{}, ErrNotFound
	}
	var userID uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.LockGuestRequestKey(ctx, key[:]); err != nil {
			return err
		}
		existing, err := q.FindGuestRequest(ctx, query.FindGuestRequestParams{RequestKey: key[:], Code: code})
		if err == nil {
			userID = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		l, err := q.UseGuestLink(ctx, query.UseGuestLinkParams{Code: code, Now: ts(s.now())})
		if errors.Is(err, pgx.ErrNoRows) {
			if _, perr := q.PreviewGuestLink(ctx, code); errors.Is(perr, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return ErrExpired
		}
		if err != nil {
			return fmt.Errorf("use guest link: %w", err)
		}
		userID, err = q.CreateGuestUser(ctx, name)
		if err != nil {
			return fmt.Errorf("create guest: %w", err)
		}
		return q.CreateGuestRequest(ctx, query.CreateGuestRequestParams{ID: uuid.New(), LinkID: l.ID, UserID: userID, RequestKey: key[:], CreatedAt: ts(s.now())})
	})
	if isUnique(err) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, err
	}
	return s.Status(ctx, userID)
}

// Status returns the guest's own request state.
func (s *Service) Status(ctx context.Context, guestID uuid.UUID) (Status, error) {
	r, err := s.q.GuestStatus(ctx, query.GuestStatusParams{UserID: guestID, Now: ts(s.now())})
	if errors.Is(err, pgx.ErrNoRows) {
		return Status{}, ErrNotFound
	}
	if err != nil {
		return Status{}, fmt.Errorf("guest status: %w", err)
	}
	if r.Banned {
		return Status{}, ErrKicked
	}
	if !r.Active {
		return Status{}, ErrExpired
	}
	expires := s.now().Add(tokenTTL)
	if r.ExpiresAt.Time.Before(expires) {
		expires = r.ExpiresAt.Time
	}
	tok, err := s.tokens.IssueGuest(guestID, expires)
	if err != nil {
		return Status{}, err
	}
	return Status{Status: string(r.Status), Name: r.DisplayName, RoomID: r.RoomID, RoomName: r.RoomName, MapKey: r.MapKey,
		OrgName: r.OrgName, Token: tok}, nil
}

// Requests lists pending guests of a room for its host.
func (s *Service) Requests(ctx context.Context, userID, roomID uuid.UUID) ([]Request, error) {
	if _, err := s.host(ctx, userID, roomID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListGuestRequests(ctx, query.ListGuestRequestsParams{RoomID: roomID, Now: ts(s.now())})
	if err != nil {
		return nil, fmt.Errorf("list guest requests: %w", err)
	}
	out := make([]Request, 0, len(rows))
	for _, r := range rows {
		out = append(out, Request{ID: r.ID, UserID: r.UserID, Name: r.DisplayName, Status: "pending", CreatedAt: r.CreatedAt.Time})
	}
	return out, nil
}

// Decide approves or denies a pending request. Deciding twice is 404.
func (s *Service) Decide(ctx context.Context, userID, roomID, requestID uuid.UUID, approve bool) (Request, error) {
	if _, err := s.host(ctx, userID, roomID); err != nil {
		return Request{}, err
	}
	status := query.GuestStatusDenied
	if approve {
		status = query.GuestStatusApproved
	}
	var r query.DecideGuestRequestRow
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		linkID, err := q.LockGuestRequestLink(ctx, query.LockGuestRequestLinkParams{RoomID: roomID, RequestID: requestID})
		if err != nil {
			return err
		}
		r, err = q.DecideGuestRequest(ctx, query.DecideGuestRequestParams{
			Status: status, DecidedBy: uuid.NullUUID{UUID: userID, Valid: true}, Now: ts(s.now()), ID: requestID, RoomID: roomID,
		})
		if err != nil {
			return err
		}
		action := "deny"
		if approve {
			action = "approve"
		}
		return q.RecordGuestDecision(ctx, query.RecordGuestDecisionParams{RoomID: roomID, LinkID: linkID, ActorID: userID, TargetID: uuid.NullUUID{UUID: r.UserID, Valid: true}, Action: action, CreatedAt: ts(s.now())})
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, ErrNotFound
	}
	if err != nil {
		return Request{}, fmt.Errorf("decide guest request: %w", err)
	}
	s.log.InfoContext(ctx, "guest decided", slog.String("room_id", roomID.String()), slog.String("status", string(status)))
	return Request{ID: r.ID, UserID: r.UserID, Name: r.DisplayName, Status: string(r.Status), CreatedAt: r.CreatedAt.Time}, nil
}

func toLink(l query.GuestLink) Link {
	return Link{ID: l.ID, Code: invite.DisplayCode(l.Code), MaxGuests: int(l.MaxGuests), UsedCount: int(l.UsedCount),
		ExpiresAt: l.ExpiresAt.Time, CreatedAt: l.CreatedAt.Time}
}

func ts(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t.UTC(), Valid: true} }

func isUnique(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
