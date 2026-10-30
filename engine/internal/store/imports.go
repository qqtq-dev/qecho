package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ImportItem struct {
	Row   int
	Input QuestionInput
	Err   string
}

type ImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

type ImportOutcome struct {
	JobID    int64            `json:"jobId"`
	Inserted int              `json:"inserted"`
	Skipped  int              `json:"skipped"`
	Failed   int              `json:"failed"`
	Errors   []ImportRowError `json:"errors"`
}

const importErrorLimit = 200

func (s *Store) QuestionHashes(bankID int64) (map[string]bool, error) {
	rows, err := s.DB.Query(
		"SELECT content_hash FROM questions WHERE bank_id = ? AND is_deleted = 0", bankID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out[h] = true
	}
	return out, rows.Err()
}

func (s *Store) ImportQuestions(bankID int64, sourceName, format string, items []ImportItem) (ImportOutcome, error) {
	out := ImportOutcome{Errors: []ImportRowError{}}
	var bankExists int
	if err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM banks WHERE id = ? AND is_deleted = 0", bankID,
	).Scan(&bankExists); err != nil {
		return out, err
	}
	if bankExists == 0 {
		return out, fmt.Errorf("题库不存在")
	}
	existing, err := s.QuestionHashes(bankID)
	if err != nil {
		return out, err
	}

	now := nowString()
	tx, err := s.DB.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()

	var sortOrder int
	if err := tx.QueryRow(
		"SELECT COALESCE(MAX(sort_order),0) FROM questions WHERE bank_id = ?", bankID,
	).Scan(&sortOrder); err != nil {
		return out, err
	}
	res, err := tx.Exec(`
		INSERT INTO import_jobs(bank_id, source_name, format, status, created_at)
		VALUES(?, ?, ?, 'done', ?)`, bankID, sourceName, format, now)
	if err != nil {
		return out, err
	}
	jobID, _ := res.LastInsertId()
	out.JobID = jobID

	addErr := func(row int, message string) {
		out.Failed++
		if len(out.Errors) < importErrorLimit {
			out.Errors = append(out.Errors, ImportRowError{Row: row, Message: message})
		}
	}

	for _, item := range items {
		if item.Err != "" {
			addErr(item.Row, item.Err)
			continue
		}
		in := item.Input
		if err := validateQuestionInput(&in); err != nil {
			addErr(item.Row, err.Error())
			continue
		}
		hash := questionHash(&in)
		if existing[hash] {
			out.Skipped++
			continue
		}
		existing[hash] = true
		sortOrder++
		answer, _ := json.Marshal(in.Answer)
		qres, err := tx.Exec(`
			INSERT INTO questions(bank_id, type, stem, stem_image, answer, analysis, difficulty,
			                      source, sort_order, content_hash, import_job_id, created_at, updated_at)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			bankID, in.Type, in.Stem, nullable(in.StemImage), string(answer), in.Analysis,
			in.Difficulty, "", sortOrder, hash, jobID, now, now,
		)
		if err != nil {
			return out, err
		}
		qid, _ := qres.LastInsertId()
		for i, o := range in.Options {
			if _, err := tx.Exec(
				"INSERT INTO question_options(question_id, key, content, sort_order) VALUES(?, ?, ?, ?)",
				qid, o.Key, o.Content, i,
			); err != nil {
				return out, err
			}
		}
		out.Inserted++
	}

	if out.Inserted == 0 && out.Failed == 0 {
		return out, fmt.Errorf("没有可导入的题目：全部与题库现有题目重复")
	}
	if _, err := tx.Exec(
		"UPDATE import_jobs SET total = ?, inserted = ?, skipped = ?, failed = ? WHERE id = ?",
		len(items), out.Inserted, out.Skipped, out.Failed, jobID,
	); err != nil {
		return out, err
	}
	if _, err := tx.Exec("UPDATE banks SET updated_at = ? WHERE id = ?", now, bankID); err != nil {
		return out, err
	}
	if err := tx.Commit(); err != nil {
		return out, err
	}
	return out, nil
}

func (s *Store) ImportJobBankID(jobID int64) (int64, error) {
	var bankID int64
	err := s.DB.QueryRow("SELECT bank_id FROM import_jobs WHERE id = ?", jobID).Scan(&bankID)
	if err != nil {
		return 0, err
	}
	return bankID, nil
}

// RevertImport 软删除该批次未被手动编辑过的题目，返回撤销数量。
func (s *Store) RevertImport(jobID int64) (int64, error) {
	var bankID int64
	var status, createdAt string
	var reverted sql.NullString
	err := s.DB.QueryRow(
		"SELECT bank_id, status, created_at, reverted_at FROM import_jobs WHERE id = ?", jobID,
	).Scan(&bankID, &status, &createdAt, &reverted)
	if err != nil {
		return 0, err
	}
	if status != "done" || reverted.Valid {
		return 0, fmt.Errorf("该批次已撤销")
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil || time.Since(created) > 7*24*time.Hour {
		return 0, fmt.Errorf("导入已超过 7 天，不能撤销")
	}

	now := nowString()
	tx, err := s.DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		UPDATE questions SET is_deleted = 1, updated_at = ?
		WHERE import_job_id = ? AND is_deleted = 0 AND updated_at = created_at`, now, jobID)
	if err != nil {
		return 0, err
	}
	count, _ := res.RowsAffected()
	if _, err := tx.Exec(
		"UPDATE import_jobs SET status = 'reverted', reverted_at = ? WHERE id = ?", now, jobID,
	); err != nil {
		return 0, err
	}
	if _, err := tx.Exec("UPDATE banks SET updated_at = ? WHERE id = ?", now, bankID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}
