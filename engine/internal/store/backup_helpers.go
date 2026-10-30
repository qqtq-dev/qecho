package store

import (
	"archive/zip"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func listBackupsLocked(dir string) ([]BackupInfo, error) {
	root := filepath.Join(dir, backupsDirName)
	items := []BackupInfo{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".zip") {
			return nil
		}
		info, err := backupInfoFor(dir, path)
		if err != nil {
			log.Printf("read backup %s: %v", path, err)
			return nil
		}
		items = append(items, *info)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].Name > items[j].Name
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	return items, nil
}

func backupInfoFor(dir, path string) (*BackupInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	name := normalizeBackupName(path, dir)
	kind := parseBackupKind(filepath.Base(path))
	schemaVersion := 0
	if manifest, err := readBackupManifest(path); err == nil && manifest != nil {
		schemaVersion = manifest.SchemaVersion
	}
	return &BackupInfo{
		Name:          name,
		Kind:          kind,
		Size:          stat.Size(),
		CreatedAt:     stat.ModTime().UTC().Format(time.RFC3339Nano),
		SchemaVersion: schemaVersion,
		Auto:          kind != BackupManual,
	}, nil
}

func normalizeBackupName(path, dir string) string {
	rel, err := filepath.Rel(filepath.Join(dir, backupsDirName), path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.ToSlash(rel)
}

func parseBackupKind(base string) BackupKind {
	switch {
	case strings.HasPrefix(base, "quizbox-daily-"):
		return BackupDaily
	case strings.HasPrefix(base, "quizbox-pre-import-"):
		return BackupPreImport
	case strings.HasPrefix(base, "quizbox-pre-migrate-"):
		return BackupPreMigrate
	case strings.HasPrefix(base, "quizbox-pre-restore-"):
		return BackupPreRestore
	default:
		return BackupManual
	}
}

func backupPath(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("备份名称不能为空")
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || strings.Contains(clean, ":") || !strings.HasSuffix(strings.ToLower(clean), ".zip") {
		return "", errors.New("备份名称无效")
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("备份名称无效")
		}
	}
	root := filepath.Join(dir, backupsDirName)
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errors.New("备份名称无效")
	}
	return full, nil
}

func enforceBackupLimitsLocked(dir string, retention map[BackupKind]int) error {
	items, err := listBackupsLocked(dir)
	if err != nil {
		return err
	}
	byKind := map[BackupKind][]BackupInfo{}
	for _, item := range items {
		if item.Auto {
			byKind[item.Kind] = append(byKind[item.Kind], item)
		}
	}
	for kind, limit := range retention {
		group := byKind[kind]
		if len(group) <= limit {
			continue
		}
		sort.Slice(group, func(i, j int) bool { return group[i].CreatedAt < group[j].CreatedAt })
		for _, item := range group[:len(group)-limit] {
			path, err := backupPath(dir, item.Name)
			if err != nil {
				continue
			}
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("remove old backup %s: %v", item.Name, err)
			}
		}
	}

	items, err = listBackupsLocked(dir)
	if err != nil {
		return err
	}
	auto := make([]BackupInfo, 0, len(items))
	var total int64
	for _, item := range items {
		if item.Auto {
			auto = append(auto, item)
			total += item.Size
		}
	}
	sort.Slice(auto, func(i, j int) bool { return auto[i].CreatedAt < auto[j].CreatedAt })
	for total > backupCapBytes && len(auto) > 1 {
		oldest := auto[0]
		path, err := backupPath(dir, oldest.Name)
		if err != nil {
			auto = auto[1:]
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("remove old backup %s: %v", oldest.Name, err)
		}
		total -= oldest.Size
		auto = auto[1:]
	}
	return nil
}

func fileSHA256(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

func dbSchemaVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func readBackupManifest(path string) (*backupManifest, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	for _, entry := range zr.File {
		if entry.Name != "manifest.json" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		var manifest backupManifest
		if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
			return nil, err
		}
		if manifest.Format != backupFormatName {
			return nil, errors.New("不是 Qecho 备份文件")
		}
		return &manifest, nil
	}
	return nil, nil
}

func inspectBackup(archive string) (int, error) {
	tmp, err := os.CreateTemp("", "qecho-inspect-*.db")
	if err != nil {
		return 0, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	removeDBFiles(tmpPath)
	defer removeDBFiles(tmpPath)
	if err := extractBackupDB(archive, tmpPath); err != nil {
		return 0, err
	}
	return verifyBackupDB(tmpPath)
}

func extractBackupDB(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return errors.New("备份文件已损坏或不是 Qecho 备份")
	}
	defer zr.Close()
	var manifest *backupManifest
	for _, entry := range zr.File {
		if entry.Name != "manifest.json" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return err
		}
		var m backupManifest
		if err := json.NewDecoder(rc).Decode(&m); err == nil && m.Format == backupFormatName {
			manifest = &m
		}
		rc.Close()
		break
	}
	var dbEntry *zip.File
	for _, entry := range zr.File {
		if entry.Name == "quiz.db" {
			dbEntry = entry
			break
		}
	}
	if dbEntry == nil {
		return errors.New("备份中缺少数据库文件")
	}
	rc, err := dbEntry.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	os.Remove(dest)
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(out, h), rc); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if manifest != nil && manifest.DBSHA256 != "" {
		if hex.EncodeToString(h.Sum(nil)) != manifest.DBSHA256 {
			return errors.New("备份校验失败：数据库内容与校验值不一致")
		}
	}
	return nil
}

func verifyBackupDB(path string) (int, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, errors.New("无法读取备份数据库")
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return 0, errors.New("备份数据库无法通过完整性校验")
	}
	if result != "ok" {
		return 0, fmt.Errorf("备份数据库完整性校验失败：%s", result)
	}
	var tables int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('sessions', 'settings')").Scan(&tables); err != nil || tables < 2 {
		return 0, errors.New("备份不是有效的 Qecho 数据库")
	}
	version, err := dbSchemaVersion(db)
	if err != nil {
		return 0, err
	}
	if version < 1 {
		return 0, errors.New("备份数据库版本无效")
	}
	if version > LatestSchemaVersion() {
		return 0, fmt.Errorf("备份来自更高版本的应用（数据版本 %d），请升级后再恢复", version)
	}
	return version, nil
}

func applyBackupArchive(archive, dir, tmpDB string) error {
	removeDBFiles(tmpDB)
	if err := extractBackupDB(archive, tmpDB); err != nil {
		return err
	}
	if _, err := verifyBackupDB(tmpDB); err != nil {
		return err
	}
	return nil
}

func extractBackupImages(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer zr.Close()
	imagesDir := filepath.Join(dir, "images")
	var firstErr error
	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() || !strings.HasPrefix(entry.Name, "images/") {
			continue
		}
		base := filepath.Base(entry.Name)
		if base == "" || base == "." || base == ".." {
			continue
		}
		if err := os.MkdirAll(imagesDir, 0o755); err != nil {
			return err
		}
		rc, err := entry.Open()
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		target := filepath.Join(imagesDir, base)
		tmp := target + ".restore-tmp"
		out, err := os.Create(tmp)
		if err != nil {
			rc.Close()
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			os.Remove(tmp)
			if firstErr == nil {
				firstErr = copyErr
			}
			continue
		}
		if err := os.Rename(tmp, target); err != nil {
			os.Remove(tmp)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func quarantineRestoreMarker(dir string, cause error) {
	markerPath := filepath.Join(dir, restoreMarkerName)
	failed := filepath.Join(dir, restoreFailedName)
	os.Remove(failed)
	if err := os.Rename(markerPath, failed); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("quarantine restore marker: %v", err)
	}
	log.Printf("pending restore failed: %v", cause)
}

func removeDBFiles(path string) {
	os.Remove(path)
	os.Remove(path + "-wal")
	os.Remove(path + "-shm")
}
