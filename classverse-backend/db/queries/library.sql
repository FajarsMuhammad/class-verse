-- name: CreateBook :one
INSERT INTO library_books (id, org_id, title, author, shelf, level, format, copies, file_id, cover, sample,
  created_by, created_at, updated_at, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13, $14)
ON CONFLICT (id) DO NOTHING
RETURNING *;

-- name: GetBook :one
SELECT * FROM library_books WHERE org_id = $1 AND id = $2 AND archived_at IS NULL;

-- name: LockBook :one
SELECT * FROM library_books WHERE org_id = $1 AND id = $2 AND archived_at IS NULL FOR UPDATE;

-- name: UpdateBook :one
UPDATE library_books SET title = $3, author = $4, shelf = $5, level = $6, format = $7, copies = $8,
  file_id = $9, cover = $10, published_at = $11, updated_at = $12
WHERE org_id = $1 AND id = $2 AND archived_at IS NULL
RETURNING *;

-- name: ArchiveBook :execrows
UPDATE library_books SET archived_at = $3, updated_at = $3
WHERE org_id = $1 AND id = $2 AND archived_at IS NULL;

-- name: ListBooks :many
-- Catalog page ordered by title, with the caller's reader position. Staff may include drafts.
SELECT sqlc.embed(b), COALESCE(p.position, 0)::integer AS position,
  (SELECT count(*) FROM library_loans l WHERE l.book_id = b.id AND l.returned_at IS NULL)::integer AS on_loan,
  m.id AS my_loan_id, m.due_at AS my_due_at
FROM library_books b
LEFT JOIN reader_positions p ON p.file_id = b.file_id AND p.user_id = sqlc.arg(user_id)
LEFT JOIN library_loans m ON m.book_id = b.id AND m.user_id = sqlc.arg(user_id) AND m.returned_at IS NULL
WHERE b.org_id = sqlc.arg(org_id)
  AND b.archived_at IS NULL
  AND (sqlc.arg(include_drafts)::boolean OR b.published_at IS NOT NULL)
  AND (sqlc.narg(shelf)::text IS NULL OR b.shelf = sqlc.narg(shelf))
  AND (sqlc.narg(pattern)::text IS NULL OR b.title ILIKE sqlc.narg(pattern) OR b.author ILIKE sqlc.narg(pattern))
  AND (sqlc.narg(cursor_title)::text IS NULL
       OR (b.title, b.id) > (sqlc.narg(cursor_title)::text, sqlc.narg(cursor_id)::uuid))
ORDER BY b.title, b.id
LIMIT sqlc.arg(row_limit);

-- name: OrgAdmin :one
SELECT user_id FROM memberships WHERE org_id = $1 AND role = 'admin' ORDER BY joined_at, user_id LIMIT 1;

-- name: FilePublished :one
-- A participant may read a file only through a published, non-archived book or module step.
SELECT (EXISTS (SELECT 1 FROM library_books b
    WHERE b.org_id = sqlc.arg(org_id) AND b.file_id = sqlc.arg(file_id) AND b.archived_at IS NULL AND b.published_at IS NOT NULL)
)::boolean;

-- name: FileInUse :one
-- Books and module steps that are not archived keep their file from being deleted.
SELECT (EXISTS (SELECT 1 FROM library_books b
    WHERE b.org_id = sqlc.arg(org_id) AND b.file_id = sqlc.arg(file_id) AND b.archived_at IS NULL)
  OR EXISTS (SELECT 1 FROM module_steps s JOIN modules m ON m.org_id = s.org_id AND m.id = s.module_id
    WHERE s.org_id = sqlc.arg(org_id) AND s.file_id = sqlc.arg(file_id) AND s.archived_at IS NULL AND m.archived_at IS NULL)
  OR EXISTS (SELECT 1 FROM module_revision_steps s WHERE s.org_id=sqlc.arg(org_id) AND s.file_id=sqlc.arg(file_id))
  OR EXISTS (SELECT 1 FROM assignment_steps s WHERE s.org_id=sqlc.arg(org_id) AND s.file_id=sqlc.arg(file_id)))::boolean;

-- name: LockMembership :one
-- Serializes one member's borrows so the per-member limit holds under concurrency.
SELECT role FROM memberships WHERE org_id = $1 AND user_id = $2 FOR UPDATE;

-- name: ActiveLoan :one
SELECT * FROM library_loans WHERE book_id = $1 AND user_id = $2 AND returned_at IS NULL;

-- name: CountMemberLoans :one
SELECT count(*)::integer FROM library_loans WHERE org_id = $1 AND user_id = $2 AND returned_at IS NULL;

-- name: CountBookLoans :one
SELECT count(*)::integer FROM library_loans WHERE book_id = $1 AND returned_at IS NULL;

-- name: CreateLoan :one
INSERT INTO library_loans (id, org_id, book_id, user_id, borrowed_at, due_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ReturnLoan :one
-- Idempotent: a retried return keeps the first return time.
UPDATE library_loans SET returned_at = COALESCE(returned_at, sqlc.arg(now))
WHERE org_id = sqlc.arg(org_id) AND id = sqlc.arg(id) AND user_id = sqlc.arg(user_id)
RETURNING *;

-- name: ListMemberLoans :many
-- "Pinjamanku": active loans, soonest due first. Archived books stay listed so they can be returned.
SELECT sqlc.embed(l), b.title, b.author, b.cover
FROM library_loans l JOIN library_books b ON b.org_id = l.org_id AND b.id = l.book_id
WHERE l.org_id = $1 AND l.user_id = $2 AND l.returned_at IS NULL
ORDER BY l.due_at, l.id;
