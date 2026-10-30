CREATE TABLE IF NOT EXISTS banks (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  name        TEXT    NOT NULL,
  description TEXT    NOT NULL DEFAULT '',
  is_deleted  INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT    NOT NULL,
  updated_at  TEXT    NOT NULL
);

CREATE TABLE IF NOT EXISTS questions (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  bank_id      INTEGER NOT NULL REFERENCES banks(id),
  type         TEXT    NOT NULL CHECK (type IN ('single','multiple')),
  stem         TEXT    NOT NULL,
  stem_image   TEXT,
  answer       TEXT    NOT NULL,
  analysis     TEXT    NOT NULL DEFAULT '',
  difficulty   INTEGER NOT NULL DEFAULT 2 CHECK (difficulty BETWEEN 1 AND 5),
  source       TEXT    NOT NULL DEFAULT '',
  sort_order   INTEGER NOT NULL DEFAULT 0,
  content_hash TEXT    NOT NULL,
  is_deleted   INTEGER NOT NULL DEFAULT 0,
  created_at   TEXT    NOT NULL,
  updated_at   TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_questions_bank ON questions(bank_id, is_deleted);
CREATE INDEX IF NOT EXISTS idx_questions_sort ON questions(bank_id, is_deleted, sort_order, id);

CREATE TABLE IF NOT EXISTS question_options (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  question_id INTEGER NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
  key         TEXT    NOT NULL CHECK (length(key) = 1),
  content     TEXT    NOT NULL DEFAULT '',
  sort_order  INTEGER NOT NULL DEFAULT 0,
  UNIQUE (question_id, key)
);

CREATE INDEX IF NOT EXISTS idx_options_question ON question_options(question_id, sort_order);

CREATE TABLE IF NOT EXISTS exams (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  title              TEXT    NOT NULL,
  bank_id            INTEGER NOT NULL REFERENCES banks(id),
  question_ids_json  TEXT    NOT NULL,
  total_count        INTEGER NOT NULL,
  time_limit_sec     INTEGER NOT NULL,
  open_at            TEXT    NOT NULL,
  close_at           TEXT    NOT NULL,
  password_salt      TEXT    NOT NULL,
  password_hash      TEXT    NOT NULL,
  token              TEXT    NOT NULL UNIQUE,
  status             TEXT    NOT NULL DEFAULT 'published'
                             CHECK (status IN ('published','closed')),
  shuffle_questions  INTEGER NOT NULL DEFAULT 1,
  shuffle_options    INTEGER NOT NULL DEFAULT 0,
  created_at         TEXT    NOT NULL,
  updated_at         TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_exams_status ON exams(status, created_at);

CREATE TABLE IF NOT EXISTS exam_attempts (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  exam_id        INTEGER NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  name           TEXT    NOT NULL,
  student_no     TEXT    NOT NULL,
  token          TEXT    NOT NULL UNIQUE,
  status         TEXT    NOT NULL DEFAULT 'ongoing'
                         CHECK (status IN ('ongoing','submitted')),
  started_at     TEXT    NOT NULL,
  deadline_at    TEXT    NOT NULL,
  submitted_at   TEXT,
  score          REAL,
  correct_count  INTEGER NOT NULL DEFAULT 0,
  answered_count INTEGER NOT NULL DEFAULT 0,
  total_count    INTEGER NOT NULL,
  UNIQUE (exam_id, student_no)
);

CREATE INDEX IF NOT EXISTS idx_exam_attempts_exam ON exam_attempts(exam_id, status, started_at);

CREATE TABLE IF NOT EXISTS exam_answers (
  attempt_id    INTEGER NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
  seq           INTEGER NOT NULL,
  question_id   INTEGER NOT NULL,
  snapshot_json TEXT    NOT NULL,
  chosen        TEXT,
  is_correct    INTEGER,
  PRIMARY KEY (attempt_id, seq)
);

PRAGMA user_version = 2;
