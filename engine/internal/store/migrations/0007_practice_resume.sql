ALTER TABLE practice_sessions ADD COLUMN status TEXT NOT NULL DEFAULT 'ongoing';
ALTER TABLE practice_sessions ADD COLUMN finished_at TEXT;
ALTER TABLE practice_sessions ADD COLUMN last_seq INTEGER NOT NULL DEFAULT 1;
ALTER TABLE practice_sessions ADD COLUMN scope_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE practice_sessions ADD COLUMN shuffle_options INTEGER NOT NULL DEFAULT 0;
ALTER TABLE practice_sessions ADD COLUMN option_orders_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE practice_sessions ADD COLUMN marked_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE practice_sessions ADD COLUMN duration_sec INTEGER NOT NULL DEFAULT 0;
ALTER TABLE practice_sessions ADD COLUMN bank_name_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE practice_answers ADD COLUMN time_spent_ms INTEGER NOT NULL DEFAULT 0;

UPDATE practice_sessions
SET status = 'finished', finished_at = updated_at
WHERE (SELECT COUNT(*) FROM practice_answers a WHERE a.session_id = practice_sessions.id) >= total_count;

PRAGMA user_version = 7;
