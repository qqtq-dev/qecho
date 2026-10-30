CREATE TABLE IF NOT EXISTS tags (
  id   INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS question_tags (
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  tag_id      INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (question_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_question_tags_tag ON question_tags(tag_id);

ALTER TABLE import_jobs ADD COLUMN errors_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE import_jobs ADD COLUMN duplicate_policy TEXT NOT NULL DEFAULT 'skip';
ALTER TABLE exam_answers ADD COLUMN option_order TEXT NOT NULL DEFAULT '';

PRAGMA user_version = 8;
