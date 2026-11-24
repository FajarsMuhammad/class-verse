-- +goose Up
-- Forward repair for local 00020 rollout; no downgrade or modification of existing snapshots.
-- Preflight composite ownership before adding constraint; fail rather than delete mismatched data.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM module_revision_steps r JOIN module_steps s ON s.id=r.id
 WHERE s.org_id<>r.org_id OR s.module_id<>r.module_id) THEN
  RAISE EXCEPTION 'revision step ownership mismatch; investigate before migration';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE module_steps ADD CONSTRAINT module_steps_revision_parent_key UNIQUE(org_id,module_id,id);
ALTER TABLE module_revision_steps ADD CONSTRAINT module_revision_steps_parent_fk
 FOREIGN KEY(org_id,module_id,id) REFERENCES module_steps(org_id,module_id,id);

-- Working/issued legacy documents gain explicit schema version. Published snapshots stay immutable;
-- absent schema_version means legacy schema 1 in validator/renderer.
UPDATE module_steps SET rich_content=jsonb_set(rich_content,'{attrs,schema_version}','1',true)
WHERE rich_content IS NOT NULL AND rich_content->'attrs' IS NOT NULL AND NOT(rich_content->'attrs' ? 'schema_version');
UPDATE module_steps SET rich_content=rich_content || '{"attrs":{"schema_version":1}}'::jsonb
WHERE rich_content IS NOT NULL AND rich_content->'attrs' IS NULL;
UPDATE assignment_steps SET rich_content=rich_content || '{"attrs":{"schema_version":1}}'::jsonb
WHERE rich_content IS NOT NULL AND rich_content->'attrs' IS NULL;

-- Preserve otherwise orphaned legacy task/attempt quiz context, without publishing shared drafts.
INSERT INTO quiz_revisions
SELECT gen_random_uuid(),z.org_id,z.id,1,z.draft_version,z.building_key,z.level,z.title,z.description,z.pass_score,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'prompt',q.prompt,'options',q.options,'answer',q.answer,'explanation',q.explanation) ORDER BY q.position,q.id) FROM quiz_questions q WHERE q.quiz_id=z.id),'[]'::jsonb),z.updated_at
FROM quizzes z WHERE NOT EXISTS(SELECT 1 FROM quiz_revisions r WHERE r.quiz_id=z.id)
 AND (EXISTS(SELECT 1 FROM assignment_steps s WHERE s.quiz_id=z.id)
 OR EXISTS(SELECT 1 FROM quiz_attempts a WHERE a.quiz_id=z.id)
 OR EXISTS(SELECT 1 FROM module_steps s JOIN module_progress p ON p.step_id=s.id WHERE s.quiz_id=z.id));
UPDATE assignment_steps s SET quiz_revision_id=r.id FROM quiz_revisions r
WHERE r.quiz_id=s.quiz_id AND r.revision=1 AND s.quiz_revision_id IS NULL;
UPDATE quiz_attempts a SET quiz_revision_id=r.id FROM quiz_revisions r
WHERE r.quiz_id=a.quiz_id AND r.revision=1 AND a.quiz_revision_id IS NULL;

WITH repaired AS (
INSERT INTO module_revisions
SELECT gen_random_uuid(),m.org_id,m.id,1,m.draft_version,m.building_key,m.level,m.title,m.description,m.updated_at
FROM modules m WHERE NOT EXISTS(SELECT 1 FROM module_revisions r WHERE r.module_id=m.id)
 AND EXISTS(SELECT 1 FROM module_steps s JOIN module_progress p ON p.step_id=s.id WHERE s.module_id=m.id) RETURNING *
)
INSERT INTO module_revision_steps
SELECT s.org_id,s.module_id,r.id,s.id,s.position,s.kind,s.title,s.body,s.rich_content,s.file_id,s.video_id,s.quiz_id,q.id,s.minutes
FROM module_steps s JOIN repaired r ON r.module_id=s.module_id
LEFT JOIN quiz_revisions q ON q.quiz_id=s.quiz_id AND q.revision=1
WHERE s.archived_at IS NULL AND NOT EXISTS(SELECT 1 FROM module_revision_steps x WHERE x.revision_id=r.id);
INSERT INTO module_revision_pins
SELECT DISTINCT s.org_id,s.module_id,p.user_id,r.id FROM module_progress p JOIN module_steps s ON s.id=p.step_id
JOIN module_revisions r ON r.module_id=s.module_id AND r.revision=1
ON CONFLICT DO NOTHING;

-- +goose Down
-- Keep repaired context and composite ownership. Removing it cannot restore pre-CS academic semantics.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM module_revisions) OR EXISTS(SELECT 1 FROM quiz_revisions) OR EXISTS(SELECT 1 FROM quiz_questions WHERE cardinality(options)=2) THEN
 RAISE EXCEPTION 'content studio hardening is forward-only with data; restore reviewed backup for rollback';
 END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE module_revision_steps DROP CONSTRAINT module_revision_steps_parent_fk;
ALTER TABLE module_steps DROP CONSTRAINT module_steps_revision_parent_key;
