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
SELECT sqlc.embed(b), COALESCE(p.position, 0)::integer AS position
FROM library_books b
LEFT JOIN reader_positions p ON p.file_id = b.file_id AND p.user_id = sqlc.arg(user_id)
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
-- A participant may read a file only through a published, non-archived book.
SELECT EXISTS (SELECT 1 FROM library_books
  WHERE org_id = $1 AND file_id = $2 AND archived_at IS NULL AND published_at IS NOT NULL);

-- name: FileInUse :one
SELECT EXISTS (SELECT 1 FROM library_books WHERE org_id = $1 AND file_id = $2 AND archived_at IS NULL);
