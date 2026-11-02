ALTER TABLE users ADD COLUMN can_create_bank INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN can_export_import INTEGER NOT NULL DEFAULT 0;
ALTER TABLE banks ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private';

CREATE TABLE IF NOT EXISTS bank_members (
  bank_id    INTEGER NOT NULL REFERENCES banks(id) ON DELETE CASCADE,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  level      TEXT    NOT NULL DEFAULT 'viewer' CHECK (level IN ('viewer','editor')),
  created_at TEXT    NOT NULL,
  PRIMARY KEY (bank_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_bank_members_user ON bank_members(user_id);

PRAGMA user_version = 9;
