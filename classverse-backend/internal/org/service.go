package org

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

const (
	minNameLen = 3
	maxNameLen = 80
	maxURLLen  = 500
)

var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,46}[a-z0-9])$`)

// Domain errors.
var (
	ErrNotMember     = errors.New("not a member")
	ErrForbidden     = errors.New("forbidden")
	ErrSlugTaken     = errors.New("slug taken")
	ErrNotAdult      = errors.New("must be adult to create organization")
	ErrNotEligible   = errors.New("email or profile incomplete")
	ErrUserNotFound  = errors.New("user not found")
	errInvalidFields = errors.New("invalid fields")
)

// ValidationError lists invalid input fields with user-facing messages.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return errInvalidFields.Error() }

// Organization is an organization as seen by one member.
type Organization struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	LogoURL      *string
	Role         Role
	CampusRoomID uuid.UUID
	CreatedAt    time.Time
}

// Summary is one entry of the user's organization list.
type Summary struct {
	ID   uuid.UUID
	Name string
	Slug string
	Role Role
}

// CreateInput is the body of POST /api/orgs.
type CreateInput struct {
	Name    string
	Slug    string
	LogoURL *string
}

// Service implements organization rules.
type Service struct {
	store *Store
	now   func() time.Time
	log   *slog.Logger
}

// NewService wires the organization service.
func NewService(store *Store, now func() time.Time, log *slog.Logger) *Service {
	return &Service{store: store, now: now, log: log}
}

// Create makes a new organization with the caller as admin and its campus
// map room, all in one transaction.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Organization, error) {
	name := strings.TrimSpace(in.Name)
	slug := strings.TrimSpace(in.Slug)
	fields := map[string]string{}
	if n := utf8.RuneCountInString(name); n < minNameLen || n > maxNameLen {
		fields["name"] = "Nama sekolah 3–80 karakter"
	}
	if !slugRe.MatchString(slug) {
		fields["slug"] = "Slug 3–48 karakter: huruf kecil, angka, dan tanda hubung"
	}
	var logo pgtype.Text
	if in.LogoURL != nil && strings.TrimSpace(*in.LogoURL) != "" {
		u := strings.TrimSpace(*in.LogoURL)
		if !validHTTPS(u) {
			fields["logo_url"] = "Logo harus berupa URL https"
		}
		logo = pgtype.Text{String: u, Valid: true}
	}
	if len(fields) > 0 {
		return Organization{}, &ValidationError{Fields: fields}
	}

	now := s.now()
	var (
		org    query.Organization
		campus uuid.UUID
	)
	err := s.store.InTx(ctx, func(q *query.Queries) error {
		u, err := q.GetUserEligibility(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}
		if !u.EmailVerifiedAt.Valid || !u.BirthDate.Valid {
			return ErrNotEligible
		}
		if Age(u.BirthDate.Time, now) < AdultAge {
			return ErrNotAdult
		}
		org, err = q.CreateOrg(ctx, query.CreateOrgParams{
			Name: name, Slug: slug, LogoUrl: logo, CreatedBy: userID, CreatedAt: pgTime(now),
		})
		if err != nil {
			return fmt.Errorf("create org: %w", err)
		}
		_, err = q.CreateMembership(ctx, query.CreateMembershipParams{
			OrgID: org.ID, UserID: userID, Role: query.OrgRoleAdmin, JoinedAt: pgTime(now),
		})
		if err != nil {
			return fmt.Errorf("create admin membership: %w", err)
		}
		campus, err = q.CreateCampusRoom(ctx, query.CreateCampusRoomParams{
			OrgID: org.ID, CreatedBy: userID, CreatedAt: pgTime(now),
		})
		if err != nil {
			return fmt.Errorf("create campus room: %w", err)
		}
		return nil
	})
	switch {
	case isUniqueViolation(err, "organizations_slug_key"):
		return Organization{}, ErrSlugTaken
	case errors.Is(err, ErrNotEligible), errors.Is(err, ErrNotAdult), errors.Is(err, ErrUserNotFound):
		return Organization{}, unwrapDomain(err)
	case err != nil:
		return Organization{}, fmt.Errorf("create organization: %w", err)
	}
	s.log.InfoContext(ctx, "org created", slog.String("org_id", org.ID.String()), slog.String("user_id", userID.String()))
	return Organization{
		ID: org.ID, Name: org.Name, Slug: org.Slug, LogoURL: textPtr(org.LogoUrl),
		Role: RoleAdmin, CampusRoomID: campus, CreatedAt: org.CreatedAt.Time,
	}, nil
}

// Get returns the organization if userID is a member.
func (s *Service) Get(ctx context.Context, userID, orgID uuid.UUID) (Organization, error) {
	row, err := s.store.GetOrgForMember(ctx, query.GetOrgForMemberParams{OrgID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotMember
	}
	if err != nil {
		return Organization{}, fmt.Errorf("get org: %w", err)
	}
	return Organization{
		ID: row.ID, Name: row.Name, Slug: row.Slug, LogoURL: textPtr(row.LogoUrl),
		Role: Role(row.Role), CampusRoomID: row.CampusRoomID, CreatedAt: row.CreatedAt.Time,
	}, nil
}

// Authorize returns the caller's role if it is at least minRole.
// Non-members get ErrNotMember so the organization's existence is not revealed.
func (s *Service) Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole Role) (Role, error) {
	r, err := s.store.GetMemberRole(ctx, query.GetMemberRoleParams{OrgID: orgID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotMember
	}
	if err != nil {
		return "", fmt.Errorf("get member role: %w", err)
	}
	role := Role(r)
	if !role.AtLeast(minRole) {
		return role, ErrForbidden
	}
	return role, nil
}

// ListForUser returns the organizations the user belongs to.
func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]Summary, error) {
	rows, err := s.store.ListUserOrgs(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user orgs: %w", err)
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, Summary{ID: r.ID, Name: r.Name, Slug: r.Slug, Role: Role(r.Role)})
	}
	return out, nil
}

func validHTTPS(raw string) bool {
	if len(raw) > maxURLLen {
		return false
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

// unwrapDomain returns the sentinel inside err (InTx wraps errors).
func unwrapDomain(err error) error {
	for _, target := range []error{ErrNotEligible, ErrNotAdult, ErrUserNotFound} {
		if errors.Is(err, target) {
			return target
		}
	}
	return err
}

// ListOrgSummaries adapts ListForUser to the API model used by GET /api/me.
func (s *Service) ListOrgSummaries(ctx context.Context, userID uuid.UUID) ([]api.OrgSummary, error) {
	orgs, err := s.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]api.OrgSummary, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, api.OrgSummary{Id: o.ID, Name: o.Name, Slug: o.Slug, Role: api.Role(o.Role)})
	}
	return out, nil
}
