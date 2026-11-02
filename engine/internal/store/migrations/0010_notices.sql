CREATE TABLE IF NOT EXISTS user_notices (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT    NOT NULL,
  text       TEXT    NOT NULL,
  bank_id    INTEGER,
  created_at TEXT    NOT NULL,
  read_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_user_notices_user ON user_notices(user_id, id DESC);

PRAGMA user_version = 10;
