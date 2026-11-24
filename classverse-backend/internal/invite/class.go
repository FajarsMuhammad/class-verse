package invite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

// inviteClass locks an active class and checks current authority: admin or its active wali kelas.
// Teacher changes/archive also lock this class, so creation and acceptance cannot race those changes.
func inviteClass(ctx context.Context, q *query.Queries, orgID, classID, creator uuid.UUID) (query.Class, error) {
	c, err := q.LockClass(ctx, query.LockClassParams{OrgID: orgID, ID: classID})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("lock invite class: %w", err)
	}
	role, err := q.GetMemberRole(ctx, query.GetMemberRoleParams{OrgID: orgID, UserID: creator})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrForbidden
	}
	if err != nil {
		return c, fmt.Errorf("invite creator role: %w", err)
	}
	if role == query.OrgRoleAdmin {
		return c, nil
	}
	if role != query.OrgRoleTeacher {
		return c, ErrForbidden
	}
	cr, err := q.ClassTeacherRole(ctx, query.ClassTeacherRoleParams{ClassID: classID, UserID: creator})
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrForbidden
	}
	if err != nil {
		return c, fmt.Errorf("invite homeroom role: %w", err)
	}
	if cr != query.ClassTeacherRoleHomeroom {
		return c, ErrForbidden
	}
	return c, nil
}

// Caller holds roster and class locks; never ends an existing placement or changes a membership role.
func placeInviteStudent(ctx context.Context, q *query.Queries, m query.Membership, c query.Class, creator uuid.UUID, now time.Time) (string, bool, error) {
	role, err := q.GetMemberRoleForUpdate(ctx, query.GetMemberRoleForUpdateParams{OrgID: m.OrgID, UserID: m.UserID})
	if err != nil {
		return "", false, fmt.Errorf("lock invite student: %w", err)
	}
	if role != query.OrgRoleParticipant {
		return "not_student", false, nil
	}
	current, err := q.ActiveEnrollment(ctx, query.ActiveEnrollmentParams{OrgID: m.OrgID, AcademicYearID: c.AcademicYearID, UserID: m.UserID})
	if err == nil {
		if current.ClassID == c.ID {
			return "placed", false, nil
		}
		return "has_class", false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", false, fmt.Errorf("invite active placement: %w", err)
	}
	_, err = q.CreateEnrollment(ctx, query.CreateEnrollmentParams{ID: uuid.New(), OrgID: m.OrgID, AcademicYearID: c.AcademicYearID, ClassID: c.ID, UserID: m.UserID, StartedAt: pgTime(now), CreatedBy: creator})
	if err != nil {
		return "", false, fmt.Errorf("invite placement: %w", err)
	}
	return "placed", true, nil
}
