package invite

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

const (
	maxExpiresDays = 7
	maxUses        = 500
	defaultLimit   = 20
	maxLimit       = 100
	codeAttempts   = 5
)

// Domain errors.
var (
	ErrNotFound          = errors.New("invite not found")
	ErrExpired           = errors.New("invite expired, revoked, or used up")
	ErrAlreadyMember     = errors.New("already a member")
	ErrProfileIncomplete = errors.New("profile incomplete")
	ErrForbidden         = errors.New("forbidden")
	ErrInvalidCursor     = errors.New("invalid cursor")
)

// ValidationError lists invalid input fields.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "invalid invite fields" }

// Authorizer checks a user's role in an organization (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// Invite is an invite as shown to admins and teachers.
type Invite struct {
	ClassID   *uuid.UUID
	ID        uuid.UUID
	Code      string // display form ABCD-EFGH
	Role      org.Role
	MaxUses   *int
	UsedCount int
	ExpiresAt time.Time
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

// Preview is what a user sees before joining.
type Preview struct {
	OrgName   string
	Role      org.Role
	ClassName string
	YearLabel string
}

// Membership is the result of joining.
type Membership struct {
	PlacementStatus string // placed, has_class, class_unavailable, not_student; empty for school-only invites
	OrgID           uuid.UUID
	UserID          uuid.UUID
	Role            org.Role
	JoinedAt        time.Time
}

// CreateInput is the body of POST /api/orgs/{org_id}/invites.
type CreateInput struct {
	ClassID       *uuid.UUID
	Role          org.Role
	ExpiresInDays *int
	MaxUses       *int
}

// Page is one page of invites.
type Page struct {
	Items      []Invite
	NextCursor string
}

// Service implements invite rules.
type Service struct {
	store *Store
	auth  Authorizer
	now   func() time.Time
	log   *slog.Logger
}

// NewService wires the invite service.
func NewService(store *Store, auth Authorizer, now func() time.Time, log *slog.Logger) *Service {
	return &Service{store: store, auth: auth, now: now, log: log}
}

// Create makes an invite. Teachers may only invite participants.
func (s *Service) Create(ctx context.Context, userID, orgID uuid.UUID, in CreateInput) (Invite, error) {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher)
	if err != nil {
		return Invite{}, err
	}
	fields := map[string]string{}
	if in.Role != org.RoleTeacher && in.Role != org.RoleParticipant {
		fields["role"] = "Role harus pengajar atau peserta"
	}
	days := maxExpiresDays
	if in.ExpiresInDays != nil {
		days = *in.ExpiresInDays
		if days < 1 || days > maxExpiresDays {
			fields["expires_in_days"] = "Masa berlaku 1–7 hari"
		}
	}
	var uses pgtype.Int4
	if in.MaxUses != nil {
		if *in.MaxUses < 1 || *in.MaxUses > maxUses {
			fields["max_uses"] = "Batas pemakaian 1–500"
		}
		uses = pgtype.Int4{Int32: int32(min(max(*in.MaxUses, 0), maxUses)), Valid: true}
	}
	if in.ClassID != nil && in.Role != org.RoleParticipant {
		fields["role"] = "Undangan kelas hanya untuk peserta"
	}
	if len(fields) > 0 {
		return Invite{}, &ValidationError{Fields: fields}
	}
	if role == org.RoleTeacher && in.Role != org.RoleParticipant {
		return Invite{}, ErrForbidden
	}

	now := s.now()
	for range codeAttempts {
		code, err := newCode()
		if err != nil {
			return Invite{}, err
		}
		var row query.Invite
		err = s.store.InTx(ctx, func(q *query.Queries) error {
			var classID uuid.NullUUID
			if in.ClassID != nil {
				if _, err := inviteClass(ctx, q, orgID, *in.ClassID, userID); err != nil {
					return err
				}
				classID = uuid.NullUUID{UUID: *in.ClassID, Valid: true}
			}
			var err error
			row, err = q.CreateInvite(ctx, query.CreateInviteParams{
				OrgID: orgID, Code: code, Role: query.OrgRole(in.Role), MaxUses: uses, CreatedBy: userID,
				ExpiresAt: pgTime(now.Add(time.Duration(days) * 24 * time.Hour)), CreatedAt: pgTime(now), ClassID: classID,
			})
			return err
		})
		if isUniqueViolation(err) {
			continue // code collision, try another
		}
		if err != nil {
			return Invite{}, fmt.Errorf("create invite: %w", err)
		}
		s.log.InfoContext(ctx, "invite created", slog.String("org_id", orgID.String()),
			slog.String("invite_id", row.ID.String()), slog.String("user_id", userID.String()))
		return toInvite(row), nil
	}
	return Invite{}, errors.New("create invite: could not find a free code")
}

// List returns active invites, newest first.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, limit int, cursor string, classID *uuid.UUID) (Page, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return Page{}, err
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	params := query.ListActiveInvitesParams{OrgID: orgID, Now: pgTime(s.now()), RowLimit: int32(limit + 1)} //nolint:gosec // bounded by maxLimit
	if classID != nil {
		params.ClassID = uuid.NullUUID{UUID: *classID, Valid: true}
	}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return Page{}, ErrInvalidCursor
		}
		params.CursorCreatedAt = pgTime(at)
		params.CursorID = uuid.NullUUID{UUID: id, Valid: true}
	}
	rows, err := s.store.ListActiveInvites(ctx, params)
	if err != nil {
		return Page{}, fmt.Errorf("list invites: %w", err)
	}
	page := Page{Items: make([]Invite, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			last := rows[limit-1]
			page.NextCursor = encodeCursor(last.CreatedAt.Time, last.ID)
			break
		}
		page.Items = append(page.Items, toInvite(r))
	}
	return page, nil
}

// Revoke disables an invite. Admins may revoke any invite; teachers only their own.
func (s *Service) Revoke(ctx context.Context, userID, orgID, inviteID uuid.UUID) error {
	role, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher)
	if err != nil {
		return err
	}
	inv, err := s.store.GetInviteInOrg(ctx, query.GetInviteInOrgParams{ID: inviteID, OrgID: orgID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("get invite: %w", err)
	}
	if role != org.RoleAdmin && inv.CreatedBy != userID {
		return ErrForbidden
	}
	if err := s.store.RevokeInvite(ctx, query.RevokeInviteParams{Now: pgTime(s.now()), ID: inviteID, OrgID: orgID}); err != nil {
		return fmt.Errorf("revoke invite: %w", err)
	}
	s.log.InfoContext(ctx, "invite revoked", slog.String("org_id", orgID.String()),
		slog.String("invite_id", inviteID.String()), slog.String("user_id", userID.String()))
	return nil
}

// Preview shows which organization and role a code grants.
func (s *Service) Preview(ctx context.Context, rawCode string) (Preview, error) {
	code, ok := normalizeCode(rawCode)
	if !ok {
		return Preview{}, ErrNotFound
	}
	row, err := s.store.GetInvitePreview(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return Preview{}, ErrNotFound
	}
	if err != nil {
		return Preview{}, fmt.Errorf("preview invite: %w", err)
	}
	if !usable(row.RevokedAt, row.ExpiresAt, row.MaxUses, row.UsedCount, s.now()) {
		return Preview{}, ErrExpired
	}
	return Preview{OrgName: row.OrgName, Role: org.Role(row.Role), ClassName: row.ClassName.String, YearLabel: row.YearLabel.String}, nil
}

// Accept joins the organization of the code. The invite row is locked so
// concurrent accepts cannot exceed max_uses.
func (s *Service) Accept(ctx context.Context, userID uuid.UUID, rawCode string) (Membership, error) {
	code, ok := normalizeCode(rawCode)
	if !ok {
		return Membership{}, ErrNotFound
	}
	now := s.now()
	var (
		m         query.Membership
		placement string
		outcome   error
	)
	err := s.store.InTx(ctx, func(q *query.Queries) error {
		u, err := q.GetUserEligibility(ctx, userID)
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if !u.EmailVerifiedAt.Valid || !u.BirthDate.Valid {
			outcome = ErrProfileIncomplete
			return nil
		}
		inv, err := q.GetInviteByCodeForUpdate(ctx, code)
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = ErrNotFound
			return nil
		}
		if err != nil {
			return fmt.Errorf("get invite: %w", err)
		}
		if !usable(inv.RevokedAt, inv.ExpiresAt, inv.MaxUses, inv.UsedCount, now) {
			outcome = ErrExpired
			return nil
		}
		// Class placement shares the roster lock and lock order used by class.Service.
		var target *query.Class
		if inv.ClassID.Valid {
			c, err := q.GetClass(ctx, query.GetClassParams{OrgID: inv.OrgID, ID: inv.ClassID.UUID})
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("get invite class: %w", err)
			}
			placement = "class_unavailable"
			if err == nil {
				if err := q.LockAssignmentRoster(ctx, query.LockAssignmentRosterParams{OrgID: inv.OrgID, YearID: c.AcademicYearID}); err != nil {
					return fmt.Errorf("lock invite roster: %w", err)
				}
				c, err := inviteClass(ctx, q, inv.OrgID, c.ID, inv.CreatedBy)
				if err == nil {
					target = &c
				} else if !errors.Is(err, ErrForbidden) && !errors.Is(err, ErrNotFound) {
					return err
				}
			}
		}
		m, err = q.CreateMembership(ctx, query.CreateMembershipParams{
			OrgID: inv.OrgID, UserID: userID, Role: inv.Role, JoinedAt: pgTime(now),
		})
		newMember := err == nil
		if errors.Is(err, pgx.ErrNoRows) {
			if !inv.ClassID.Valid {
				outcome = ErrAlreadyMember
				return nil
			}
			m, err = q.GetInviteMembership(ctx, query.GetInviteMembershipParams{OrgID: inv.OrgID, UserID: userID})
		}
		if err != nil {
			return fmt.Errorf("create/get membership: %w", err)
		}
		placed := false
		if target != nil {
			placement, placed, err = placeInviteStudent(ctx, q, m, *target, inv.CreatedBy, now)
			if err != nil {
				return err
			}
		}
		if newMember || placed {
			if err := q.IncrementInviteUse(ctx, inv.ID); err != nil {
				return fmt.Errorf("increment invite use: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Membership{}, fmt.Errorf("accept invite: %w", err)
	}
	if outcome != nil {
		return Membership{}, outcome
	}
	s.log.InfoContext(ctx, "invite accepted", slog.String("org_id", m.OrgID.String()), slog.String("user_id", userID.String()))
	return Membership{OrgID: m.OrgID, UserID: m.UserID, Role: org.Role(m.Role), JoinedAt: m.JoinedAt.Time, PlacementStatus: placement}, nil
}

func usable(revokedAt, expiresAt pgtype.Timestamptz, maxUses pgtype.Int4, used int32, now time.Time) bool {
	if revokedAt.Valid || !now.Before(expiresAt.Time) {
		return false
	}
	return !maxUses.Valid || used < maxUses.Int32
}

func toInvite(r query.Invite) Invite {
	inv := Invite{
		ID: r.ID, Code: displayCode(r.Code), Role: org.Role(r.Role), UsedCount: int(r.UsedCount),
		ExpiresAt: r.ExpiresAt.Time, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt.Time,
	}
	if r.ClassID.Valid {
		id := r.ClassID.UUID
		inv.ClassID = &id
	}
	if r.MaxUses.Valid {
		n := int(r.MaxUses.Int32)
		inv.MaxUses = &n
	}
	return inv
}

// Cursor is opaque to clients: base64url("<unix nanos>|<uuid>").
func encodeCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString(fmt.Appendf(nil, "%d|%s", at.UnixNano(), id))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	ts, rawID, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	var nanos int64
	if _, err := fmt.Sscan(ts, &nanos); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return time.Unix(0, nanos).UTC(), id, nil
}

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
