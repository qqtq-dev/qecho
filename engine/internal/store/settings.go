package store

import (
	"database/sql"
	"strconv"
)

type Settings struct {
	PracticeCount            int  `json:"practiceCount"`
	PracticeShuffleQuestions bool `json:"practiceShuffleQuestions"`
	PracticeShuffleOptions   bool `json:"practiceShuffleOptions"`
	WrongMasterThreshold     int  `json:"wrongMasterThreshold"`
	BackupDaily              int  `json:"backupDaily"`
	BackupPreImport          int  `json:"backupPreImport"`
	BackupPreMigrate         int  `json:"backupPreMigrate"`
}

func defaultSettings() Settings {
	return Settings{
		PracticeCount:            20,
		PracticeShuffleQuestions: true,
		PracticeShuffleOptions:   false,
		WrongMasterThreshold:     2,
		BackupDaily:              7,
		BackupPreImport:          5,
		BackupPreMigrate:         3,
	}
}

func (s Settings) normalized() Settings {
	if s.PracticeCount != 0 && (s.PracticeCount < 5 || s.PracticeCount > 500) {
		s.PracticeCount = 20
	}
	if s.WrongMasterThreshold < 1 || s.WrongMasterThreshold > 10 {
		s.WrongMasterThreshold = 2
	}
	s.BackupDaily = clampInt(s.BackupDaily, 1, 30)
	s.BackupPreImport = clampInt(s.BackupPreImport, 1, 30)
	s.BackupPreMigrate = clampInt(s.BackupPreMigrate, 1, 30)
	return s
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (s *Store) GetSettings() (*Settings, error) {
	out := defaultSettings()
	rows, err := s.DB.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		applySetting(&out, key, value)
	}
	return &out, rows.Err()
}

func applySetting(out *Settings, key, value string) {
	num, err := strconv.Atoi(value)
	switch key {
	case "practice_count":
		if err == nil {
			out.PracticeCount = num
		}
	case "practice_shuffle_questions":
		out.PracticeShuffleQuestions = value == "1" || value == "true"
	case "practice_shuffle_options":
		out.PracticeShuffleOptions = value == "1" || value == "true"
	case "wrong_master_threshold":
		if err == nil {
			out.WrongMasterThreshold = num
		}
	case "backup_daily":
		if err == nil {
			out.BackupDaily = num
		}
	case "backup_pre_import":
		if err == nil {
			out.BackupPreImport = num
		}
	case "backup_pre_migrate":
		if err == nil {
			out.BackupPreMigrate = num
		}
	}
}

func (s *Store) UpdateSettings(in Settings) (*Settings, error) {
	in = in.normalized()
	values := map[string]string{
		"practice_count":             strconv.Itoa(in.PracticeCount),
		"practice_shuffle_questions": boolString(in.PracticeShuffleQuestions),
		"practice_shuffle_options":   boolString(in.PracticeShuffleOptions),
		"wrong_master_threshold":     strconv.Itoa(in.WrongMasterThreshold),
		"backup_daily":               strconv.Itoa(in.BackupDaily),
		"backup_pre_import":          strconv.Itoa(in.BackupPreImport),
		"backup_pre_migrate":         strconv.Itoa(in.BackupPreMigrate),
	}
	for key, value := range values {
		if err := s.SetSetting(key, value); err != nil {
			return nil, err
		}
	}
	return &in, nil
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (s *Store) WrongMasterThreshold() int {
	var raw string
	err := s.DB.QueryRow("SELECT value FROM settings WHERE key = 'wrong_master_threshold'").Scan(&raw)
	if err != nil {
		return 2
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 10 {
		return 2
	}
	return n
}

func settingsRetention(db queryer) map[BackupKind]int {
	out := map[BackupKind]int{
		BackupDaily:      7,
		BackupPreImport:  5,
		BackupPreMigrate: 3,
		BackupPreRestore: 3,
		BackupPreDelete:  3,
	}
	for key, kind := range map[string]BackupKind{
		"backup_daily":       BackupDaily,
		"backup_pre_import":  BackupPreImport,
		"backup_pre_migrate": BackupPreMigrate,
	} {
		var raw string
		if err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&raw); err != nil {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err == nil && n >= 1 && n <= 30 {
			out[kind] = n
		}
	}
	return out
}

type queryer interface {
	QueryRow(query string, args ...any) *sql.Row
}
