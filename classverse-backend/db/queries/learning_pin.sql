-- name: SaveLearningPin :execrows
UPDATE room_sessions rs SET pin_module_id=sqlc.narg(module_id)::uuid,pin_step_id=sqlc.narg(step_id)::uuid
FROM rooms r JOIN memberships actor ON actor.org_id=r.org_id AND actor.user_id=sqlc.arg(actor_id)
WHERE rs.room_id=r.id AND r.id=sqlc.arg(room_id) AND rs.ended_at IS NULL
 AND r.map_key<>'campus' AND r.archived_at IS NULL AND actor.role IN('teacher','admin')
 AND (sqlc.narg(module_id)::uuid IS NOT NULL OR sqlc.narg(step_id)::uuid IS NULL)
 AND (sqlc.narg(module_id)::uuid IS NULL OR EXISTS(SELECT 1 FROM modules m WHERE m.org_id=r.org_id AND m.id=sqlc.narg(module_id)::uuid AND m.published_at IS NOT NULL AND m.archived_at IS NULL
  AND (sqlc.narg(step_id)::uuid IS NULL OR EXISTS(SELECT 1 FROM module_steps s WHERE s.org_id=m.org_id AND s.module_id=m.id AND s.id=sqlc.narg(step_id)::uuid AND s.archived_at IS NULL))));

-- name: GetLearningPin :one
SELECT rs.pin_module_id,rs.pin_step_id,m.title,m.building_key,m.level
FROM room_sessions rs JOIN rooms r ON r.id=rs.room_id
JOIN modules m ON m.org_id=r.org_id AND m.id=rs.pin_module_id AND m.published_at IS NOT NULL AND m.archived_at IS NULL
WHERE rs.room_id=$1 AND rs.ended_at IS NULL
 AND (rs.pin_step_id IS NULL OR EXISTS(SELECT 1 FROM module_steps s WHERE s.org_id=m.org_id AND s.module_id=m.id AND s.id=rs.pin_step_id AND s.archived_at IS NULL));
