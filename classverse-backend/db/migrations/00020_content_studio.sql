-- +goose Up
-- Working rows remain author-owned; published snapshots never follow draft edits.
ALTER TABLE modules ADD COLUMN draft_version bigint NOT NULL DEFAULT 1 CHECK (draft_version > 0);
ALTER TABLE quizzes ADD COLUMN draft_version bigint NOT NULL DEFAULT 1 CHECK (draft_version > 0);
ALTER TABLE quiz_questions DROP CONSTRAINT quiz_questions_options_check;
ALTER TABLE quiz_questions DROP CONSTRAINT quiz_questions_answer_check;
ALTER TABLE quiz_questions ADD CONSTRAINT quiz_questions_options_check CHECK(cardinality(options) IN (2,4) AND array_position(options,NULL) IS NULL);
ALTER TABLE quiz_questions ADD CONSTRAINT quiz_questions_answer_check CHECK(answer>=0 AND answer<cardinality(options));
ALTER TABLE module_steps ADD COLUMN rich_content jsonb;
ALTER TABLE assignment_steps ADD COLUMN rich_content jsonb;

CREATE TABLE quiz_revisions (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL,
 quiz_id uuid NOT NULL,
 revision integer NOT NULL CHECK (revision > 0),
 draft_version bigint NOT NULL,
 building_key text NOT NULL,
 level text NOT NULL,
 title text NOT NULL,
 description text NOT NULL,
 pass_score smallint NOT NULL CHECK (pass_score BETWEEN 1 AND 100),
 questions jsonb NOT NULL CHECK (jsonb_typeof(questions)='array'),
 published_at timestamptz NOT NULL,
 FOREIGN KEY (org_id,quiz_id) REFERENCES quizzes(org_id,id),
 UNIQUE(org_id,quiz_id,id), UNIQUE(quiz_id,revision), UNIQUE(quiz_id,draft_version)
);
ALTER TABLE quizzes ADD COLUMN published_revision_id uuid;
ALTER TABLE quizzes ADD FOREIGN KEY(org_id,id,published_revision_id) REFERENCES quiz_revisions(org_id,quiz_id,id);

CREATE TABLE module_revisions (
 id uuid PRIMARY KEY,
 org_id uuid NOT NULL,
 module_id uuid NOT NULL,
 revision integer NOT NULL CHECK(revision > 0),
 draft_version bigint NOT NULL,
 building_key text NOT NULL,
 level text NOT NULL,
 title text NOT NULL,
 description text NOT NULL,
 published_at timestamptz NOT NULL,
 FOREIGN KEY(org_id,module_id) REFERENCES modules(org_id,id),
 UNIQUE(org_id,module_id,id), UNIQUE(module_id,revision), UNIQUE(module_id,draft_version)
);
ALTER TABLE modules ADD COLUMN published_revision_id uuid;
ALTER TABLE modules ADD FOREIGN KEY(org_id,id,published_revision_id) REFERENCES module_revisions(org_id,module_id,id);

CREATE TABLE module_revision_steps (
 org_id uuid NOT NULL,
 module_id uuid NOT NULL,
 revision_id uuid NOT NULL,
 id uuid NOT NULL REFERENCES module_steps(id),
 position integer NOT NULL,
 kind text NOT NULL,
 title text NOT NULL,
 body text NOT NULL,
 rich_content jsonb,
 file_id uuid,
 video_id text,
 quiz_id uuid,
 quiz_revision_id uuid,
 minutes integer NOT NULL,
 PRIMARY KEY(revision_id,id),
 FOREIGN KEY(org_id,module_id,revision_id) REFERENCES module_revisions(org_id,module_id,id),
 FOREIGN KEY(org_id,file_id) REFERENCES files(org_id,id),
 FOREIGN KEY(org_id,quiz_id,quiz_revision_id) REFERENCES quiz_revisions(org_id,quiz_id,id),
 CHECK ((kind='kuis')=(quiz_revision_id IS NOT NULL))
);
CREATE INDEX module_revision_steps_file_idx ON module_revision_steps(org_id,file_id) WHERE file_id IS NOT NULL;
CREATE TABLE module_revision_pins (
 org_id uuid NOT NULL,
 module_id uuid NOT NULL,
 user_id uuid NOT NULL,
 revision_id uuid NOT NULL,
 PRIMARY KEY(org_id,module_id,user_id),
 FOREIGN KEY(org_id,module_id,revision_id) REFERENCES module_revisions(org_id,module_id,id),
 FOREIGN KEY(org_id,user_id) REFERENCES memberships(org_id,user_id) ON DELETE CASCADE
);

-- Legacy content is literal text, never parsed as HTML. Preserve newlines in text nodes.
UPDATE module_steps SET rich_content=jsonb_build_object('type','doc','content',jsonb_build_array(
 jsonb_build_object('type','paragraph','attrs',jsonb_build_object('id',gen_random_uuid()::text),'content',
 CASE WHEN body='' THEN '[]'::jsonb ELSE jsonb_build_array(jsonb_build_object('type','text','text',body)) END))) WHERE kind='materi';
UPDATE assignment_steps SET rich_content=jsonb_build_object('type','doc','content',jsonb_build_array(
 jsonb_build_object('type','paragraph','attrs',jsonb_build_object('id',gen_random_uuid()::text),'content',
 CASE WHEN body='' THEN '[]'::jsonb ELSE jsonb_build_array(jsonb_build_object('type','text','text',body)) END))) WHERE kind='materi';
INSERT INTO quiz_revisions
SELECT gen_random_uuid(),z.org_id,z.id,1,z.draft_version,z.building_key,z.level,z.title,z.description,z.pass_score,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'prompt',q.prompt,'options',q.options,'answer',q.answer,'explanation',q.explanation) ORDER BY q.position,q.id) FROM quiz_questions q WHERE q.quiz_id=z.id),'[]'::jsonb),z.published_at
FROM quizzes z WHERE z.published_at IS NOT NULL;
UPDATE quizzes z SET published_revision_id=r.id FROM quiz_revisions r WHERE r.quiz_id=z.id;
-- Legacy unpublished quizzes referenced by published modules can occur after quiz unpublish.
-- Preserve their last observable question set as historical binding, without publishing the quiz.
INSERT INTO quiz_revisions
SELECT gen_random_uuid(),z.org_id,z.id,1,z.draft_version,z.building_key,z.level,z.title,z.description,z.pass_score,
 COALESCE((SELECT jsonb_agg(jsonb_build_object('id',q.id,'prompt',q.prompt,'options',q.options,'answer',q.answer,'explanation',q.explanation) ORDER BY q.position,q.id) FROM quiz_questions q WHERE q.quiz_id=z.id),'[]'::jsonb),z.updated_at
FROM quizzes z WHERE NOT EXISTS(SELECT 1 FROM quiz_revisions r WHERE r.quiz_id=z.id)
 AND EXISTS(SELECT 1 FROM module_steps s JOIN modules m ON m.id=s.module_id WHERE s.quiz_id=z.id AND m.published_at IS NOT NULL AND s.archived_at IS NULL);
INSERT INTO module_revisions
SELECT gen_random_uuid(),org_id,id,1,draft_version,building_key,level,title,description,published_at FROM modules WHERE published_at IS NOT NULL;
UPDATE modules m SET published_revision_id=r.id FROM module_revisions r WHERE r.module_id=m.id;
INSERT INTO module_revision_steps
SELECT s.org_id,s.module_id,r.id,s.id,s.position,s.kind,s.title,s.body,s.rich_content,s.file_id,s.video_id,s.quiz_id,q.id,s.minutes
FROM module_steps s JOIN module_revisions r ON r.module_id=s.module_id
LEFT JOIN quiz_revisions q ON q.quiz_id=s.quiz_id AND q.revision=1 WHERE s.archived_at IS NULL;
INSERT INTO module_revision_pins
SELECT DISTINCT s.org_id,s.module_id,p.user_id,r.id FROM module_progress p JOIN module_steps s ON s.id=p.step_id
JOIN module_revisions r ON r.module_id=s.module_id;

ALTER TABLE assignment_steps ADD COLUMN quiz_revision_id uuid;
ALTER TABLE assignment_steps ADD FOREIGN KEY(org_id,quiz_id,quiz_revision_id) REFERENCES quiz_revisions(org_id,quiz_id,id);
UPDATE assignment_steps s SET quiz_revision_id=r.id FROM quiz_revisions r WHERE r.quiz_id=s.quiz_id AND r.revision=1;
ALTER TABLE quiz_attempts ADD COLUMN quiz_revision_id uuid;
ALTER TABLE quiz_attempts ADD COLUMN module_id uuid;
ALTER TABLE quiz_attempts ADD COLUMN module_revision_id uuid;
ALTER TABLE quiz_attempts ADD FOREIGN KEY(org_id,quiz_id,quiz_revision_id) REFERENCES quiz_revisions(org_id,quiz_id,id);
ALTER TABLE quiz_attempts ADD FOREIGN KEY(org_id,module_id,module_revision_id) REFERENCES module_revisions(org_id,module_id,id);
DROP INDEX quiz_attempts_open_idx;
CREATE UNIQUE INDEX quiz_attempts_open_idx ON quiz_attempts(quiz_id,user_id,COALESCE(assignment_step_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(module_revision_id,'00000000-0000-0000-0000-000000000000'::uuid)) WHERE finished_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION protect_content_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'published content is immutable'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER immutable_module_revisions BEFORE UPDATE OR DELETE ON module_revisions FOR EACH ROW EXECUTE FUNCTION protect_content_revision();
CREATE TRIGGER immutable_module_revision_steps BEFORE UPDATE OR DELETE ON module_revision_steps FOR EACH ROW EXECUTE FUNCTION protect_content_revision();
CREATE TRIGGER immutable_quiz_revisions BEFORE UPDATE OR DELETE ON quiz_revisions FOR EACH ROW EXECUTE FUNCTION protect_content_revision();

-- +goose Down
-- Fail closed even for backfilled legacy data. Production rollback requires reviewed backup restore.
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM module_revisions) OR EXISTS(SELECT 1 FROM quiz_revisions) THEN
  RAISE EXCEPTION 'content revisions exist; restore reviewed backup instead of destructive downgrade';
 END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE quiz_questions DROP CONSTRAINT quiz_questions_options_check;
ALTER TABLE quiz_questions DROP CONSTRAINT quiz_questions_answer_check;
ALTER TABLE quiz_questions ADD CONSTRAINT quiz_questions_options_check CHECK(array_length(options,1)=4 AND array_position(options,NULL) IS NULL);
ALTER TABLE quiz_questions ADD CONSTRAINT quiz_questions_answer_check CHECK(answer BETWEEN 0 AND 3);
DROP TRIGGER immutable_module_revisions ON module_revisions;
DROP TRIGGER immutable_module_revision_steps ON module_revision_steps;
DROP TRIGGER immutable_quiz_revisions ON quiz_revisions;
DROP FUNCTION protect_content_revision();
DROP INDEX quiz_attempts_open_idx;
ALTER TABLE quiz_attempts DROP COLUMN module_revision_id, DROP COLUMN module_id, DROP COLUMN quiz_revision_id;
CREATE UNIQUE INDEX quiz_attempts_open_idx ON quiz_attempts(quiz_id,user_id,COALESCE(assignment_step_id,'00000000-0000-0000-0000-000000000000'::uuid)) WHERE finished_at IS NULL;
ALTER TABLE assignment_steps DROP COLUMN quiz_revision_id, DROP COLUMN rich_content;
ALTER TABLE modules DROP COLUMN published_revision_id;
ALTER TABLE quizzes DROP COLUMN published_revision_id;
DROP TABLE module_revision_pins;
DROP TABLE module_revision_steps;
DROP TABLE module_revisions;
DROP TABLE quiz_revisions;
ALTER TABLE module_steps DROP COLUMN rich_content;
ALTER TABLE modules DROP COLUMN draft_version;
ALTER TABLE quizzes DROP COLUMN draft_version;
