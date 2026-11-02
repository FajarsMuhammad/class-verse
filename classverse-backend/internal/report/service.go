// Package report handles participant reports and the moderation audit log.
package report

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/text/unicode/norm"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

const (
	maxReasonLen = 1000
	defaultLimit = 20
	maxLimit     = 100
)

// Report statuses.
const (
	StatusOpen      = "open"
	StatusReviewed  = "reviewed"
	StatusDismissed = "dismissed"
)

// Domain errors.
var (
	ErrNotFound      = errors.New("report target not found")
	ErrInvalidCursor = errors.New("invalid cursor")
	ErrSelfReport    = errors.New("cannot report yourself")
)

// ValidationError lists invalid input fields.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid report fields" }

// Authorizer checks a user's role in an organization (implemented by org.Service).
type Authorizer interface {
	Authorize(ctx context.Context, userID, orgID uuid.UUID, minRole org.Role) (org.Role, error)
}

// CreateInput is the body of POST /api/reports. With MessageID the target and
// room come from the stored message; otherwise RoomID and TargetID are required.
type CreateInput struct {
	TargetID  uuid.UUID
	MessageID *int64
	RoomID    *uuid.UUID
	Reason    string
}

// Report is one report with display data.
type Report struct {
	ID           uuid.UUID
	OrgID        uuid.UUID
	RoomID       *uuid.UUID
	RoomName     *string
	ReporterID   uuid.UUID
	ReporterName string
	TargetID     uuid.UUID
	TargetName   string
	MessageID    *int64
	MessageText  *string
	Reason       string
	Status       string
	ReviewedBy   *uuid.UUID
	ReviewedAt   *time.Time
	CreatedAt    time.Time
}

// Action is one moderation_actions row for the admin log.
type Action struct {
	ID         uuid.UUID
	RoomID     uuid.UUID
	RoomName   string
	ActorID    uuid.UUID
	ActorName  string
	TargetID   *uuid.UUID
	TargetName string
	Action     string
	CreatedAt  time.Time
}

// Service implements report rules.
type Service struct {
	q    *Store
	auth Authorizer
	now  func() time.Time
}

// NewService wires the report service.
func NewService(q *Store, auth Authorizer, now func() time.Time) *Service {
	return &Service{q: q, auth: auth, now: now}
}

// Create stores a report. Every failed visibility check is ErrNotFound so a
// reporter cannot probe other organizations' rooms, users or messages.
func (s *Service) Create(ctx context.Context, reporterID uuid.UUID, in CreateInput) (Report, error) {
	reason, err := validateReason(in.Reason)
	if err != nil {
		return Report{}, err
	}
	now := pgtype.Timestamptz{Time: s.now().UTC(), Valid: true}
	var id uuid.UUID
	switch {
	case in.MessageID != nil:
		id, err = s.q.CreateMessageReport(ctx, query.CreateMessageReportParams{
			ReporterID: reporterID, MessageID: *in.MessageID, Reason: reason, Now: now,
		})
	case in.RoomID != nil:
		if in.TargetID == reporterID {
			return Report{}, ErrSelfReport
		}
		id, err = s.q.CreateUserReport(ctx, query.CreateUserReportParams{
			ReporterID: reporterID, TargetID: in.TargetID, RoomID: *in.RoomID, Reason: reason, Now: now,
		})
	default:
		return Report{}, &ValidationError{Fields: map[string]string{"room_id": "Pilih ruang tempat kejadian"}}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, fmt.Errorf("create report: %w", err)
	}
	row, err := s.q.GetReport(ctx, id)
	if err != nil {
		return Report{}, fmt.Errorf("load report: %w", err)
	}
	return fromRow(query.ListReportsRow(row)), nil
}

// List returns reports of an organization for teachers and admins.
func (s *Service) List(ctx context.Context, userID, orgID uuid.UUID, status string, limit int, cursor string) ([]Report, string, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	params := query.ListReportsParams{OrgID: orgID, RowLimit: int32(limit + 1)} //nolint:gosec // limit <= maxLimit
	switch status {
	case "":
	case StatusOpen, StatusReviewed, StatusDismissed:
		params.Status = query.NullReportStatus{ReportStatus: query.ReportStatus(status), Valid: true}
	default:
		return nil, "", &ValidationError{Fields: map[string]string{"status": "Status tidak dikenal"}}
	}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		params.CursorCreatedAt = pgtype.Timestamptz{Time: at, Valid: true}
		params.CursorID = uuid.NullUUID{UUID: id, Valid: true}
	}
	rows, err := s.q.ListReports(ctx, params)
	if err != nil {
		return nil, "", fmt.Errorf("list reports: %w", err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeCursor(last.CreatedAt.Time, last.ID)
	}
	out := make([]Report, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, next, nil
}

// Review sets a report to reviewed or dismissed (teachers and admins).
func (s *Service) Review(ctx context.Context, userID, reportID uuid.UUID, status string) (Report, error) {
	if status != StatusReviewed && status != StatusDismissed {
		return Report{}, &ValidationError{Fields: map[string]string{"status": "Pilih ditinjau atau diabaikan"}}
	}
	orgID, err := s.q.GetReportOrg(ctx, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, fmt.Errorf("load report: %w", err)
	}
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleTeacher); err != nil {
		if errors.Is(err, org.ErrNotMember) {
			return Report{}, ErrNotFound // other organizations' reports look missing
		}
		return Report{}, err
	}
	if _, err := s.q.ReviewReport(ctx, query.ReviewReportParams{
		ID: reportID, OrgID: orgID, Status: query.ReportStatus(status),
		ReviewerID: uuid.NullUUID{UUID: userID, Valid: true}, Now: pgtype.Timestamptz{Time: s.now().UTC(), Valid: true},
	}); err != nil {
		return Report{}, fmt.Errorf("review report: %w", err)
	}
	row, err := s.q.GetReport(ctx, reportID)
	if err != nil {
		return Report{}, fmt.Errorf("load report: %w", err)
	}
	return fromRow(query.ListReportsRow(row)), nil
}

// Log returns the moderation audit log of an organization (admins only).
func (s *Service) Log(ctx context.Context, userID, orgID uuid.UUID, limit int, cursor string) ([]Action, string, error) {
	if _, err := s.auth.Authorize(ctx, userID, orgID, org.RoleAdmin); err != nil {
		return nil, "", err
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	params := query.ListModerationActionsParams{OrgID: orgID, RowLimit: int32(limit + 1)} //nolint:gosec // limit <= maxLimit
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		params.CursorCreatedAt = pgtype.Timestamptz{Time: at, Valid: true}
		params.CursorID = uuid.NullUUID{UUID: id, Valid: true}
	}
	rows, err := s.q.ListModerationActions(ctx, params)
	if err != nil {
		return nil, "", fmt.Errorf("list moderation log: %w", err)
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = encodeCursor(last.CreatedAt.Time, last.ID)
	}
	out := make([]Action, 0, len(rows))
	for _, r := range rows {
		a := Action{ID: r.ID, RoomID: r.RoomID, RoomName: r.RoomName, ActorID: r.ActorID, ActorName: r.ActorName,
			Action: r.Action, CreatedAt: r.CreatedAt.Time.UTC(), TargetName: r.TargetName.String}
		if r.TargetID.Valid {
			id := r.TargetID.UUID
			a.TargetID = &id
		}
		out = append(out, a)
	}
	return out, next, nil
}

// validateReason normalizes (NFC, trim) and limits to 1–1000 code points;
// control characters other than newline and tab are rejected.
func validateReason(raw string) (string, error) {
	s := strings.TrimSpace(norm.NFC.String(raw))
	n := utf8.RuneCountInString(s)
	bad := func(msg string) error { return &ValidationError{Fields: map[string]string{"reason": msg}} }
	if n == 0 {
		return "", bad("Tulis alasan laporan")
	}
	if n > maxReasonLen {
		return "", bad("Alasan maksimal 1000 karakter")
	}
	for _, r := range s {
		if r != '\n' && r != '\t' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
			return "", bad("Alasan berisi karakter yang tidak didukung")
		}
	}
	return s, nil
}

func fromRow(r query.ListReportsRow) Report {
	out := Report{
		ID: r.ID, OrgID: r.OrgID, ReporterID: r.ReporterID, ReporterName: r.ReporterName,
		TargetID: r.TargetUserID, TargetName: r.TargetName, Reason: r.Reason, Status: string(r.Status),
		CreatedAt: r.CreatedAt.Time.UTC(),
	}
	if r.RoomID.Valid {
		id := r.RoomID.UUID
		out.RoomID = &id
	}
	if r.RoomName.Valid {
		name := r.RoomName.String
		out.RoomName = &name
	}
	if r.MessageID.Valid {
		id := r.MessageID.Int64
		out.MessageID = &id
	}
	if r.MessageText.Valid {
		text := r.MessageText.String
		out.MessageText = &text
	}
	if r.ReviewedBy.Valid {
		id := r.ReviewedBy.UUID
		out.ReviewedBy = &id
	}
	if r.ReviewedAt.Valid {
		at := r.ReviewedAt.Time.UTC()
		out.ReviewedAt = &at
	}
	return out
}

// Cursor is opaque to clients: base64url("<unix nanos>|<uuid>").
func encodeCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(at.UnixNano(), 10) + "|" + id.String()))
}

func decodeCursor(c string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	nanos, idPart, ok := strings.Cut(string(raw), "|")
	n, err := strconv.ParseInt(nanos, 10, 64)
	id, err2 := uuid.Parse(idPart)
	if !ok || err != nil || err2 != nil {
		return time.Time{}, uuid.Nil, ErrInvalidCursor
	}
	return time.Unix(0, n).UTC(), id, nil
}
