package store

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type BackupKind string

const (
	BackupManual     BackupKind = "manual"
	BackupDaily      BackupKind = "daily"
	BackupPreImport  BackupKind = "pre-import"
	BackupPreMigrate BackupKind = "pre-migrate"
	BackupPreRestore BackupKind = "pre-restore"
	BackupPreDelete  BackupKind = "pre-delete"
)

const (
	backupsDirName    = "backups"
	autoBackupsSubdir = "auto"
	restoreMarkerName = "restore.pending.json"
	restoreFailedName = "restore.pending.failed.json"
	backupFormatName  = "qecho-backup"
	backupCapBytes    = int64(2) << 30
)

var backupMu sync.Mutex

func defaultBackupRetention() map[BackupKind]int {
	return map[BackupKind]int{
		BackupDaily:      7,
		BackupPreImport:  5,
		BackupPreMigrate: 3,
		BackupPreRestore: 3,
		BackupPreDelete:  3,
	}
}

type BackupInfo struct {
	Name          string     `json:"name"`
	Kind          BackupKind `json:"kind"`
	Size          int64      `json:"size"`
	CreatedAt     string     `json:"createdAt"`
	SchemaVersion int        `json:"schemaVersion"`
	Auto          bool       `json:"auto"`
}

type backupManifest struct {
	Format        string     `json:"format"`
	FormatVersion int        `json:"formatVersion"`
	Kind          BackupKind `json:"kind"`
	CreatedAt     string     `json:"createdAt"`
	SchemaVersion int        `json:"schemaVersion"`
	DBSHA256      string     `json:"dbSha256"`
	DBSize        int64      `json:"dbSize"`
}

type restoreMarker struct {
	Archive   string `json:"archive"`
	CreatedAt string `json:"createdAt"`
}

func (s *Store) BackupsPath() string {
	return filepath.Join(s.Dir, backupsDirName)
}

func (s *Store) BackupPolicy() (int64, map[string]int) {
	retention := settingsRetention(s.DB)
	out := make(map[string]int, len(retention))
	for kind, limit := range retention {
		out[string(kind)] = limit
	}
	return backupCapBytes, out
}

func (s *Store) ListBackups() ([]BackupInfo, error) {
	backupMu.Lock()
	defer backupMu.Unlock()
	return listBackupsLocked(s.Dir)
}

func (s *Store) CreateBackup(kind BackupKind) (*BackupInfo, error) {
	backupMu.Lock()
	defer backupMu.Unlock()
	return createBackupLocked(s.DB, s.Dir, kind, 0)
}

func (s *Store) DeleteBackup(name string) error {
	backupMu.Lock()
	defer backupMu.Unlock()
	path, err := backupPath(s.Dir, name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	enforceBackupLimitsLocked(s.Dir, settingsRetention(s.DB))
	return nil
}

// EnsureDailyBackup 每天只生成一份，多次启动不会重复占用空间。
func (s *Store) EnsureDailyBackup() (*BackupInfo, error) {
	backupMu.Lock()
	defer backupMu.Unlock()
	items, err := listBackupsLocked(s.Dir)
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("20060102")
	for _, item := range items {
		if item.Kind == BackupDaily && strings.Contains(filepath.Base(item.Name), today) {
			return nil, nil
		}
	}
	return createBackupLocked(s.DB, s.Dir, BackupDaily, 0)
}

func (s *Store) RestorePending() bool {
	_, err := os.Stat(filepath.Join(s.Dir, restoreMarkerName))
	return err == nil
}

// PrepareRestore 校验备份并留下启动标记；真正替换数据库发生在下次启动。
func (s *Store) PrepareRestore(name string) error {
	backupMu.Lock()
	defer backupMu.Unlock()
	if _, err := os.Stat(filepath.Join(s.Dir, restoreMarkerName)); err == nil {
		return errors.New("已有待生效的恢复，请先重启应用或取消恢复")
	}
	path, err := backupPath(s.Dir, name)
	if err != nil {
		return err
	}
	if _, err := inspectBackup(path); err != nil {
		return err
	}
	if _, err := createBackupLocked(s.DB, s.Dir, BackupPreRestore, 0); err != nil {
		return fmt.Errorf("恢复前自动备份失败：%w", err)
	}
	marker := restoreMarker{
		Archive:   normalizeBackupName(path, s.Dir),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return writeJSONAtomic(filepath.Join(s.Dir, restoreMarkerName), marker)
}

func (s *Store) CancelPendingRestore() error {
	backupMu.Lock()
	defer backupMu.Unlock()
	err := os.Remove(filepath.Join(s.Dir, restoreMarkerName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	removeDBFiles(filepath.Join(s.Dir, "quiz.db.restore-tmp"))
	return nil
}

// ApplyPendingRestore 在打开数据库之前执行。失败时保留原数据库并把标记归档。
func ApplyPendingRestore(dir string) error {
	backupMu.Lock()
	defer backupMu.Unlock()
	markerPath := filepath.Join(dir, restoreMarkerName)
	data, err := os.ReadFile(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		removeDBFiles(filepath.Join(dir, "quiz.db.restore-tmp"))
		return nil
	}
	if err != nil {
		quarantineRestoreMarker(dir, fmt.Errorf("读取恢复标记失败：%w", err))
		return err
	}
	var marker restoreMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		quarantineRestoreMarker(dir, fmt.Errorf("恢复标记格式错误：%w", err))
		return err
	}
	archive, err := backupPath(dir, marker.Archive)
	if err != nil {
		quarantineRestoreMarker(dir, err)
		return err
	}
	tmpDB := filepath.Join(dir, "quiz.db.restore-tmp")
	if err := applyBackupArchive(archive, dir, tmpDB); err != nil {
		removeDBFiles(tmpDB)
		quarantineRestoreMarker(dir, err)
		return err
	}
	os.Remove(tmpDB + "-wal")
	os.Remove(tmpDB + "-shm")

	dbPath := filepath.Join(dir, "quiz.db")
	oldPath := filepath.Join(dir, "quiz.db.restore-old")
	os.Remove(oldPath)
	if err := os.Rename(dbPath, oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		removeDBFiles(tmpDB)
		quarantineRestoreMarker(dir, fmt.Errorf("暂存当前数据库失败：%w", err))
		return err
	}
	// WAL/SHM 属于旧库，必须在换上备份库之前清理，否则会被误认作新库日志。
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")
	if err := os.Rename(tmpDB, dbPath); err != nil {
		os.Rename(oldPath, dbPath)
		removeDBFiles(tmpDB)
		quarantineRestoreMarker(dir, fmt.Errorf("替换数据库失败：%w", err))
		return err
	}
	os.Remove(oldPath)
	if err := extractBackupImages(archive, dir); err != nil {
		log.Printf("restore images: %v", err)
	}
	os.Remove(markerPath)
	log.Printf("restore applied: %s", marker.Archive)
	return nil
}

func createBackupLocked(db *sql.DB, dir string, kind BackupKind, fromVersion int) (*BackupInfo, error) {
	if kind == "" {
		kind = BackupManual
	}
	backupsDir := filepath.Join(dir, backupsDirName)
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建备份目录失败：%w", err)
	}
	now := time.Now()
	var target string
	if kind == BackupManual {
		target = filepath.Join(backupsDir, "quizbox-"+now.Format("20060102-150405")+".zip")
	} else {
		autoDir := filepath.Join(backupsDir, autoBackupsSubdir)
		if err := os.MkdirAll(autoDir, 0o755); err != nil {
			return nil, fmt.Errorf("创建备份目录失败：%w", err)
		}
		name := "quizbox-" + string(kind) + "-"
		if fromVersion > 0 {
			name += fmt.Sprintf("v%d-", fromVersion)
		}
		target = filepath.Join(autoDir, name+now.Format("20060102-150405")+".zip")
	}
	if _, err := os.Stat(target); err == nil {
		target = strings.TrimSuffix(target, ".zip") + fmt.Sprintf("-%d.zip", now.UnixNano()%1000)
	}

	tmpDir := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建临时目录失败：%w", err)
	}
	snapPath := filepath.Join(tmpDir, fmt.Sprintf("snapshot-%d.db", now.UnixNano()))
	defer os.Remove(snapPath)
	if err := vacuumInto(db, snapPath); err != nil {
		return nil, err
	}
	sha, size, err := fileSHA256(snapPath)
	if err != nil {
		return nil, err
	}
	schemaVersion, err := dbSchemaVersion(db)
	if err != nil {
		return nil, err
	}
	manifest := backupManifest{
		Format:        backupFormatName,
		FormatVersion: 1,
		Kind:          kind,
		CreatedAt:     now.UTC().Format(time.RFC3339),
		SchemaVersion: schemaVersion,
		DBSHA256:      sha,
		DBSize:        size,
	}
	part := target + ".part"
	if err := writeBackupArchive(part, manifest, snapPath, filepath.Join(dir, "images")); err != nil {
		os.Remove(part)
		return nil, err
	}
	if err := os.Rename(part, target); err != nil {
		os.Remove(part)
		return nil, fmt.Errorf("保存备份失败：%w", err)
	}
	info, err := backupInfoFor(dir, target)
	if err != nil {
		return nil, err
	}
	if err := enforceBackupLimitsLocked(dir, settingsRetention(db)); err != nil {
		log.Printf("cleanup backups: %v", err)
	}
	return info, nil
}

func vacuumInto(db *sql.DB, path string) error {
	os.Remove(path)
	quoted := strings.ReplaceAll(path, "'", "''")
	if _, err := db.Exec("VACUUM INTO '" + quoted + "'"); err != nil {
		return fmt.Errorf("生成数据库快照失败：%w", err)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("生成数据库快照失败：%w", err)
	}
	return nil
}

func writeBackupArchive(target string, manifest backupManifest, dbPath, imagesDir string) error {
	f, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("创建备份文件失败：%w", err)
	}
	zw := zip.NewWriter(f)
	fail := func(err error) error {
		zw.Close()
		f.Close()
		return err
	}
	dbFile, err := os.Open(dbPath)
	if err != nil {
		return fail(err)
	}
	defer dbFile.Close()
	header := &zip.FileHeader{Name: "manifest.json", Method: zip.Deflate, Modified: time.Now()}
	header.SetMode(0o644)
	mw, err := zw.CreateHeader(header)
	if err != nil {
		return fail(err)
	}
	if err := json.NewEncoder(mw).Encode(manifest); err != nil {
		return fail(err)
	}
	hdr := &zip.FileHeader{Name: "quiz.db", Method: zip.Deflate, Modified: time.Now()}
	hdr.SetMode(0o644)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fail(err)
	}
	if _, err := io.Copy(w, dbFile); err != nil {
		return fail(err)
	}
	if info, err := os.Stat(imagesDir); err == nil && info.IsDir() {
		err = filepath.WalkDir(imagesDir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(imagesDir, path)
			if err != nil {
				return err
			}
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			stat, err := src.Stat()
			if err != nil {
				src.Close()
				return err
			}
			ih := &zip.FileHeader{
				Name:     "images/" + filepath.ToSlash(rel),
				Method:   zip.Store,
				Modified: stat.ModTime(),
			}
			ih.SetMode(0o644)
			dst, err := zw.CreateHeader(ih)
			if err != nil {
				src.Close()
				return err
			}
			_, err = io.Copy(dst, src)
			src.Close()
			return err
		})
		if err != nil {
			return fail(err)
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
