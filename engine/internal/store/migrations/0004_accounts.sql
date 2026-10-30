CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
  password_salt TEXT    NOT NULL DEFAULT '',
  password_hash TEXT    NOT NULL DEFAULT '',
  role          TEXT    NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
  is_disabled   INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT    NOT NULL,
  last_login_at TEXT
);

ALTER TABLE sessions ADD COLUMN user_id INTEGER NOT NULL DEFAULT 0;
ALTER TABLE banks ADD COLUMN owner_id INTEGER;
ALTER TABLE exams ADD COLUMN owner_id INTEGER;

CREATE INDEX IF NOT EXISTS idx_banks_owner ON banks(owner_id, is_deleted);
CREATE INDEX IF NOT EXISTS idx_exams_owner ON exams(owner_id, status, created_at);

PRAGMA user_version = 4;
