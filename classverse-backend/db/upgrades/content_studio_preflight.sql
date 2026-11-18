-- Canonical upgrade preflight after 00020 and before 00021. Also safe to retry after 00021.
-- No publication pointers, draft versions, scores or existing snapshots change.
BEGIN;
SELECT pg_advisory_xact_lock(hashtextextended('content_studio_upgrade',0));
DO $$ BEGIN
 IF (SELECT max(version_id) FROM goose_db_version WHERE is_applied) < 20 THEN
  RAISE EXCEPTION 'content studio preflight requires migration 00020';
 END IF;
END $$;
INSERT INTO quiz_revisions
SELECT gen_random_uuid(),z.org_id,z.id,1,z.draft_version,z.building_key,z.level,z.title,z.description,z.pass_score,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'prompt',q.prompt,'options',q.options,'answer',q.answer,'explanation',q.explanation) ORDER BY q.position,q.id) FROM quiz_questions q WHERE q.quiz_id=z.id),'[]'::jsonb),z.updated_at
FROM quizzes z WHERE NOT EXISTS(SELECT 1 FROM quiz_revisions r WHERE r.quiz_id=z.id)
 AND EXISTS(SELECT 1 FROM module_steps reference
 WHERE reference.org_id=z.org_id AND reference.quiz_id=z.id
 AND EXISTS(SELECT 1 FROM module_steps activity JOIN module_progress p ON p.org_id=activity.org_id AND p.step_id=activity.id
 WHERE activity.org_id=reference.org_id AND activity.module_id=reference.module_id))
ON CONFLICT DO NOTHING;
COMMIT;
