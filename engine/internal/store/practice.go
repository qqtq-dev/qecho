package store

import (
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"strconv"
	"strings"
)

var (
	ErrPracticeNotFound = errors.New("练习不存在")
	ErrQuestionNotInSet = errors.New("题目不在本次练习中")
)

const (
	practiceMaxQuestions = 5000
	maxAnswerTimeMs      = 30 * 60 * 1000
)

type PracticeSession struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"userId"`
	BankID         int64  `json:"bankId"`
	BankName       string `json:"bankName"`
	Source         string `json:"source"`
	TotalCount     int    `json:"totalCount"`
	OrderMode      string `json:"order"`
	Answered       int    `json:"answeredCount"`
	Correct        int    `json:"correctCount"`
	Status         string `json:"status"`
	LastSeq        int    `json:"lastSeq"`
	DurationSec    int    `json:"durationSec"`
	ShuffleOptions bool   `json:"shuffleOptions"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type PracticeInput struct {
	BankIDs        []int64
	QuestionIDs    []int64
	Count          int
	Order          string
	Source         string
	QuestionType   string
	Difficulty     int
	ShuffleOptions bool
	Admin          bool
}

type PracticeOption struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Content string `json:"content"`
	Image   string `json:"image"`
}

type PracticeItem struct {
	ID         int64            `json:"id"`
	Type       string           `json:"type"`
	Stem       string           `json:"stem"`
	StemImage  string           `json:"stemImage"`
	Difficulty int              `json:"difficulty"`
	Options    []PracticeOption `json:"options"`
}

type PracticeAnswerResult struct {
	IsCorrect     bool       `json:"isCorrect"`
	Answer        []string   `json:"answer"`
	Analysis      string     `json:"analysis"`
	WrongCount    int        `json:"wrongCount"`
	CorrectStreak int        `json:"correctStreak"`
	Mastered      bool       `json:"mastered"`
	TimeSpentMs   int        `json:"timeSpentMs"`
	DurationSec   int        `json:"durationSec"`
	Stats         WrongStats `json:"stats"`
}

type PracticeAnswerState struct {
	QuestionID    int64    `json:"questionId"`
	Chosen        []string `json:"chosen"`
	IsCorrect     bool     `json:"isCorrect"`
	Answer        []string `json:"answer"`
	Analysis      string   `json:"analysis"`
	WrongCount    int      `json:"wrongCount"`
	CorrectStreak int      `json:"correctStreak"`
	Mastered      bool     `json:"mastered"`
	TimeSpentMs   int      `json:"timeSpentMs"`
	AnsweredAt    string   `json:"answeredAt"`
}

type PracticeDetail struct {
	Session   *PracticeSession      `json:"session"`
	Questions []PracticeItem        `json:"questions"`
	Answers   []PracticeAnswerState `json:"answers"`
	Marked    []int64               `json:"marked"`
	LastSeq   int                   `json:"lastSeq"`
	Stats     WrongStats            `json:"stats"`
}

type PracticeSummary struct {
	SessionID     int64  `json:"sessionId"`
	TotalCount    int    `json:"totalCount"`
	AnsweredCount int    `json:"answeredCount"`
	CorrectCount  int    `json:"correctCount"`
	DurationSec   int    `json:"durationSec"`
	Status        string `json:"status"`
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

type practiceSessionRow struct {
	PracticeSession
	idsRaw          string
	markedRaw       string
	optionOrdersRaw string
}

const practiceSessionSelect = `
	SELECT s.id, s.user_id, s.bank_id,
	       COALESCE(NULLIF(s.bank_name_snapshot, ''), b.name,
	                CASE WHEN s.source = 'wrong' THEN '错题本' ELSE '题库已删除' END),
	       s.source, s.total_count, s.order_mode, s.created_at, s.updated_at,
	       (SELECT COUNT(*) FROM practice_answers a WHERE a.session_id = s.id),
	       (SELECT COALESCE(SUM(a.is_correct), 0) FROM practice_answers a WHERE a.session_id = s.id),
	       s.status, s.last_seq, s.duration_sec, s.shuffle_options,
	       s.question_ids_json, s.marked_json, s.option_orders_json
	FROM practice_sessions s LEFT JOIN banks b ON b.id = s.bank_id`

func scanPracticeSession(row interface{ Scan(...any) error }) (*practiceSessionRow, error) {
	var r practiceSessionRow
	var shuffle int
	if err := row.Scan(&r.ID, &r.UserID, &r.BankID, &r.BankName, &r.Source, &r.TotalCount,
		&r.OrderMode, &r.CreatedAt, &r.UpdatedAt, &r.Answered, &r.Correct,
		&r.Status, &r.LastSeq, &r.DurationSec, &shuffle,
		&r.idsRaw, &r.markedRaw, &r.optionOrdersRaw); err != nil {
		return nil, err
	}
	r.ShuffleOptions = shuffle == 1
	return &r, nil
}

func (s *Store) loadPracticeRow(sessionID int64) (*practiceSessionRow, error) {
	row, err := scanPracticeSession(s.DB.QueryRow(practiceSessionSelect+` WHERE s.id = ?`, sessionID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPracticeNotFound
	}
	return row, err
}

func (s *Store) CreatePracticeSession(userID int64, in PracticeInput) (*PracticeSession, []PracticeItem, bool, error) {
	in.BankIDs = normalizeIDList(in.BankIDs)
	in.QuestionIDs = normalizeIDList(in.QuestionIDs)
	if in.Order != "fixed" {
		in.Order = "random"
	}
	if in.Source != "wrong" {
		in.Source = "bank"
	}
	if in.QuestionType != "single" && in.QuestionType != "multiple" {
		in.QuestionType = ""
	}
	if in.Difficulty < 0 || in.Difficulty > 5 {
		in.Difficulty = 0
	}
	if in.Count < 0 {
		in.Count = 0
	}
	limit := in.Count
	if limit == 0 || limit > practiceMaxQuestions {
		limit = practiceMaxQuestions
	}
	if len(in.QuestionIDs) > 0 {
		limit = len(in.QuestionIDs)
	}
	if in.Source == "bank" && len(in.BankIDs) == 0 && len(in.QuestionIDs) == 0 {
		return nil, nil, false, fmt.Errorf("请选择题库")
	}

	from := "questions q"
	where := "q.is_deleted = 0"
	args := []any{}
	if in.Source == "wrong" && len(in.QuestionIDs) == 0 {
		from = "wrong_book w JOIN questions q ON q.id = w.question_id"
		where = "w.user_id = ? AND w.mastered = 0 AND q.is_deleted = 0"
		args = append(args, userID)
	}
	if len(in.BankIDs) > 0 {
		placeholders, bankArgs := idPlaceholders(in.BankIDs)
		where += " AND q.bank_id IN (" + placeholders + ")"
		args = append(args, bankArgs...)
	}
	if len(in.QuestionIDs) > 0 {
		placeholders, idArgs := idPlaceholders(in.QuestionIDs)
		where += " AND q.id IN (" + placeholders + ")"
		args = append(args, idArgs...)
		if !in.Admin {
			where += ` AND EXISTS (
				SELECT 1 FROM banks b2 WHERE b2.id = q.bank_id AND b2.is_deleted = 0
				  AND (b2.owner_id IS NULL OR b2.owner_id = ?))`
			args = append(args, userID)
		}
	}
	if in.QuestionType != "" {
		where += " AND q.type = ?"
		args = append(args, in.QuestionType)
	}
	if in.Difficulty > 0 {
		where += " AND q.difficulty = ?"
		args = append(args, in.Difficulty)
	}
	orderBy := " ORDER BY q.sort_order, q.id"
	if in.Order == "random" {
		orderBy = " ORDER BY RANDOM()"
	}
	fetchLimit := practiceMaxQuestions + 1
	if in.Count > 0 && in.Count < practiceMaxQuestions {
		fetchLimit = in.Count
	}
	rows, err := s.DB.Query(
		"SELECT q.id FROM "+from+" WHERE "+where+orderBy+" LIMIT ?",
		append(args, fetchLimit)...,
	)
	if err != nil {
		return nil, nil, false, err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, false, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, false, err
	}

	truncated := len(ids) > limit
	if truncated {
		ids = ids[:limit]
	}
	if len(ids) == 0 {
		if len(in.QuestionIDs) > 0 {
			return nil, nil, false, fmt.Errorf("题目不可用或已被删除")
		}
		if in.Source == "wrong" {
			return nil, nil, false, fmt.Errorf("错题本里还没有待复习的题目")
		}
		if in.QuestionType != "" || in.Difficulty > 0 || len(in.BankIDs) > 1 {
			return nil, nil, false, fmt.Errorf("没有符合筛选条件的题目")
		}
		return nil, nil, false, fmt.Errorf("这个题库还没有题目")
	}

	bankName, err := s.practiceBankName(in, userID)
	if err != nil {
		return nil, nil, false, err
	}

	options, err := s.optionsForQuestions(ids)
	if err != nil {
		return nil, nil, false, err
	}
	orders := make(map[int64][]string, len(ids))
	for _, id := range ids {
		keys := make([]string, 0, len(options[id]))
		for _, o := range options[id] {
			keys = append(keys, o.Key)
		}
		if in.ShuffleOptions {
			keys = shuffleKeys(keys)
		}
		orders[id] = keys
	}

	now := nowString()
	idsJSON, _ := json.Marshal(ids)
	ordersJSON, _ := json.Marshal(orders)
	scope := map[string]any{
		"bankIds":          in.BankIDs,
		"questionIds":      in.QuestionIDs,
		"questionType":     in.QuestionType,
		"difficulty":       in.Difficulty,
		"count":            in.Count,
		"shuffleQuestions": in.Order == "random",
		"shuffleOptions":   in.ShuffleOptions,
	}
	scopeJSON, _ := json.Marshal(scope)
	bankID := int64(0)
	if len(in.BankIDs) == 1 {
		bankID = in.BankIDs[0]
	}
	res, err := s.DB.Exec(`
		INSERT INTO practice_sessions(user_id, bank_id, source, question_ids_json, total_count,
		                              order_mode, status, last_seq, scope_json, shuffle_options,
		                              option_orders_json, marked_json, duration_sec, bank_name_snapshot,
		                              created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, 'ongoing', 1, ?, ?, ?, '[]', 0, ?, ?, ?)`,
		userID, bankID, in.Source, string(idsJSON), len(ids), in.Order, string(scopeJSON),
		boolInt(in.ShuffleOptions), string(ordersJSON), bankName, now, now,
	)
	if err != nil {
		return nil, nil, false, err
	}
	id, _ := res.LastInsertId()
	session := &PracticeSession{
		ID: id, UserID: userID, BankID: bankID, BankName: bankName, Source: in.Source,
		TotalCount: len(ids), OrderMode: in.Order, Status: "ongoing", LastSeq: 1,
		ShuffleOptions: in.ShuffleOptions, CreatedAt: now, UpdatedAt: now,
	}
	items, err := s.buildPracticeItems(ids, orders)
	if err != nil {
		return nil, nil, false, err
	}
	return session, items, truncated, nil
}

func (s *Store) practiceBankName(in PracticeInput, userID int64) (string, error) {
	if len(in.BankIDs) == 0 {
		if in.Source == "wrong" {
			return "错题本", nil
		}
		return "自选题目", nil
	}
	placeholders, args := idPlaceholders(in.BankIDs)
	rows, err := s.DB.Query(
		"SELECT id, name FROM banks WHERE id IN ("+placeholders+") AND is_deleted = 0", args...,
	)
	if err != nil {
		return "", err
	}
	names := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return "", err
		}
		names[id] = name
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return "", err
	}
	rows.Close()
	ordered := make([]string, 0, len(in.BankIDs))
	for _, id := range in.BankIDs {
		if name, ok := names[id]; ok {
			ordered = append(ordered, name)
		}
	}
	if len(ordered) == 0 {
		return "", fmt.Errorf("题库不存在")
	}
	return strings.Join(ordered, "、"), nil
}

func (s *Store) optionsForQuestions(ids []int64) (map[int64][]QuestionOption, error) {
	out := map[int64][]QuestionOption{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders, args := idPlaceholders(ids)
	rows, err := s.DB.Query(`
		SELECT question_id, key, content, COALESCE(image, '')
		FROM question_options WHERE question_id IN (`+placeholders+`)
		ORDER BY question_id, sort_order, key`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var questionID int64
		var o QuestionOption
		if err := rows.Scan(&questionID, &o.Key, &o.Content, &o.Image); err != nil {
			return nil, err
		}
		out[questionID] = append(out[questionID], o)
	}
	return out, rows.Err()
}

func (s *Store) questionOptions(questionID int64) ([]QuestionOption, error) {
	options, err := s.optionsForQuestions([]int64{questionID})
	if err != nil {
		return nil, err
	}
	return options[questionID], nil
}

func (s *Store) buildPracticeItems(ids []int64, orders map[int64][]string) ([]PracticeItem, error) {
	if len(ids) == 0 {
		return []PracticeItem{}, nil
	}
	placeholders, args := idPlaceholders(ids)
	rows, err := s.DB.Query(`
		SELECT id, type, stem, COALESCE(stem_image, ''), difficulty
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

	options, err := s.optionsForQuestions(ids)
	if err != nil {
		return nil, err
	}
	out := make([]PracticeItem, 0, len(ids))
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			continue
		}
		opts := options[id]
		keys := orderedOptionKeys(orders[id], opts)
		item.Options = make([]PracticeOption, 0, len(opts))
		byKey := map[string]QuestionOption{}
		for _, o := range opts {
			byKey[o.Key] = o
		}
		for i, key := range keys {
			o, ok := byKey[key]
			if !ok {
				continue
			}
			item.Options = append(item.Options, PracticeOption{
				Key:     o.Key,
				Label:   optionLabel(i),
				Content: o.Content,
				Image:   o.Image,
			})
		}
		out = append(out, item)
	}
	return out, nil
}

func orderedOptionKeys(order []string, options []QuestionOption) []string {
	natural := make([]string, 0, len(options))
	exists := map[string]bool{}
	for _, o := range options {
		natural = append(natural, o.Key)
		exists[o.Key] = true
	}
	if len(order) != len(natural) {
		return natural
	}
	seen := map[string]bool{}
	for _, key := range order {
		if !exists[key] || seen[key] {
			return natural
		}
		seen[key] = true
	}
	return order
}

func optionLabel(index int) string {
	return string(rune('A' + index))
}

func displayLabels(order []string, keys []string) []string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[key] = true
	}
	labels := []string{}
	seen := map[string]bool{}
	for i, key := range order {
		if wanted[key] {
			labels = append(labels, optionLabel(i))
			seen[key] = true
		}
	}
	for _, key := range keys {
		if !seen[key] {
			labels = append(labels, key)
		}
	}
	return labels
}

func shuffleKeys(keys []string) []string {
	out := append([]string(nil), keys...)
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return out
	}
	seed := int64(binary.BigEndian.Uint64(buf[:]) & 0x7fffffffffffffff)
	r := mathrand.New(mathrand.NewSource(seed))
	r.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

func normalizeIDList(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func idPlaceholders(ids []int64) (string, []any) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return placeholders, args
}

func parseIDList(raw string) []int64 {
	var ids []int64
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &ids)
	}
	return ids
}

func parseOptionOrders(raw string) map[int64][]string {
	parsed := map[string][]string{}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &parsed)
	}
	out := make(map[int64][]string, len(parsed))
	for key, order := range parsed {
		id, err := strconv.ParseInt(key, 10, 64)
		if err == nil {
			out[id] = order
		}
	}
	return out
}

func (s *Store) ensurePracticeOrders(ids []int64, orders map[int64][]string) error {
	missing := []int64{}
	for _, id := range ids {
		if len(orders[id]) == 0 {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	options, err := s.optionsForQuestions(missing)
	if err != nil {
		return err
	}
	for _, id := range missing {
		keys := make([]string, 0, len(options[id]))
		for _, o := range options[id] {
			keys = append(keys, o.Key)
		}
		orders[id] = keys
	}
	return nil
}

func (s *Store) GetPracticeSession(userID, sessionID int64) (*PracticeDetail, error) {
	row, err := s.loadPracticeRow(sessionID)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrPracticeNotFound
	}
	ids := parseIDList(row.idsRaw)
	orders := parseOptionOrders(row.optionOrdersRaw)
	if err := s.ensurePracticeOrders(ids, orders); err != nil {
		return nil, err
	}
	items, err := s.buildPracticeItems(ids, orders)
	if err != nil {
		return nil, err
	}
	answers, err := s.practiceAnswerStates(userID, sessionID, orders)
	if err != nil {
		return nil, err
	}
	marked := filterIDList(parseIDList(row.markedRaw), ids)
	lastSeq := row.LastSeq
	if lastSeq < 1 {
		lastSeq = 1
	}
	if lastSeq > len(ids) {
		lastSeq = len(ids)
	}
	stats, err := s.WrongStatsOf(userID)
	if err != nil {
		return nil, err
	}
	return &PracticeDetail{
		Session:   &row.PracticeSession,
		Questions: items,
		Answers:   answers,
		Marked:    marked,
		LastSeq:   lastSeq,
		Stats:     stats,
	}, nil
}

func (s *Store) practiceAnswerStates(userID, sessionID int64, orders map[int64][]string) ([]PracticeAnswerState, error) {
	rows, err := s.DB.Query(`
		SELECT a.question_id, a.chosen_json, a.is_correct, a.answered_at, a.time_spent_ms,
		       COALESCE(q.answer, ''), COALESCE(q.analysis, ''),
		       COALESCE(w.wrong_count, 0), COALESCE(w.correct_streak, 0), COALESCE(w.mastered, 0)
		FROM practice_answers a
		LEFT JOIN questions q ON q.id = a.question_id
		LEFT JOIN wrong_book w ON w.user_id = ? AND w.question_id = a.question_id
		WHERE a.session_id = ?
		ORDER BY a.answered_at, a.question_id`, userID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PracticeAnswerState{}
	for rows.Next() {
		var state PracticeAnswerState
		var chosenRaw, answerRaw string
		var isCorrect, mastered int
		if err := rows.Scan(&state.QuestionID, &chosenRaw, &isCorrect, &state.AnsweredAt,
			&state.TimeSpentMs, &answerRaw, &state.Analysis, &state.WrongCount,
			&state.CorrectStreak, &mastered); err != nil {
			return nil, err
		}
		state.Chosen = parseKeys(chosenRaw)
		state.IsCorrect = isCorrect == 1
		state.Mastered = mastered == 1
		state.Answer = displayLabels(orders[state.QuestionID], parseKeys(answerRaw))
		out = append(out, state)
	}
	return out, rows.Err()
}

func filterIDList(input, allowed []int64) []int64 {
	set := map[int64]bool{}
	for _, id := range allowed {
		set[id] = true
	}
	seen := map[int64]bool{}
	out := []int64{}
	for _, id := range input {
		if set[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
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

func (s *Store) AnswerPractice(userID, sessionID, questionID int64, chosen []string, timeSpentMs int) (*PracticeAnswerResult, error) {
	row, err := s.loadPracticeRow(sessionID)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrPracticeNotFound
	}
	if row.Status != "ongoing" {
		return nil, fmt.Errorf("练习已结束")
	}
	ids := parseIDList(row.idsRaw)
	seq := 0
	for i, id := range ids {
		if id == questionID {
			seq = i + 1
			break
		}
	}
	if seq == 0 {
		return nil, ErrQuestionNotInSet
	}
	orders := parseOptionOrders(row.optionOrdersRaw)
	if err := s.ensurePracticeOrders(ids, orders); err != nil {
		return nil, err
	}
	if timeSpentMs < 0 {
		timeSpentMs = 0
	}
	if timeSpentMs > maxAnswerTimeMs {
		timeSpentMs = maxAnswerTimeMs
	}

	var storedChosen string
	var storedCorrect, storedSpent int
	var storedAt string
	err = s.DB.QueryRow(`
		SELECT chosen_json, is_correct, time_spent_ms, answered_at
		FROM practice_answers WHERE session_id = ? AND question_id = ?`,
		sessionID, questionID,
	).Scan(&storedChosen, &storedCorrect, &storedSpent, &storedAt)
	if err == nil {
		question, qerr := s.GetQuestion(questionID)
		if qerr != nil {
			return nil, fmt.Errorf("题目不存在")
		}
		return s.buildPracticeAnswerResult(userID, question, storedCorrect == 1, storedSpent,
			row.DurationSec, orders[questionID])
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	question, err := s.GetQuestion(questionID)
	if err != nil {
		return nil, fmt.Errorf("题目不存在")
	}
	chosen = normalizeKeys(chosen)
	isCorrect := keysEqual(chosen, question.Answer)
	chosenJSON, _ := json.Marshal(chosen)
	now := nowString()
	res, err := s.DB.Exec(`
		INSERT INTO practice_answers(session_id, question_id, chosen_json, is_correct, answered_at, time_spent_ms)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, question_id) DO NOTHING`,
		sessionID, questionID, string(chosenJSON), boolInt(isCorrect), now, timeSpentMs,
	)
	if err != nil {
		return nil, err
	}
	inserted, _ := res.RowsAffected()

	durationSec := row.DurationSec
	if inserted > 0 {
		var totalMs int64
		if err := s.DB.QueryRow(
			"SELECT COALESCE(SUM(time_spent_ms), 0) FROM practice_answers WHERE session_id = ?",
			sessionID,
		).Scan(&totalMs); err != nil {
			return nil, err
		}
		durationSec = int(totalMs / 1000)
		if _, err := s.DB.Exec(`
			UPDATE practice_sessions SET last_seq = ?, duration_sec = ?, updated_at = ?
			WHERE id = ?`, seq, durationSec, now, sessionID); err != nil {
			return nil, err
		}
		if isCorrect {
			threshold := s.WrongMasterThreshold()
			affected, err := s.DB.Exec(`
				UPDATE wrong_book SET correct_streak = correct_streak + 1,
				       mastered = CASE WHEN correct_streak + 1 >= ? THEN 1 ELSE 0 END,
				       updated_at = ?
				WHERE user_id = ? AND question_id = ?`, threshold, now, userID, questionID)
			if err != nil {
				return nil, err
			}
			_, _ = affected.RowsAffected()
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
	} else {
		var existing int
		if err := s.DB.QueryRow(
			"SELECT is_correct FROM practice_answers WHERE session_id = ? AND question_id = ?",
			sessionID, questionID,
		).Scan(&existing); err != nil {
			return nil, err
		}
		isCorrect = existing == 1
		timeSpentMs = 0
	}

	result, err := s.buildPracticeAnswerResult(userID, question, isCorrect, timeSpentMs, durationSec, orders[questionID])
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Store) buildPracticeAnswerResult(userID int64, question *Question, isCorrect bool, timeSpentMs, durationSec int, order []string) (*PracticeAnswerResult, error) {
	result := &PracticeAnswerResult{
		IsCorrect:   isCorrect,
		Answer:      displayLabels(order, question.Answer),
		Analysis:    question.Analysis,
		TimeSpentMs: timeSpentMs,
		DurationSec: durationSec,
	}
	var wrongCount, streak int
	var mastered int
	err := s.DB.QueryRow(
		"SELECT wrong_count, correct_streak, mastered FROM wrong_book WHERE user_id = ? AND question_id = ?",
		userID, question.ID,
	).Scan(&wrongCount, &streak, &mastered)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

func (s *Store) UpdatePracticePosition(userID, sessionID int64, seq int, marked []int64) error {
	row, err := s.loadPracticeRow(sessionID)
	if err != nil {
		return err
	}
	if row.UserID != userID {
		return ErrPracticeNotFound
	}
	if row.Status != "ongoing" {
		return fmt.Errorf("练习已结束")
	}
	ids := parseIDList(row.idsRaw)
	if seq < 1 || seq > len(ids) {
		return fmt.Errorf("题号无效")
	}
	markedJSON, _ := json.Marshal(filterIDList(marked, ids))
	if _, err := s.DB.Exec(
		"UPDATE practice_sessions SET last_seq = ?, marked_json = ?, updated_at = ? WHERE id = ?",
		seq, string(markedJSON), nowString(), sessionID,
	); err != nil {
		return err
	}
	return nil
}

func (s *Store) FinishPractice(userID, sessionID int64) (*PracticeSummary, error) {
	row, err := s.loadPracticeRow(sessionID)
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrPracticeNotFound
	}
	now := nowString()
	if row.Status == "ongoing" {
		if _, err := s.DB.Exec(`
			UPDATE practice_sessions
			SET status = 'finished', finished_at = ?, updated_at = ?,
			    duration_sec = (SELECT COALESCE(SUM(time_spent_ms), 0) / 1000
			                    FROM practice_answers WHERE session_id = ?)
			WHERE id = ?`, now, now, sessionID, sessionID); err != nil {
			return nil, err
		}
	}
	var answered, correct, duration int
	if err := s.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(is_correct), 0) FROM practice_answers WHERE session_id = ?`,
		sessionID,
	).Scan(&answered, &correct); err != nil {
		return nil, err
	}
	if err := s.DB.QueryRow(
		"SELECT COALESCE(duration_sec, 0) FROM practice_sessions WHERE id = ?", sessionID,
	).Scan(&duration); err != nil {
		return nil, err
	}
	return &PracticeSummary{
		SessionID:     sessionID,
		TotalCount:    row.TotalCount,
		AnsweredCount: answered,
		CorrectCount:  correct,
		DurationSec:   duration,
		Status:        "finished",
	}, nil
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
	rows, err := s.DB.Query(practiceSessionSelect+`
		WHERE s.user_id = ?
		ORDER BY CASE WHEN s.status = 'ongoing' THEN 0 ELSE 1 END, s.updated_at DESC, s.id DESC
		LIMIT 10`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PracticeSession{}
	for rows.Next() {
		row, err := scanPracticeSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row.PracticeSession)
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
