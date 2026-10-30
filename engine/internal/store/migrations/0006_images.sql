CREATE TABLE IF NOT EXISTS images (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  filename      TEXT    NOT NULL UNIQUE,
  original_name TEXT    NOT NULL DEFAULT '',
  sha256        TEXT    NOT NULL UNIQUE,
  mime          TEXT    NOT NULL,
  size          INTEGER NOT NULL,
  created_at    TEXT    NOT NULL
);

ALTER TABLE question_options ADD COLUMN image TEXT;
