package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := st.SetSetting("backup-test", "before"); err != nil {
		t.Fatalf("seed setting: %v", err)
	}
	created, err := st.CreateBackup(BackupManual)
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if created.Kind != BackupManual || created.Size == 0 {
		t.Fatalf("unexpected backup info: %+v", created)
	}
	items, err := st.ListBackups()
	if err != nil || len(items) != 1 {
		t.Fatalf("list backups: %v, %d", err, len(items))
	}
	if err := st.SetSetting("backup-test", "after"); err != nil {
		t.Fatalf("mutate setting: %v", err)
	}
	if err := st.PrepareRestore(created.Name); err != nil {
		t.Fatalf("prepare restore: %v", err)
	}
	if !st.RestorePending() {
		t.Fatal("restore marker missing")
	}
	if _, err := os.Stat(filepath.Join(dir, restoreMarkerName)); err != nil {
		t.Fatalf("marker file: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := ApplyPendingRestore(dir); err != nil {
		t.Fatalf("apply restore: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, restoreMarkerName)); !os.IsNotExist(err) {
		t.Fatalf("marker not removed: %v", err)
	}
	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	value, err := st2.GetSetting("backup-test")
	if err != nil {
		t.Fatalf("read setting: %v", err)
	}
	if value != "before" {
		t.Fatalf("restored value = %q, want before", value)
	}
	if st2.RestorePending() {
		t.Fatal("pending should be false after restore")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "quiz.db.restore-") {
			t.Fatalf("restore temp file left behind: %s", entry.Name())
		}
	}
}

func TestBackupRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	for _, name := range []string{"../quizbox.zip", "auto/../../quizbox.zip", "quizbox.txt", ""} {
		if err := st.DeleteBackup(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}

func TestInvalidBackupCannotBeRestored(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	bad := filepath.Join(dir, backupsDirName, "quizbox-broken.zip")
	if err := os.MkdirAll(filepath.Dir(bad), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := st.PrepareRestore("quizbox-broken.zip"); err == nil {
		t.Fatal("expected restore to fail")
	}
	if st.RestorePending() {
		t.Fatal("marker should not be written for invalid backup")
	}
}

func TestDailyBackupOnlyOncePerDay(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer st.Close()
	first, err := st.EnsureDailyBackup()
	if err != nil || first == nil {
		t.Fatalf("first daily backup: %v, %+v", err, first)
	}
	second, err := st.EnsureDailyBackup()
	if err != nil || second != nil {
		t.Fatalf("second daily backup should be skipped: %v, %+v", err, second)
	}
}

func TestPreMigrationBackup(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// 模拟 v5 数据库：去掉 v6 以后新增的列，再回退版本号。
	drops := []string{
		"ALTER TABLE question_options DROP COLUMN image",
		"ALTER TABLE practice_sessions DROP COLUMN status",
		"ALTER TABLE practice_sessions DROP COLUMN finished_at",
		"ALTER TABLE practice_sessions DROP COLUMN last_seq",
		"ALTER TABLE practice_sessions DROP COLUMN scope_json",
		"ALTER TABLE practice_sessions DROP COLUMN shuffle_options",
		"ALTER TABLE practice_sessions DROP COLUMN option_orders_json",
		"ALTER TABLE practice_sessions DROP COLUMN marked_json",
		"ALTER TABLE practice_sessions DROP COLUMN duration_sec",
		"ALTER TABLE practice_sessions DROP COLUMN bank_name_snapshot",
		"ALTER TABLE practice_answers DROP COLUMN time_spent_ms",
		"ALTER TABLE import_jobs DROP COLUMN errors_json",
		"ALTER TABLE import_jobs DROP COLUMN duplicate_policy",
		"ALTER TABLE exam_answers DROP COLUMN option_order",
		"ALTER TABLE users DROP COLUMN can_create_bank",
		"ALTER TABLE users DROP COLUMN can_export_import",
		"ALTER TABLE banks DROP COLUMN visibility",
	}
	for _, stmt := range drops {
		if _, err := st.DB.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if _, err := st.DB.Exec("PRAGMA user_version = 5"); err != nil {
		t.Fatalf("set version: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer st2.Close()
	items, err := st2.ListBackups()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, item := range items {
		if item.Kind == BackupPreMigrate && item.SchemaVersion == 5 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no pre-migrate backup: %+v", items)
	}
}
