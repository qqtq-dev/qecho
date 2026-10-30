package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrPracticeNotFound = errors.New("练习不存在")
	ErrQuestionNotInSet = errors.New("题目不在本次练习中")
)

type PracticeSession struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"userId"`
	BankID     int64  `json:"bankId"`
	BankName   string `json:"bankName"`
	Source     string `json:"source"`
	TotalCount int    `json:"totalCount"`
	OrderMode  string `json:"order"`
	Answered   int    `json:"answeredCount"`
	Correct    int    `json:"correctCount"`
	CreatedAt  string `json:"createdAt"`
}

type PracticeItem struct {
	ID         int64            `json:"id"`
	Type       string           `json:"type"`
	Stem       string           `json:"stem"`
	StemImage  string           `json:"stemImage"`
	Difficulty int              `json:"difficulty"`
	Options    []QuestionOption `json:"options"`
}

type PracticeAnswerResult struct {
	IsCorrect     bool       `json:"isCorrect"`
	Answer        []string   `json:"answer"`
	Analysis      string     `json:"analysis"`
	WrongCount    int        `json:"wrongCount"`
	CorrectStreak int        `json:"correctStreak"`
	Mastered      bool       `json:"mastered"`
	Stats         WrongStats `json:"stats"`
}

type WrongStats struct {
	Pending  int `json:"pending"`
	Mastered int `json:"mastered"`
	Total    int `json:"total"`
}

type WrongItem struct {
	QuestionID    int64            `json:"questionId"`
	BankID        int64            `json:"bankId"`
	BankName      string           `json:"bankName"`
	Type          string           `json:"type"`
	Stem          string           `json:"stem"`
	StemImage     string           `json:"stemImage"`
	Answer        []string         `json:"answer"`
	Analysis      string           `json:"analysis"`
	Difficulty    int              `json:"difficulty"`
	Options       []QuestionOption `json:"options"`
	WrongCount    int              `json:"wrongCount"`
	CorrectStreak int              `json:"correctStreak"`
	Mastered      bool             `json:"mastered"`
	LastWrongAt   string           `json:"lastWrongAt"`
	UpdatedAt     string           `json:"updatedAt"`
}

func (s *Store) CreatePracticeSession(userID, bankID int64, count int, order, source, questionType string) (*PracticeSession, []PracticeItem, error) {
	if order != "fixed" {
		order = "random"
	}
	if source != "wrong" {
		source = "bank"
	}
	if questionType != "single" && questionType != "multiple" {
		questionType = ""
	}
	if count < 1 {
		count = 20
	}
	if count > 300 {
		count = 300
	}

	ids := []int64{}
	bankName := "错题本"
	queryOrder := " ORDER BY RANDOM()"
	if order == "fixed" {
		queryOrder = " ORDER BY q.sort_order, q.id"
	}

	if source == "wrong" {
		query := `SELECT w.question_id FROM wrong_book w
			JOIN questions q ON q.id = w.question_id AND q.is_deleted = 0
			WHERE w.user_id = ? AND w.mastered = 0`
		args := []any{userID}
		if bankID > 0 {
			query += " AND q.bank_id = ?"
			args = append(args, bankID)
		}
		if questionType != "" {
			query += " AND q.type = ?"
			args = append(args, questionType)
		}
		query += queryOrder + " LIMIT ?"
		args = append(args, count)
		rows, err := s.DB.Query(query, args...)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, nil, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, nil, err
		}
		if bankID > 0 {
			if err := s.DB.QueryRow("SELECT name FROM banks WHERE id = ?", bankID).Scan(&bankName); err != nil {
				bankName = "错题本"
			}
		}
	} else {
		if err := s.DB.QueryRow(
			"SELECT name FROM banks WHERE id = ? AND is_deleted = 0", bankID,
		).Scan(&bankName); err != nil {
			return nil, nil, fmt.Errorf("题库不存在")
		}
		where := "q.bank_id = ? AND q.is_deleted = 0"
		args := []any{bankID}
		if questionType != "" {
			where += " AND q.type = ?"
			args = append(args, questionType)
		}
		args = append(args, count)
		rows, err := s.DB.Query(
			"SELECT q.id FROM questions q WHERE "+where+queryOrder+" LIMIT ?", args...,
		)
		if err != nil {
			return nil, nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, nil, err
			}
			ids = append(ids, id)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, nil, err
		}
		if len(ids) == 0 {
			if questionType != "" {
				return nil, nil, fmt.Errorf("没有该题型的题目")
			}
			return nil, nil, fmt.Errorf("这个题库还没有题目")
		}
	}
	if len(ids) == 0 {
		if source == "wrong" {
			return nil, nil, fmt.Errorf("错题本里还没有待复习的题目")
		}
		return nil, nil, fmt.Errorf("没有可练习的题目")
	}

	now := nowString()
	idsJSON, _ := json.Marshal(ids)
	res, err := s.DB.Exec(`
		INSERT INTO practice_sessions(user_id, bank_id, source, question_ids_json, total_count,
		                              order_mode, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, bankID, source, string(idsJSON), len(ids), order, now, now,
	)
	if err != nil {
		return nil, nil, err
	}
	id, _ := res.LastInsertId()
	session := &PracticeSession{
		ID: id, UserID: userID, BankID: bankID, BankName: bankName, Source: source,
		TotalCount: len(ids), OrderMode: order, CreatedAt: now,
	}
	items, err := s.practiceItems(ids)
	if err != nil {
		return nil, nil, err
	}
	return session, items, nil
}

func (s *Store) practiceItems(ids []int64) ([]PracticeItem, error) {
	if len(ids) == 0 {
		return []PracticeItem{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.DB.Query(`
		SELECT id, type, stem, COALESCE(stem_image,''), difficulty
		FROM questions WHERE id IN (`+placeholders+`) AND is_deleted = 0`, args...)
	if err != nil {
		return nil, err
	}
	byID := map[int64]PracticeItem{}
	for rows.Next() {
		var item PracticeItem
		if err := rows.Scan(&item.ID, &item.Type, &item.Stem, &item.StemImage, &item.Difficulty); err != nil {
			rows.Close()
			return nil, err
		}
		byID[item.ID] = item
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	out := make([]PracticeItem, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			continue
		}
		opts, err := s.questionOptions(id)
		if err != nil {
			return nil, err
		}
		item.Options = opts
		out = append(out, item)
	}
	return out, nil
}

func (s *Store) questionOptions(questionID int64) ([]QuestionOption, error) {
	rows, err := s.DB.Query(
		"SELECT key, content FROM question_options WHERE question_id = ? ORDER BY sort_order, key",
		questionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []QuestionOption{}
	for rows.Next() {
		var o QuestionOption
		if err := rows.Scan(&o.Key, &o.Content); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *Store) practiceSessionOwner(sessionID int64) (int64, int64, error) {
	var userID, bankID int64
	err := s.DB.QueryRow(
		"SELECT user_id, bank_id FROM practice_sessions WHERE id = ?", sessionID,
	).Scan(&userID, &bankID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrPracticeNotFound
	}
	return userID, bankID, err
}

func (s *Store) AnswerPractice(userID, sessionID, questionID int64, chosen []string) (*PracticeAnswerResult, error) {
	ownerID, _, err := s.practiceSessionOwner(sessionID)
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrPracticeNotFound
	}
	var idsRaw string
	if err := s.DB.QueryRow(
		"SELECT question_ids_json FROM practice_sessions WHERE id = ?", sessionID,
	).Scan(&idsRaw); err != nil {
		return nil, ErrPracticeNotFound
	}
	inSet := false
	var ids []int64
	_ = json.Unmarshal([]byte(idsRaw), &ids)
	for _, id := range ids {
		if id == questionID {
			inSet = true
			break
		}
	}
	if !inSet {
		return nil, ErrQuestionNotInSet
	}

	question, err := s.GetQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("题目不存在")
	}
	chosen = normalizeKeys(chosen)
	isCorrect := keysEqual(chosen, question.Answer)

	now := nowString()
	chosenJSON, _ := json.Marshal(chosen)
	if _, err := s.DB.Exec(`
		INSERT INTO practice_answers(session_id, question_id, chosen_json, is_correct, answered_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(session_id, question_id) DO UPDATE SET
		  chosen_json = excluded.chosen_json,
		  is_correct = excluded.is_correct,
		  answered_at = excluded.answered_at`,
		sessionID, questionID, string(chosenJSON), boolInt(isCorrect), now,
	); err != nil {
		return nil, err
	}
	if _, err := s.DB.Exec(
		"UPDATE practice_sessions SET updated_at = ? WHERE id = ?", now, sessionID,
	); err != nil {
		return nil, err
	}

	result := &PracticeAnswerResult{
		IsCorrect: isCorrect,
		Answer:    question.Answer,
		Analysis:  question.Analysis,
	}
	if isCorrect {
		res, err := s.DB.Exec(`
			UPDATE wrong_book SET correct_streak = correct_streak + 1,
			       mastered = CASE WHEN correct_streak + 1 >= 2 THEN 1 ELSE 0 END,
			       updated_at = ?
			WHERE user_id = ? AND question_id = ?`, now, userID, questionID)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			// 没在错题本里，答对不影响错题本。
			stats, _ := s.WrongStatsOf(userID)
			result.Stats = stats
			return result, nil
		}
	} else {
		if _, err := s.DB.Exec(`
			INSERT INTO wrong_book(user_id, question_id, wrong_count, correct_streak, mastered,
			                       last_wrong_at, created_at, updated_at)
			VALUES(?, ?, 1, 0, 0, ?, ?, ?)
			ON CONFLICT(user_id, question_id) DO UPDATE SET
			  wrong_count = wrong_count + 1,
			  correct_streak = 0,
			  mastered = 0,
			  last_wrong_at = excluded.last_wrong_at,
			  updated_at = excluded.updated_at`,
			userID, questionID, now, now, now,
		); err != nil {
			return nil, err
		}
	}

	var wrongCount, streak int
	var mastered int
	if err := s.DB.QueryRow(
		"SELECT wrong_count, correct_streak, mastered FROM wrong_book WHERE user_id = ? AND question_id = ?",
		userID, questionID,
	).Scan(&wrongCount, &streak, &mastered); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	result.WrongCount = wrongCount
	result.CorrectStreak = streak
	result.Mastered = mastered == 1
	stats, err := s.WrongStatsOf(userID)
	if err != nil {
		return nil, err
	}
	result.Stats = stats
	return result, nil
}

func keysEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Store) DeletePracticeSession(userID, sessionID int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var ownerID int64
	err = tx.QueryRow("SELECT user_id FROM practice_sessions WHERE id = ?", sessionID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPracticeNotFound
	}
	if err != nil {
		return err
	}
	if ownerID != userID {
		return ErrPracticeNotFound
	}
	if _, err := tx.Exec("DELETE FROM practice_answers WHERE session_id = ?", sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM practice_sessions WHERE id = ? AND user_id = ?", sessionID, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListPracticeSessions(userID int64) ([]PracticeSession, error) {
	rows, err := s.DB.Query(`
		SELECT s.id, s.user_id, s.bank_id, COALESCE(b.name, CASE WHEN s.source = 'wrong' THEN '错题本' ELSE '题库已删除' END),
		       s.source, s.total_count, s.order_mode, s.created_at,
		       (SELECT COUNT(*) FROM practice_answers a WHERE a.session_id = s.id),
		       (SELECT COALESCE(SUM(a.is_correct), 0) FROM practice_answers a WHERE a.session_id = s.id)
		FROM practice_sessions s LEFT JOIN banks b ON b.id = s.bank_id
		WHERE s.user_id = ? ORDER BY s.id DESC LIMIT 10`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PracticeSession{}
	for rows.Next() {
		var p PracticeSession
		if err := rows.Scan(&p.ID, &p.UserID, &p.BankID, &p.BankName, &p.Source,
			&p.TotalCount, &p.OrderMode, &p.CreatedAt, &p.Answered, &p.Correct); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) WrongStatsOf(userID int64) (WrongStats, error) {
	var stats WrongStats
	err := s.DB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN mastered = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN mastered = 1 THEN 1 ELSE 0 END), 0),
		       COUNT(*)
		FROM wrong_book WHERE user_id = ?`, userID,
	).Scan(&stats.Pending, &stats.Mastered, &stats.Total)
	return stats, err
}

func (s *Store) ListWrong(userID int64, status string, page, size int) ([]WrongItem, int, WrongStats, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	where := " WHERE w.user_id = ?"
	args := []any{userID}
	switch status {
	case "mastered":
		where += " AND w.mastered = 1"
	case "pending":
		where += " AND w.mastered = 0"
	}
	var total int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM wrong_book w"+where, args...).Scan(&total); err != nil {
		return nil, 0, WrongStats{}, err
	}
	rows, err := s.DB.Query(`
		SELECT w.question_id, w.wrong_count, w.correct_streak, w.mastered, w.last_wrong_at, w.updated_at,
		       q.bank_id, COALESCE(b.name, '题库已删除'), q.type, q.stem, COALESCE(q.stem_image, ''),
		       q.answer, q.analysis, q.difficulty
		FROM wrong_book w
		JOIN questions q ON q.id = w.question_id AND q.is_deleted = 0
		LEFT JOIN banks b ON b.id = q.bank_id
		`+where+` ORDER BY w.updated_at DESC, w.question_id DESC LIMIT ? OFFSET ?`,
		append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, WrongStats{}, err
	}
	items := []WrongItem{}
	for rows.Next() {
		var it WrongItem
		var answer string
		var mastered int
		if err := rows.Scan(&it.QuestionID, &it.WrongCount, &it.CorrectStreak, &mastered,
			&it.LastWrongAt, &it.UpdatedAt, &it.BankID, &it.BankName, &it.Type, &it.Stem,
			&it.StemImage, &answer, &it.Analysis, &it.Difficulty); err != nil {
			rows.Close()
			return nil, 0, WrongStats{}, err
		}
		it.Mastered = mastered == 1
		it.Answer = parseKeys(answer)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, 0, WrongStats{}, err
	}
	rows.Close()
	for i := range items {
		opts, err := s.questionOptions(items[i].QuestionID)
		if err != nil {
			return nil, 0, WrongStats{}, err
		}
		items[i].Options = opts
	}
	stats, err := s.WrongStatsOf(userID)
	if err != nil {
		return nil, 0, WrongStats{}, err
	}
	return items, total, stats, nil
}

func (s *Store) RemoveWrong(userID, questionID int64) error {
	_, err := s.DB.Exec("DELETE FROM wrong_book WHERE user_id = ? AND question_id = ?", userID, questionID)
	return err
}
