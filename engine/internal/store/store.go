package store

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Store struct {
	DB  *sql.DB
	Dir string
}

var migrationFiles = []string{
	"migrations/0001_init.sql",
	"migrations/0002_exams.sql",
	"migrations/0003_imports.sql",
	"migrations/0004_accounts.sql",
	"migrations/0005_practice.sql",
	"migrations/0006_images.sql",
	"migrations/0007_practice_resume.sql",
	"migrations/0008_polish.sql",
	"migrations/0009_sharing.sql",
	"migrations/0010_notices.sql",
}

func LatestSchemaVersion() int {
	return len(migrationFiles)
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dir, "quiz.db")
	existing := false
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		existing = true
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单写连接，避免 database/sql 连接池造成锁竞争。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	if existing {
		version, err := dbSchemaVersion(db)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("read schema version: %w", err)
		}
		if version < LatestSchemaVersion() {
			backupMu.Lock()
			_, err = createBackupLocked(db, dir, BackupPreMigrate, version)
			backupMu.Unlock()
			if err != nil {
				db.Close()
				return nil, fmt.Errorf("backup before migration: %w", err)
			}
		}
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db, Dir: dir}, nil
}

func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	for i, file := range migrationFiles {
		target := i + 1
		if version >= target {
			continue
		}
		script, err := migrations.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		for _, stmt := range splitStatements(string(script)) {
			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("migration %s statement %q: %w", file, stmt, err)
			}
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", target)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		version = target
	}
	return nil
}

func splitStatements(script string) []string {
	var out []string
	for _, part := range strings.Split(script, ";") {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasPrefix(part, "--") {
			continue
		}
		out = append(out, part)
	}
	return out
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) Ping() error {
	return s.DB.Ping()
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (s *Store) PruneSessions() error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
