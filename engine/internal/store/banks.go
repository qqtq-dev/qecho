package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Bank struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	QuestionCount int     `json:"questionCount"`
	AnsweredCount int     `json:"answeredCount"`
	CorrectRate   float64 `json:"correctRate"`
	OwnerID       *int64  `json:"ownerId"`
	OwnerName     string  `json:"ownerName"`
	IsPublic      bool    `json:"isPublic"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
}

const bankColumns = `b.id, b.name, b.description,
	(SELECT COUNT(*) FROM questions q WHERE q.bank_id = b.id AND q.is_deleted = 0),
	(SELECT COUNT(*) FROM practice_answers pa
	   JOIN practice_sessions ps ON ps.id = pa.session_id
	   JOIN questions q ON q.id = pa.question_id
	  WHERE q.bank_id = b.id AND ps.status = 'finished'),
	(SELECT COALESCE(SUM(pa.is_correct), 0) FROM practice_answers pa
	   JOIN practice_sessions ps ON ps.id = pa.session_id
	   JOIN questions q ON q.id = pa.question_id
	  WHERE q.bank_id = b.id AND ps.status = 'finished'),
	b.owner_id, COALESCE(u.username,''), b.created_at, b.updated_at`

func scanBank(row interface{ Scan(...any) error }) (*Bank, error) {
	var b Bank
	var owner sql.NullInt64
	var correct int
	if err := row.Scan(&b.ID, &b.Name, &b.Description, &b.QuestionCount,
		&b.AnsweredCount, &correct, &owner, &b.OwnerName, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	if b.AnsweredCount > 0 {
		b.CorrectRate = float64(correct) / float64(b.AnsweredCount)
	}
	if owner.Valid {
		id := owner.Int64
		b.OwnerID = &id
	}
	b.IsPublic = !owner.Valid
	return &b, nil
}

type QuestionOption struct {
	Key     string `json:"key"`
	Content string `json:"content"`
	Image   string `json:"image"`
}

type Question struct {
	ID         int64            `json:"id"`
	BankID     int64            `json:"bankId"`
	Type       string           `json:"type"`
	Stem       string           `json:"stem"`
	StemImage  string           `json:"stemImage"`
	Answer     []string         `json:"answer"`
	Analysis   string           `json:"analysis"`
	Difficulty int              `json:"difficulty"`
	Tags       []string         `json:"tags"`
	Options    []QuestionOption `json:"options"`
	CreatedAt  string           `json:"createdAt"`
	UpdatedAt  string           `json:"updatedAt"`
}

type QuestionInput struct {
	Type       string           `json:"type"`
	Stem       string           `json:"stem"`
	StemImage  string           `json:"stemImage"`
	Answer     []string         `json:"answer"`
	Analysis   string           `json:"analysis"`
	Difficulty int              `json:"difficulty"`
	Tags       []string         `json:"tags"`
	Options    []QuestionOption `json:"options"`
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func parseKeys(raw string) []string {
	var keys []string
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &keys)
	}
	if keys == nil {
		keys = []string{}
	}
	return keys
}

func normalizeKeys(keys []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.ToUpper(strings.TrimSpace(k))
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Store) ListBanks(viewer *User) ([]Bank, error) {
	query := `SELECT ` + bankColumns + `
		FROM banks b LEFT JOIN users u ON u.id = b.owner_id
		WHERE b.is_deleted = 0`
	args := []any{}
	if viewer.Role != "admin" {
		query += " AND (b.owner_id IS NULL OR b.owner_id = ?)"
		args = append(args, viewer.ID)
	}
	query += " ORDER BY b.updated_at DESC, b.id DESC"
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Bank{}
	for rows.Next() {
		b, err := scanBank(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *b)
	}
	return out, rows.Err()
}

func (s *Store) CreateBank(name, description string, ownerID *int64) (*Bank, error) {
	now := nowString()
	res, err := s.DB.Exec(
		"INSERT INTO banks(name, description, owner_id, created_at, updated_at) VALUES(?, ?, ?, ?, ?)",
		name, description, ownerID, now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	b := &Bank{ID: id, Name: name, Description: description, CreatedAt: now, UpdatedAt: now,
		OwnerID: ownerID, IsPublic: ownerID == nil}
	return b, nil
}

func (s *Store) UpdateBank(id int64, name, description string) error {
	_, err := s.DB.Exec(
		"UPDATE banks SET name = ?, description = ?, updated_at = ? WHERE id = ? AND is_deleted = 0",
		name, description, nowString(), id,
	)
	return err
}

func (s *Store) DeleteBank(id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowString()
	if _, err := tx.Exec("UPDATE banks SET is_deleted = 1, updated_at = ? WHERE id = ?", now, id); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE questions SET is_deleted = 1, updated_at = ? WHERE bank_id = ?", now, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetBank(id int64) (*Bank, error) {
	row := s.DB.QueryRow(`SELECT `+bankColumns+`
		FROM banks b LEFT JOIN users u ON u.id = b.owner_id
		WHERE b.id = ? AND b.is_deleted = 0`, id)
	return scanBank(row)
}

type QuestionFilter struct {
	Page       int
	Size       int
	Keyword    string
	Type       string
	Tag        string
	Difficulty int
}

func (s *Store) ListQuestions(bankID int64, filter QuestionFilter) ([]Question, int, error) {
	page, size := filter.Page, filter.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 500 {
		size = 50
	}
	where := []string{"q.bank_id = ?", "q.is_deleted = 0"}
	args := []any{bankID}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, "(q.stem LIKE ? OR q.analysis LIKE ?)")
		args = append(args, like, like)
	}
	if filter.Type == "single" || filter.Type == "multiple" {
		where = append(where, "q.type = ?")
		args = append(args, filter.Type)
	}
	if filter.Difficulty >= 1 && filter.Difficulty <= 5 {
		where = append(where, "q.difficulty = ?")
		args = append(args, filter.Difficulty)
	}
	if tag := strings.TrimSpace(filter.Tag); tag != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM question_tags qt JOIN tags t ON t.id = qt.tag_id
			WHERE qt.question_id = q.id AND t.name = ?)`)
		args = append(args, tag)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM questions q WHERE "+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.Query(`
		SELECT q.id, q.bank_id, q.type, q.stem, COALESCE(q.stem_image,''), q.answer, q.analysis,
		       q.difficulty, q.created_at, q.updated_at
		FROM questions q WHERE `+whereSQL+`
		ORDER BY q.sort_order, q.id LIMIT ? OFFSET ?`,
		append(args, size, (page-1)*size)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := []Question{}
	for rows.Next() {
		var q Question
		var answer string
		if err := rows.Scan(&q.ID, &q.BankID, &q.Type, &q.Stem, &q.StemImage, &answer,
			&q.Analysis, &q.Difficulty, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, 0, err
		}
		q.Answer = parseKeys(answer)
		list = append(list, q)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	for i := range list {
		if err := s.loadOptions(&list[i]); err != nil {
			return nil, 0, err
		}
		if err := s.loadQuestionTags(&list[i]); err != nil {
			return nil, 0, err
		}
	}
	return list, total, nil
}

// ExportQuestions 按题库顺序返回全部有效题目，供导出使用。
func (s *Store) ExportQuestions(bankID int64) ([]Question, error) {
	out := []Question{}
	for page := 1; ; page++ {
		list, total, err := s.ListQuestions(bankID, QuestionFilter{Page: page, Size: 500})
		if err != nil {
			return nil, err
		}
		out = append(out, list...)
		if len(list) == 0 || len(out) >= total {
			return out, nil
		}
	}
}

func (s *Store) GetQuestion(id int64) (*Question, error) {
	var q Question
	var answer string
	err := s.DB.QueryRow(`
		SELECT id, bank_id, type, stem, COALESCE(stem_image,''), answer, analysis,
		       difficulty, created_at, updated_at
		FROM questions WHERE id = ? AND is_deleted = 0`, id,
	).Scan(&q.ID, &q.BankID, &q.Type, &q.Stem, &q.StemImage, &answer,
		&q.Analysis, &q.Difficulty, &q.CreatedAt, &q.UpdatedAt)
	if err != nil {
		return nil, err
	}
	q.Answer = parseKeys(answer)
	if err := s.loadOptions(&q); err != nil {
		return nil, err
	}
	if err := s.loadQuestionTags(&q); err != nil {
		return nil, err
	}
	return &q, nil
}

func (s *Store) loadOptions(q *Question) error {
	rows, err := s.DB.Query(
		"SELECT key, content, COALESCE(image,'') FROM question_options WHERE question_id = ? ORDER BY sort_order, key",
		q.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	q.Options = []QuestionOption{}
	for rows.Next() {
		var o QuestionOption
		if err := rows.Scan(&o.Key, &o.Content, &o.Image); err != nil {
			return err
		}
		q.Options = append(q.Options, o)
	}
	return rows.Err()
}

func validateQuestionInput(in *QuestionInput) error {
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))
	in.Stem = strings.TrimSpace(in.Stem)
	if in.Type != "single" && in.Type != "multiple" {
		return fmt.Errorf("题型只能是 single 或 multiple")
	}
	if in.Stem == "" {
		return fmt.Errorf("题干不能为空")
	}
	if len(in.Options) < 2 || len(in.Options) > 8 {
		return fmt.Errorf("选项数量需在 2–8 之间")
	}
	seen := map[string]bool{}
	for i := range in.Options {
		in.Options[i].Key = strings.ToUpper(strings.TrimSpace(in.Options[i].Key))
		in.Options[i].Content = strings.TrimSpace(in.Options[i].Content)
		image, err := NormalizeImageRef(in.Options[i].Image)
		if err != nil {
			return err
		}
		in.Options[i].Image = image
		key := in.Options[i].Key
		if len(key) != 1 || key < "A" || key > "H" {
			return fmt.Errorf("选项键必须是 A–H")
		}
		if seen[key] {
			return fmt.Errorf("选项键 %s 重复", key)
		}
		if in.Options[i].Content == "" {
			return fmt.Errorf("选项 %s 内容不能为空", key)
		}
		seen[key] = true
	}
	in.Answer = normalizeKeys(in.Answer)
	if len(in.Answer) == 0 {
		return fmt.Errorf("请设置正确答案")
	}
	for _, k := range in.Answer {
		if !seen[k] {
			return fmt.Errorf("答案 %s 不在选项中", k)
		}
	}
	if in.Type == "single" && len(in.Answer) != 1 {
		return fmt.Errorf("单选题只能有一个答案")
	}
	if in.Type == "multiple" && len(in.Answer) < 2 {
		return fmt.Errorf("多选题至少有两个答案")
	}
	if in.Difficulty < 1 || in.Difficulty > 5 {
		in.Difficulty = 2
	}
	in.Tags = normalizeTags(in.Tags)
	in.Analysis = strings.TrimSpace(in.Analysis)
	image, err := NormalizeImageRef(in.StemImage)
	if err != nil {
		return err
	}
	in.StemImage = image
	return nil
}

// ValidateQuestion 供导入器复用同一套校验与归一化。
func ValidateQuestion(in *QuestionInput) error {
	return validateQuestionInput(in)
}

// HashQuestion 返回与入库一致的查重哈希。
func HashQuestion(in *QuestionInput) string {
	return questionHash(in)
}

func questionHash(in *QuestionInput) string {
	parts := []string{in.Type, in.Stem}
	opts := append([]QuestionOption(nil), in.Options...)
	sort.Slice(opts, func(i, j int) bool { return opts[i].Key < opts[j].Key })
	for _, o := range opts {
		parts = append(parts, o.Key, o.Content)
	}
	parts = append(parts, strings.Join(in.Answer, ","))
	return hashContent(parts...)
}

func (s *Store) CreateQuestion(bankID int64, in QuestionInput) (*Question, error) {
	if err := validateQuestionInput(&in); err != nil {
		return nil, err
	}
	var bankExists int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM banks WHERE id = ? AND is_deleted = 0", bankID).Scan(&bankExists); err != nil {
		return nil, err
	}
	if bankExists == 0 {
		return nil, fmt.Errorf("题库不存在")
	}
	now := nowString()
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var sortOrder int
	_ = tx.QueryRow("SELECT COALESCE(MAX(sort_order),0)+1 FROM questions WHERE bank_id = ?", bankID).Scan(&sortOrder)
	answer, _ := json.Marshal(in.Answer)
	res, err := tx.Exec(`
		INSERT INTO questions(bank_id, type, stem, stem_image, answer, analysis, difficulty,
		                      sort_order, content_hash, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bankID, in.Type, in.Stem, nullable(in.StemImage), string(answer), in.Analysis,
		in.Difficulty, sortOrder, questionHash(&in), now, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	for i, o := range in.Options {
		if _, err := tx.Exec(
			"INSERT INTO question_options(question_id, key, content, image, sort_order) VALUES(?, ?, ?, ?, ?)",
			id, o.Key, o.Content, nullable(o.Image), i,
		); err != nil {
			return nil, err
		}
	}
	if err := s.setQuestionTags(tx, id, in.Tags); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetQuestion(id)
}

func (s *Store) UpdateQuestion(id int64, in QuestionInput) (*Question, error) {
	if err := validateQuestionInput(&in); err != nil {
		return nil, err
	}
	now := nowString()
	answer, _ := json.Marshal(in.Answer)
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`
		UPDATE questions SET type = ?, stem = ?, stem_image = ?, answer = ?, analysis = ?,
		       difficulty = ?, content_hash = ?, updated_at = ?
		WHERE id = ? AND is_deleted = 0`,
		in.Type, in.Stem, nullable(in.StemImage), string(answer), in.Analysis,
		in.Difficulty, questionHash(&in), now, id,
	)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.Exec("DELETE FROM question_options WHERE question_id = ?", id); err != nil {
		return nil, err
	}
	for i, o := range in.Options {
		if _, err := tx.Exec(
			"INSERT INTO question_options(question_id, key, content, image, sort_order) VALUES(?, ?, ?, ?, ?)",
			id, o.Key, o.Content, nullable(o.Image), i,
		); err != nil {
			return nil, err
		}
	}
	if err := s.setQuestionTags(tx, id, in.Tags); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.deleteOrphanTags()
	return s.GetQuestion(id)
}

func (s *Store) DeleteQuestion(id int64) error {
	_, err := s.DB.Exec(
		"UPDATE questions SET is_deleted = 1, updated_at = ? WHERE id = ?", nowString(), id,
	)
	if err == nil {
		s.deleteOrphanTags()
	}
	return err
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}
