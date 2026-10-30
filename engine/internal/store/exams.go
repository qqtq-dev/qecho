package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var (
	ErrExamNotFound    = errors.New("考试链接不存在")
	ErrExamClosed      = errors.New("考试已关闭")
	ErrExamNotOpen     = errors.New("当前不在开考时间段内")
	ErrWrongPassword   = errors.New("考试密码不正确")
	ErrAttemptExpired  = errors.New("答题时间已结束")
	ErrAttemptNotFound = errors.New("答题记录不存在")
)

type Exam struct {
	ID               int64   `json:"id"`
	Title            string  `json:"title"`
	BankID           int64   `json:"bankId"`
	BankName         string  `json:"bankName"`
	TotalCount       int     `json:"totalCount"`
	TimeLimitSec     int     `json:"timeLimitSec"`
	OpenAt           string  `json:"openAt"`
	CloseAt          string  `json:"closeAt"`
	Status           string  `json:"status"`
	Token            string  `json:"token"`
	ParticipantCount int     `json:"participantCount"`
	SubmittedCount   int     `json:"submittedCount"`
	AvgScore         float64 `json:"avgScore"`
	OwnerID          int64   `json:"ownerId"`
	ShuffleQuestions bool    `json:"shuffleQuestions"`
	ShuffleOptions   bool    `json:"shuffleOptions"`
	CreatedAt        string  `json:"createdAt"`
}

type CreateExamInput struct {
	Title            string
	BankID           int64
	Count            int
	TimeLimitSec     int
	OpenAt           time.Time
	CloseAt          time.Time
	Password         string
	ShuffleQuestions bool
	ShuffleOptions   bool
}

type PublicOption struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Content string `json:"content"`
	Image   string `json:"image"`
}

type PublicQuestion struct {
	Seq       int            `json:"seq"`
	Type      string         `json:"type"`
	Stem      string         `json:"stem"`
	StemImage string         `json:"stemImage"`
	Options   []PublicOption `json:"options"`
}

type SavedAnswer struct {
	Seq    int      `json:"seq"`
	Chosen []string `json:"chosen"`
}

type ResultDetail struct {
	Seq          int            `json:"seq"`
	Type         string         `json:"type"`
	Stem         string         `json:"stem"`
	StemImage    string         `json:"stemImage"`
	Options      []PublicOption `json:"options"`
	Chosen       []string       `json:"chosen"`
	Answer       []string       `json:"answer"`
	ChosenLabels []string       `json:"chosenLabels"`
	AnswerLabels []string       `json:"answerLabels"`
	Analysis     string         `json:"analysis"`
	IsCorrect    bool           `json:"isCorrect"`
}

type ExamResult struct {
	Score         float64        `json:"score"`
	CorrectCount  int            `json:"correctCount"`
	AnsweredCount int            `json:"answeredCount"`
	TotalCount    int            `json:"totalCount"`
	SubmittedAt   string         `json:"submittedAt"`
	Details       []ResultDetail `json:"details"`
}

type ExamStart struct {
	State        string           `json:"state"`
	AttemptToken string           `json:"attemptToken,omitempty"`
	Name         string           `json:"name,omitempty"`
	StudentNo    string           `json:"studentNo,omitempty"`
	DeadlineAt   string           `json:"deadlineAt,omitempty"`
	RemainingSec int              `json:"remainingSec"`
	Title        string           `json:"title"`
	TotalCount   int              `json:"totalCount"`
	TimeLimitSec int              `json:"timeLimitSec"`
	Questions    []PublicQuestion `json:"questions,omitempty"`
	Saved        []SavedAnswer    `json:"saved,omitempty"`
	Result       *ExamResult      `json:"result,omitempty"`
}

type AttemptSummary struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	StudentNo     string   `json:"studentNo"`
	Status        string   `json:"status"`
	Score         *float64 `json:"score"`
	StartedAt     string   `json:"startedAt"`
	SubmittedAt   string   `json:"submittedAt"`
	DeadlineAt    string   `json:"deadlineAt"`
	AnsweredCount int      `json:"answeredCount"`
	TotalCount    int      `json:"totalCount"`
}

type questionSnapshot struct {
	ID        int64            `json:"id"`
	Type      string           `json:"type"`
	Stem      string           `json:"stem"`
	StemImage string           `json:"stemImage"`
	Options   []QuestionOption `json:"options"`
	Answer    []string         `json:"answer"`
	Analysis  string           `json:"analysis"`
}

type examRecord struct {
	Exam
	QuestionIDs  []int64
	PasswordSalt string
	PasswordHash string
}

func initialOptionOrder(options []QuestionOption, shuffle bool) []string {
	keys := make([]string, 0, len(options))
	for _, o := range options {
		keys = append(keys, o.Key)
	}
	if shuffle {
		return shuffleKeys(keys)
	}
	return keys
}

func publicOptions(options []QuestionOption, order []string) []PublicOption {
	ordered := orderedOptionKeys(order, options)
	byKey := make(map[string]QuestionOption, len(options))
	for _, o := range options {
		byKey[o.Key] = o
	}
	out := make([]PublicOption, 0, len(ordered))
	for i, key := range ordered {
		opt := byKey[key]
		out = append(out, PublicOption{
			Key: key, Label: optionLabel(i), Content: opt.Content, Image: opt.Image,
		})
	}
	return out
}

func optionOrderOf(raw string, options []QuestionOption) []string {
	var order []string
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &order)
	}
	return orderedOptionKeys(order, options)
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *Store) CreateExam(in CreateExamInput, ownerID int64) (*Exam, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, fmt.Errorf("考试名称不能为空")
	}
	if in.Count < 1 || in.Count > 500 {
		return nil, fmt.Errorf("题量需在 1–500 之间")
	}
	if in.TimeLimitSec < 60 || in.TimeLimitSec > 6*3600 {
		return nil, fmt.Errorf("考试时长需在 1–360 分钟之间")
	}
	if !in.OpenAt.Before(in.CloseAt) {
		return nil, fmt.Errorf("结束时间必须晚于开始时间")
	}
	if in.CloseAt.Before(time.Now()) {
		return nil, fmt.Errorf("结束时间已过")
	}
	if len(in.Password) < 4 {
		return nil, fmt.Errorf("考试密码至少 4 位")
	}
	var bankName string
	if err := s.DB.QueryRow(
		"SELECT name FROM banks WHERE id = ? AND is_deleted = 0", in.BankID,
	).Scan(&bankName); err != nil {
		return nil, fmt.Errorf("题库不存在")
	}

	rows, err := s.DB.Query(`
		SELECT id FROM questions WHERE bank_id = ? AND is_deleted = 0
		ORDER BY RANDOM() LIMIT ?`, in.BankID, in.Count)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if len(ids) == 0 {
		return nil, fmt.Errorf("该题库还没有题目")
	}
	if len(ids) < in.Count {
		return nil, fmt.Errorf("该题库仅有 %d 道题，不足 %d 道", len(ids), in.Count)
	}

	salt, hash, err := HashPassword(in.Password)
	if err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	questionIDs, _ := json.Marshal(ids)
	now := nowString()
	res, err := s.DB.Exec(`
		INSERT INTO exams(title, bank_id, question_ids_json, total_count, time_limit_sec,
		                  open_at, close_at, password_salt, password_hash, token, status,
		                  shuffle_questions, shuffle_options, owner_id, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'published', ?, ?, ?, ?, ?)`,
		in.Title, in.BankID, string(questionIDs), len(ids), in.TimeLimitSec,
		in.OpenAt.UTC().Format(time.RFC3339Nano), in.CloseAt.UTC().Format(time.RFC3339Nano),
		salt, hash, token, boolInt(in.ShuffleQuestions), boolInt(in.ShuffleOptions), ownerID, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Exam{
		ID: id, Title: in.Title, BankID: in.BankID, BankName: bankName,
		TotalCount: len(ids), TimeLimitSec: in.TimeLimitSec,
		OpenAt:  in.OpenAt.UTC().Format(time.RFC3339Nano),
		CloseAt: in.CloseAt.UTC().Format(time.RFC3339Nano),
		Status:  "published", Token: token, OwnerID: ownerID, CreatedAt: now,
		ShuffleQuestions: in.ShuffleQuestions, ShuffleOptions: in.ShuffleOptions,
	}, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *Store) ListExams(viewer *User) ([]Exam, error) {
	query := `
		SELECT e.id, e.title, e.bank_id, COALESCE(b.name,''), e.total_count, e.time_limit_sec,
		       e.open_at, e.close_at, e.status, e.token, e.created_at,
		       (SELECT COUNT(*) FROM exam_attempts a WHERE a.exam_id = e.id),
		       (SELECT COUNT(*) FROM exam_attempts a WHERE a.exam_id = e.id AND a.status = 'submitted'),
		       COALESCE((SELECT AVG(score) FROM exam_attempts a WHERE a.exam_id = e.id AND a.status = 'submitted'), 0),
		       COALESCE(e.owner_id, 0), e.shuffle_questions, e.shuffle_options
		FROM exams e LEFT JOIN banks b ON b.id = e.bank_id`
	args := []any{}
	if viewer.Role != "admin" {
		query += " WHERE e.owner_id = ?"
		args = append(args, viewer.ID)
	}
	query += " ORDER BY e.created_at DESC, e.id DESC"
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Exam{}
	for rows.Next() {
		var e Exam
		var shuffleQ, shuffleO int
		if err := rows.Scan(&e.ID, &e.Title, &e.BankID, &e.BankName, &e.TotalCount, &e.TimeLimitSec,
			&e.OpenAt, &e.CloseAt, &e.Status, &e.Token, &e.CreatedAt,
			&e.ParticipantCount, &e.SubmittedCount, &e.AvgScore, &e.OwnerID,
			&shuffleQ, &shuffleO); err != nil {
			return nil, err
		}
		e.ShuffleQuestions = shuffleQ == 1
		e.ShuffleOptions = shuffleO == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetExam(id int64) (*Exam, error) {
	var e Exam
	var shuffleQ, shuffleO int
	err := s.DB.QueryRow(`
		SELECT e.id, e.title, e.bank_id, COALESCE(b.name,''), e.total_count, e.time_limit_sec,
		       e.open_at, e.close_at, e.status, e.token, e.created_at,
		       (SELECT COUNT(*) FROM exam_attempts a WHERE a.exam_id = e.id),
		       (SELECT COUNT(*) FROM exam_attempts a WHERE a.exam_id = e.id AND a.status = 'submitted'),
		       COALESCE((SELECT AVG(score) FROM exam_attempts a WHERE a.exam_id = e.id AND a.status = 'submitted'), 0),
		       COALESCE(e.owner_id, 0), e.shuffle_questions, e.shuffle_options
		FROM exams e LEFT JOIN banks b ON b.id = e.bank_id WHERE e.id = ?`, id,
	).Scan(&e.ID, &e.Title, &e.BankID, &e.BankName, &e.TotalCount, &e.TimeLimitSec,
		&e.OpenAt, &e.CloseAt, &e.Status, &e.Token, &e.CreatedAt,
		&e.ParticipantCount, &e.SubmittedCount, &e.AvgScore, &e.OwnerID, &shuffleQ, &shuffleO)
	if err != nil {
		return nil, err
	}
	e.ShuffleQuestions = shuffleQ == 1
	e.ShuffleOptions = shuffleO == 1
	return &e, nil
}

func (s *Store) examByToken(token string) (*examRecord, error) {
	var e examRecord
	var idsJSON, salt, hash string
	var shuffleOptions int
	err := s.DB.QueryRow(`
		SELECT id, title, bank_id, total_count, time_limit_sec, open_at, close_at, status,
		       token, created_at, question_ids_json, password_salt, password_hash, shuffle_options
		FROM exams WHERE token = ?`, token,
	).Scan(&e.ID, &e.Title, &e.BankID, &e.TotalCount, &e.TimeLimitSec, &e.OpenAt, &e.CloseAt,
		&e.Status, &e.Token, &e.CreatedAt, &idsJSON, &salt, &hash, &shuffleOptions)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}
	e.PasswordSalt = salt
	e.PasswordHash = hash
	e.ShuffleOptions = shuffleOptions == 1
	_ = json.Unmarshal([]byte(idsJSON), &e.QuestionIDs)
	return &e, nil
}

func (s *Store) DeleteExam(id int64) error {
	_, err := s.DB.Exec("DELETE FROM exams WHERE id = ?", id)
	return err
}

func (s *Store) CloseExam(id int64) error {
	if _, err := s.DB.Exec("UPDATE exams SET status = 'closed', updated_at = ? WHERE id = ?", nowString(), id); err != nil {
		return err
	}
	rows, err := s.DB.Query("SELECT id FROM exam_attempts WHERE exam_id = ? AND status = 'ongoing'", id)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var attemptID int64
		if err := rows.Scan(&attemptID); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, attemptID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, attemptID := range ids {
		if _, err := s.finalizeAttempt(attemptID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) StartExamAttempt(token, name, studentNo, password string) (*ExamStart, error) {
	exam, err := s.examByToken(token)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	openAt, _ := time.Parse(time.RFC3339Nano, exam.OpenAt)
	closeAt, _ := time.Parse(time.RFC3339Nano, exam.CloseAt)
	if !VerifyPassword(exam.PasswordSalt, exam.PasswordHash, password) {
		return nil, ErrWrongPassword
	}
	name = strings.TrimSpace(name)
	studentNo = strings.TrimSpace(studentNo)
	if name == "" || len([]rune(name)) > 30 {
		return nil, fmt.Errorf("请填写姓名（不超过 30 个字）")
	}
	if studentNo == "" || len([]rune(studentNo)) > 40 {
		return nil, fmt.Errorf("请填写学号/工号（不超过 40 个字符）")
	}

	var attemptID int64
	var storedName, storedToken, status, deadline string
	err = s.DB.QueryRow(`
		SELECT id, name, token, status, deadline_at FROM exam_attempts
		WHERE exam_id = ? AND student_no = ?`, exam.ID, studentNo,
	).Scan(&attemptID, &storedName, &storedToken, &status, &deadline)
	if err == nil {
		deadlineAt, _ := time.Parse(time.RFC3339Nano, deadline)
		if status == "submitted" {
			result, err := s.resultForAttempt(attemptID)
			if err != nil {
				return nil, err
			}
			return &ExamStart{
				State: "submitted", Title: exam.Title, TotalCount: exam.TotalCount,
				TimeLimitSec: exam.TimeLimitSec, Name: storedName, StudentNo: studentNo,
				Result: result,
			}, nil
		}
		if now.After(deadlineAt) || exam.Status != "published" {
			result, err := s.finalizeAttempt(attemptID, exam.ID)
			if err != nil {
				return nil, err
			}
			return &ExamStart{
				State: "submitted", Title: exam.Title, TotalCount: exam.TotalCount,
				TimeLimitSec: exam.TimeLimitSec, Name: storedName, StudentNo: studentNo,
				Result: result,
			}, nil
		}
		questions, saved, err := s.loadAttempt(exam.ID, storedToken)
		if err != nil {
			return nil, err
		}
		return &ExamStart{
			State: "ongoing", AttemptToken: storedToken, Name: storedName, StudentNo: studentNo,
			DeadlineAt: deadline, RemainingSec: int(deadlineAt.Sub(now).Seconds()),
			Title: exam.Title, TotalCount: exam.TotalCount, TimeLimitSec: exam.TimeLimitSec,
			Questions: questions, Saved: saved,
		}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exam.Status != "published" {
		return nil, ErrExamClosed
	}
	if now.Before(openAt) || now.After(closeAt) {
		return nil, ErrExamNotOpen
	}

	snapshots := make([]questionSnapshot, 0, len(exam.QuestionIDs))
	for _, qid := range exam.QuestionIDs {
		q, err := s.GetQuestion(qid)
		if err != nil || q.Stem == "" {
			return nil, fmt.Errorf("试卷题目已失效，请联系组织者")
		}
		snapshots = append(snapshots, questionSnapshot{
			ID: q.ID, Type: q.Type, Stem: q.Stem, StemImage: q.StemImage,
			Options: q.Options, Answer: q.Answer, Analysis: q.Analysis,
		})
	}
	deadlineAt := now.Add(time.Duration(exam.TimeLimitSec) * time.Second)
	if deadlineAt.After(closeAt) {
		deadlineAt = closeAt
	}
	attemptToken, err := randomToken()
	if err != nil {
		return nil, err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		INSERT INTO exam_attempts(exam_id, name, student_no, token, status, started_at, deadline_at, total_count)
		VALUES(?, ?, ?, ?, 'ongoing', ?, ?, ?)`,
		exam.ID, name, studentNo, attemptToken, now.Format(time.RFC3339Nano),
		deadlineAt.Format(time.RFC3339Nano), len(snapshots),
	)
	if err != nil {
		return nil, err
	}
	attemptID, _ = res.LastInsertId()
	questions := make([]PublicQuestion, 0, len(snapshots))
	for i, snap := range snapshots {
		raw, _ := json.Marshal(snap)
		order := initialOptionOrder(snap.Options, exam.ShuffleOptions)
		orderJSON, _ := json.Marshal(order)
		if _, err := tx.Exec(`
			INSERT INTO exam_answers(attempt_id, seq, question_id, snapshot_json, option_order)
			VALUES(?, ?, ?, ?, ?)`, attemptID, i+1, snap.ID, string(raw), string(orderJSON)); err != nil {
			return nil, err
		}
		questions = append(questions, PublicQuestion{
			Seq: i + 1, Type: snap.Type, Stem: snap.Stem, StemImage: snap.StemImage,
			Options: publicOptions(snap.Options, order),
		})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ExamStart{
		State: "ongoing", AttemptToken: attemptToken, Name: name, StudentNo: studentNo,
		DeadlineAt:   deadlineAt.Format(time.RFC3339Nano),
		RemainingSec: int(deadlineAt.Sub(now).Seconds()),
		Title:        exam.Title, TotalCount: exam.TotalCount, TimeLimitSec: exam.TimeLimitSec,
		Questions: questions, Saved: []SavedAnswer{},
	}, nil
}

func (s *Store) loadAttempt(examID int64, attemptToken string) ([]PublicQuestion, []SavedAnswer, error) {
	var attemptID int64
	err := s.DB.QueryRow(
		"SELECT id FROM exam_attempts WHERE exam_id = ? AND token = ?", examID, attemptToken,
	).Scan(&attemptID)
	if err != nil {
		return nil, nil, ErrAttemptNotFound
	}
	rows, err := s.DB.Query(`
		SELECT seq, snapshot_json, COALESCE(chosen,''), COALESCE(option_order,'') FROM exam_answers
		WHERE attempt_id = ? ORDER BY seq`, attemptID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	questions := []PublicQuestion{}
	saved := []SavedAnswer{}
	for rows.Next() {
		var seq int
		var raw, chosenRaw, orderRaw string
		if err := rows.Scan(&seq, &raw, &chosenRaw, &orderRaw); err != nil {
			return nil, nil, err
		}
		var snap questionSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			return nil, nil, err
		}
		order := optionOrderOf(orderRaw, snap.Options)
		questions = append(questions, PublicQuestion{
			Seq: seq, Type: snap.Type, Stem: snap.Stem, StemImage: snap.StemImage,
			Options: publicOptions(snap.Options, order),
		})
		if chosenRaw != "" {
			saved = append(saved, SavedAnswer{Seq: seq, Chosen: parseKeys(chosenRaw)})
		}
	}
	return questions, saved, rows.Err()
}

func (s *Store) SaveExamAnswer(examToken, attemptToken string, seq int, chosen []string) error {
	var attemptID, examID int64
	var status, deadline string
	err := s.DB.QueryRow(`
		SELECT a.id, a.exam_id, a.status, a.deadline_at
		FROM exam_attempts a JOIN exams e ON e.id = a.exam_id
		WHERE e.token = ? AND a.token = ?`, examToken, attemptToken,
	).Scan(&attemptID, &examID, &status, &deadline)
	if err != nil {
		return ErrAttemptNotFound
	}
	if status != "ongoing" {
		return ErrAttemptExpired
	}
	deadlineAt, _ := time.Parse(time.RFC3339Nano, deadline)
	if time.Now().UTC().After(deadlineAt) {
		_, _ = s.finalizeAttempt(attemptID, examID)
		return ErrAttemptExpired
	}
	var raw string
	if err := s.DB.QueryRow(
		"SELECT snapshot_json FROM exam_answers WHERE attempt_id = ? AND seq = ?", attemptID, seq,
	).Scan(&raw); err != nil {
		return ErrAttemptNotFound
	}
	var snap questionSnapshot
	if err := json.Unmarshal([]byte(raw), &snap); err != nil {
		return err
	}
	valid := map[string]bool{}
	for _, o := range snap.Options {
		valid[o.Key] = true
	}
	keys := normalizeKeys(chosen)
	for _, k := range keys {
		if !valid[k] {
			return fmt.Errorf("答案选项无效")
		}
	}
	if snap.Type == "single" && len(keys) > 1 {
		return fmt.Errorf("单选题只能选择一个答案")
	}
	var chosenValue any
	if len(keys) > 0 {
		raw, _ := json.Marshal(keys)
		chosenValue = string(raw)
	}
	if _, err := s.DB.Exec(
		"UPDATE exam_answers SET chosen = ? WHERE attempt_id = ? AND seq = ?", chosenValue, attemptID, seq,
	); err != nil {
		return err
	}
	var answered int
	_ = s.DB.QueryRow(
		"SELECT COUNT(*) FROM exam_answers WHERE attempt_id = ? AND chosen IS NOT NULL", attemptID,
	).Scan(&answered)
	_, err = s.DB.Exec("UPDATE exam_attempts SET answered_count = ? WHERE id = ?", answered, attemptID)
	return err
}

func (s *Store) SubmitExamAttempt(examToken, attemptToken string) (*ExamResult, error) {
	var attemptID, examID int64
	var status string
	err := s.DB.QueryRow(`
		SELECT a.id, a.exam_id, a.status
		FROM exam_attempts a JOIN exams e ON e.id = a.exam_id
		WHERE e.token = ? AND a.token = ?`, examToken, attemptToken,
	).Scan(&attemptID, &examID, &status)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	if status == "submitted" {
		return s.resultForAttempt(attemptID)
	}
	return s.finalizeAttempt(attemptID, examID)
}

func (s *Store) finalizeAttempt(attemptID, examID int64) (*ExamResult, error) {
	rows, err := s.DB.Query(`
		SELECT seq, snapshot_json, COALESCE(chosen,''), COALESCE(option_order,'') FROM exam_answers
		WHERE attempt_id = ? ORDER BY seq`, attemptID)
	if err != nil {
		return nil, err
	}
	details := []ResultDetail{}
	correct, answered := 0, 0
	for rows.Next() {
		var seq int
		var raw, chosenRaw, orderRaw string
		if err := rows.Scan(&seq, &raw, &chosenRaw, &orderRaw); err != nil {
			rows.Close()
			return nil, err
		}
		var snap questionSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			rows.Close()
			return nil, err
		}
		order := optionOrderOf(orderRaw, snap.Options)
		chosen := parseKeys(chosenRaw)
		isCorrect := len(chosen) > 0 && equalKeys(chosen, snap.Answer)
		if len(chosen) > 0 {
			answered++
		}
		if isCorrect {
			correct++
		}
		details = append(details, ResultDetail{
			Seq: seq, Type: snap.Type, Stem: snap.Stem, StemImage: snap.StemImage,
			Options: publicOptions(snap.Options, order), Chosen: chosen, Answer: snap.Answer,
			ChosenLabels: displayLabels(order, chosen), AnswerLabels: displayLabels(order, snap.Answer),
			Analysis: snap.Analysis, IsCorrect: isCorrect,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	total := len(details)
	score := 0.0
	if total > 0 {
		score = math.Round(float64(correct)/float64(total)*1000) / 10
	}
	now := nowString()
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, d := range details {
		val := 0
		if d.IsCorrect {
			val = 1
		}
		if _, err := tx.Exec(
			"UPDATE exam_answers SET is_correct = ? WHERE attempt_id = ? AND seq = ?", val, attemptID, d.Seq,
		); err != nil {
			return nil, err
		}
	}
	res, err := tx.Exec(`
		UPDATE exam_attempts SET status = 'submitted', submitted_at = ?, score = ?,
		       correct_count = ?, answered_count = ?, total_count = ?
		WHERE id = ? AND status = 'ongoing'`,
		now, score, correct, answered, total, attemptID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// 并发交卷，直接返回已落库结果。
		tx.Rollback()
		var existing int
		if err := s.DB.QueryRow("SELECT COUNT(*) FROM exam_attempts WHERE id = ? AND exam_id = ?", attemptID, examID).Scan(&existing); err != nil || existing == 0 {
			return nil, ErrAttemptNotFound
		}
		return s.resultForAttempt(attemptID)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ExamResult{
		Score: score, CorrectCount: correct, AnsweredCount: answered, TotalCount: total,
		SubmittedAt: now, Details: details,
	}, nil
}

func equalKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, k := range a {
		set[k] = true
	}
	for _, k := range b {
		if !set[k] {
			return false
		}
	}
	return true
}

func (s *Store) resultForAttempt(attemptID int64) (*ExamResult, error) {
	var score float64
	var correct, answered, total int
	var submittedAt sql.NullString
	err := s.DB.QueryRow(`
		SELECT COALESCE(score,0), correct_count, answered_count, total_count, submitted_at
		FROM exam_attempts WHERE id = ?`, attemptID,
	).Scan(&score, &correct, &answered, &total, &submittedAt)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	rows, err := s.DB.Query(`
		SELECT seq, snapshot_json, COALESCE(chosen,''), COALESCE(is_correct,0), COALESCE(option_order,'')
		FROM exam_answers WHERE attempt_id = ? ORDER BY seq`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	details := []ResultDetail{}
	for rows.Next() {
		var seq, isCorrect int
		var raw, chosenRaw, orderRaw string
		if err := rows.Scan(&seq, &raw, &chosenRaw, &isCorrect, &orderRaw); err != nil {
			return nil, err
		}
		var snap questionSnapshot
		if err := json.Unmarshal([]byte(raw), &snap); err != nil {
			return nil, err
		}
		order := optionOrderOf(orderRaw, snap.Options)
		chosen := parseKeys(chosenRaw)
		details = append(details, ResultDetail{
			Seq: seq, Type: snap.Type, Stem: snap.Stem, StemImage: snap.StemImage,
			Options: publicOptions(snap.Options, order), Chosen: chosen, Answer: snap.Answer,
			ChosenLabels: displayLabels(order, chosen), AnswerLabels: displayLabels(order, snap.Answer),
			Analysis: snap.Analysis, IsCorrect: isCorrect == 1,
		})
	}
	return &ExamResult{
		Score: score, CorrectCount: correct, AnsweredCount: answered, TotalCount: total,
		SubmittedAt: submittedAt.String, Details: details,
	}, rows.Err()
}

func (s *Store) ListExamAttempts(examID int64) ([]AttemptSummary, error) {
	rows, err := s.DB.Query(`
		SELECT id, name, student_no, status, score, started_at, COALESCE(submitted_at,''),
		       deadline_at, answered_count, total_count
		FROM exam_attempts WHERE exam_id = ? ORDER BY status ASC, score DESC, id ASC`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AttemptSummary{}
	for rows.Next() {
		var a AttemptSummary
		var score sql.NullFloat64
		if err := rows.Scan(&a.ID, &a.Name, &a.StudentNo, &a.Status, &score, &a.StartedAt,
			&a.SubmittedAt, &a.DeadlineAt, &a.AnsweredCount, &a.TotalCount); err != nil {
			return nil, err
		}
		if score.Valid {
			v := score.Float64
			a.Score = &v
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *Store) SetSetting(key, value string) error {
	_, err := s.DB.Exec(
		"INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

type PublicExam struct {
	Title         string `json:"title"`
	TotalCount    int    `json:"totalCount"`
	TimeLimitSec  int    `json:"timeLimitSec"`
	OpenAt        string `json:"openAt"`
	CloseAt       string `json:"closeAt"`
	Status        string `json:"status"`
	NeedsPassword bool   `json:"needsPassword"`
}

func (s *Store) PublicExamInfo(token string) (*PublicExam, error) {
	var e PublicExam
	var storedStatus string
	var hasPassword int
	err := s.DB.QueryRow(`
		SELECT title, total_count, time_limit_sec, open_at, close_at, status,
		       CASE WHEN password_hash <> '' THEN 1 ELSE 0 END
		FROM exams WHERE token = ?`, token,
	).Scan(&e.Title, &e.TotalCount, &e.TimeLimitSec, &e.OpenAt, &e.CloseAt, &storedStatus, &hasPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExamNotFound
		}
		return nil, err
	}
	e.NeedsPassword = hasPassword == 1
	openAt, errOpen := time.Parse(time.RFC3339Nano, e.OpenAt)
	closeAt, errClose := time.Parse(time.RFC3339Nano, e.CloseAt)
	now := time.Now().UTC()
	switch {
	case storedStatus != "published":
		e.Status = "closed"
	case errOpen == nil && now.Before(openAt):
		e.Status = "not_open"
	case errClose == nil && now.After(closeAt):
		e.Status = "closed"
	default:
		e.Status = "open"
	}
	return &e, nil
}

func (s *Store) AttemptResult(examToken, attemptToken string) (*ExamResult, error) {
	var attemptID int64
	var status string
	err := s.DB.QueryRow(`
		SELECT a.id, a.status FROM exam_attempts a JOIN exams e ON e.id = a.exam_id
		WHERE e.token = ? AND a.token = ?`, examToken, attemptToken,
	).Scan(&attemptID, &status)
	if err != nil || status != "submitted" {
		return nil, ErrAttemptNotFound
	}
	return s.resultForAttempt(attemptID)
}

func (s *Store) ResumeExamAttempt(examToken, attemptToken string) (*ExamStart, error) {
	var attemptID, examID int64
	var status, deadline, name, studentNo, title string
	var totalCount, timeLimit int
	err := s.DB.QueryRow(`
		SELECT a.id, a.exam_id, a.status, a.deadline_at, a.name, a.student_no,
		       e.title, e.total_count, e.time_limit_sec
		FROM exam_attempts a JOIN exams e ON e.id = a.exam_id
		WHERE e.token = ? AND a.token = ?`, examToken, attemptToken,
	).Scan(&attemptID, &examID, &status, &deadline, &name, &studentNo, &title, &totalCount, &timeLimit)
	if err != nil {
		return nil, ErrAttemptNotFound
	}
	now := time.Now().UTC()
	if status == "submitted" {
		result, err := s.resultForAttempt(attemptID)
		if err != nil {
			return nil, err
		}
		return &ExamStart{
			State: "submitted", Title: title, TotalCount: totalCount, TimeLimitSec: timeLimit,
			Name: name, StudentNo: studentNo, Result: result,
		}, nil
	}
	deadlineAt, _ := time.Parse(time.RFC3339Nano, deadline)
	if now.After(deadlineAt) {
		result, err := s.finalizeAttempt(attemptID, examID)
		if err != nil {
			return nil, err
		}
		return &ExamStart{
			State: "submitted", Title: title, TotalCount: totalCount, TimeLimitSec: timeLimit,
			Name: name, StudentNo: studentNo, Result: result,
		}, nil
	}
	questions, saved, err := s.loadAttempt(examID, attemptToken)
	if err != nil {
		return nil, err
	}
	return &ExamStart{
		State: "ongoing", AttemptToken: attemptToken, Name: name, StudentNo: studentNo,
		DeadlineAt: deadline, RemainingSec: int(deadlineAt.Sub(now).Seconds()),
		Title: title, TotalCount: totalCount, TimeLimitSec: timeLimit,
		Questions: questions, Saved: saved,
	}, nil
}
