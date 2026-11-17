package assignment_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/assignment"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestSnapshotFileRetentionAndOrderedAccess(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	id := uuid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO file_storage(org_id) VALUES($1)`, f.org); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO files(id,org_id,uploaded_by,name,kind,size_bytes,status,object_key,created_at,updated_at,sha256,content_type) VALUES($1,$2,$3,'File','pdf',10,'ready',$4,now(),now(),decode(repeat('00',32),'hex'),'application/pdf')`, id, f.org, f.admin, id.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE module_steps SET kind='berkas',body='',file_id=$1 WHERE id=$2`, id, f.second); err != nil {
		t.Fatal(err)
	}
	a, err := f.svc.Issue(ctx, f.admin, f.org, assignment.Input{ID: uuid.New(), ClassID: f.class, ModuleID: f.module})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE modules SET archived_at=now()`); err != nil {
		t.Fatal(err)
	}
	q := query.New(f.pool)
	inUse, err := q.FileInUse(ctx, query.FileInUseParams{OrgID: f.org, FileID: uuid.NullUUID{UUID: id, Valid: true}})
	if err != nil || !inUse {
		t.Fatalf("snapshot file not retained: %v %v", inUse, err)
	}
	args := query.AssignmentFileReadableParams{OrgID: f.org, UserID: f.student, FileID: uuid.NullUUID{UUID: id, Valid: true}}
	canRead, err := q.AssignmentFileReadable(ctx, args)
	if err != nil || canRead {
		t.Fatalf("locked snapshot file=%v %v", canRead, err)
	}
	steps, _ := f.svc.Steps(ctx, f.student, f.org, a.ID)
	if err := f.svc.Complete(ctx, f.student, f.org, a.ID, steps[0].ID); err != nil {
		t.Fatal(err)
	}
	canRead, err = q.AssignmentFileReadable(ctx, args)
	if err != nil || !canRead {
		t.Fatalf("unlocked snapshot file=%v %v", canRead, err)
	}
	if err := f.svc.Cancel(ctx, f.admin, f.org, a.ID); err != nil {
		t.Fatal(err)
	}
	canRead, err = q.AssignmentFileReadable(ctx, args)
	if err != nil || canRead {
		t.Fatalf("cancelled snapshot file=%v %v", canRead, err)
	}
}
