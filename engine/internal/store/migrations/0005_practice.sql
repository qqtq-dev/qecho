CREATE TABLE IF NOT EXISTS practice_sessions (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id            INTEGER NOT NULL,
  bank_id            INTEGER NOT NULL DEFAULT 0,
  source             TEXT NOT NULL DEFAULT 'bank',
  question_ids_json  TEXT NOT NULL,
  total_count        INTEGER NOT NULL,
  order_mode         TEXT NOT NULL DEFAULT 'random',
  created_at         TEXT NOT NULL,
  updated_at         TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS practice_answers (
  session_id   INTEGER NOT NULL REFERENCES practice_sessions(id) ON DELETE CASCADE,
  question_id  INTEGER NOT NULL,
  chosen_json  TEXT NOT NULL,
  is_correct   INTEGER NOT NULL,
  answered_at  TEXT NOT NULL,
  PRIMARY KEY(session_id, question_id)
);

CREATE TABLE IF NOT EXISTS wrong_book (
  user_id        INTEGER NOT NULL,
  question_id    INTEGER NOT NULL,
  wrong_count    INTEGER NOT NULL DEFAULT 1,
  correct_streak INTEGER NOT NULL DEFAULT 0,
  mastered       INTEGER NOT NULL DEFAULT 0,
  last_wrong_at  TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL,
  updated_at     TEXT NOT NULL,
  PRIMARY KEY(user_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_practice_sessions_user ON practice_sessions(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_wrong_book_user ON wrong_book(user_id, mastered, updated_at);

PRAGMA user_version = 5;
