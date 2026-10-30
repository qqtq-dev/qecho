CREATE TABLE IF NOT EXISTS import_jobs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  bank_id     INTEGER NOT NULL REFERENCES banks(id),
  source_name TEXT    NOT NULL DEFAULT '',
  format      TEXT    NOT NULL DEFAULT '',
  total       INTEGER NOT NULL DEFAULT 0,
  inserted    INTEGER NOT NULL DEFAULT 0,
  skipped     INTEGER NOT NULL DEFAULT 0,
  failed      INTEGER NOT NULL DEFAULT 0,
  status      TEXT    NOT NULL DEFAULT 'done'
                      CHECK (status IN ('done','reverted')),
  created_at  TEXT    NOT NULL,
  reverted_at TEXT
);

ALTER TABLE questions ADD COLUMN import_job_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_questions_import ON questions(import_job_id);
CREATE INDEX IF NOT EXISTS idx_import_jobs_bank ON import_jobs(bank_id, created_at);

PRAGMA user_version = 3;
