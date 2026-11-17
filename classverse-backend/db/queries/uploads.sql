-- name: CreateFileUpload :exec
INSERT INTO file_uploads(org_id,file_id,expected_sha256,multipart_id,expires_at)
VALUES ($1,$2,$3,$4,$5);

-- name: GetOwnedFileUpload :one
SELECT f.id,f.org_id,f.uploaded_by,f.kind,f.size_bytes,f.object_key,f.status,
 u.expected_sha256,u.multipart_id,u.state,u.expires_at
FROM files f JOIN file_uploads u ON u.org_id=f.org_id AND u.file_id=f.id
JOIN memberships m ON m.org_id=f.org_id AND m.user_id=f.uploaded_by
WHERE f.org_id=$1 AND f.id=$2 AND f.uploaded_by=$3
AND m.role IN ('teacher','admin');

-- name: MarkUploadAssembled :exec
UPDATE file_uploads SET state='assembled' WHERE org_id=$1 AND file_id=$2 AND state='uploading';

-- name: ListExpiredUploads :many
SELECT f.org_id,f.id,f.object_key,u.multipart_id FROM file_uploads u
JOIN files f ON f.org_id=u.org_id AND f.id=u.file_id
WHERE u.expires_at < $1 AND f.status IN ('pending','deleting')
ORDER BY u.expires_at LIMIT 100;

-- name: MarkUploadCanceled :exec
UPDATE file_uploads SET state='canceled' WHERE org_id=$1 AND file_id=$2;
