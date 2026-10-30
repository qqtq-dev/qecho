package store

import (
	"sort"
	"time"
)

type StatsTrendPoint struct {
	Date     string `json:"date"`
	Answered int    `json:"answered"`
	Correct  int    `json:"correct"`
}

type StatsBank struct {
	BankID   int64  `json:"bankId"`
	BankName string `json:"bankName"`
	Answered int    `json:"answered"`
	Correct  int    `json:"correct"`
}

type StatsTag struct {
	TagID    int64  `json:"tagId"`
	TagName  string `json:"tagName"`
	Answered int    `json:"answered"`
	Correct  int    `json:"correct"`
}

type Stats struct {
	Range            string            `json:"range"`
	AnsweredTotal    int               `json:"answeredTotal"`
	CorrectTotal     int               `json:"correctTotal"`
	PracticeCount    int               `json:"practiceCount"`
	ExamCount        int               `json:"examCount"`
	ExamScoredCount  int               `json:"examScoredCount"`
	ExamAverageScore float64           `json:"examAverageScore"`
	Trend            []StatsTrendPoint `json:"trend"`
	Banks            []StatsBank       `json:"banks"`
}

// Stats 按时间范围汇总练习与考试作答，趋势按本地日期分组。
func (s *Store) Stats(user *User, rangeKey string) (*Stats, error) {
	rangeKey, days, startDay := statsRange(rangeKey)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	admin := user.Role == "admin"
	out := &Stats{Range: rangeKey}

	var answered, correct int
	pq := `SELECT COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM practice_answers a JOIN practice_sessions s ON s.id = a.session_id
		WHERE s.user_id = ?`
	pArgs := []any{user.ID}
	if startDay != "" {
		pq += " AND date(a.answered_at, 'localtime') >= ?"
		pArgs = append(pArgs, startDay)
	}
	if err := s.DB.QueryRow(pq, pArgs...).Scan(&answered, &correct); err != nil {
		return nil, err
	}

	var examAnswered, examCorrect int
	eq := `SELECT COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM exam_answers a
		JOIN exam_attempts t ON t.id = a.attempt_id
		JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'submitted' AND a.is_correct IS NOT NULL`
	eArgs := []any{}
	if startDay != "" {
		eq += " AND date(t.submitted_at, 'localtime') >= ?"
		eArgs = append(eArgs, startDay)
	}
	if !admin {
		eq += " AND e.owner_id = ?"
		eArgs = append(eArgs, user.ID)
	}
	if err := s.DB.QueryRow(eq, eArgs...).Scan(&examAnswered, &examCorrect); err != nil {
		return nil, err
	}
	out.AnsweredTotal = answered + examAnswered
	out.CorrectTotal = correct + examCorrect

	cpq := "SELECT COUNT(*) FROM practice_sessions WHERE user_id = ?"
	cArgs := []any{user.ID}
	if startDay != "" {
		cpq += " AND date(created_at, 'localtime') >= ?"
		cArgs = append(cArgs, startDay)
	}
	if err := s.DB.QueryRow(cpq, cArgs...).Scan(&out.PracticeCount); err != nil {
		return nil, err
	}

	aq := `SELECT COUNT(*), COUNT(t.score), COALESCE(AVG(t.score), 0)
		FROM exam_attempts t JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'submitted'`
	aArgs := []any{}
	if startDay != "" {
		aq += " AND date(t.submitted_at, 'localtime') >= ?"
		aArgs = append(aArgs, startDay)
	}
	if !admin {
		aq += " AND e.owner_id = ?"
		aArgs = append(aArgs, user.ID)
	}
	if err := s.DB.QueryRow(aq, aArgs...).Scan(&out.ExamCount, &out.ExamScoredCount, &out.ExamAverageScore); err != nil {
		return nil, err
	}

	trend := map[string]*StatsTrendPoint{}
	addPoint := func(date string, n, c int) {
		p := trend[date]
		if p == nil {
			p = &StatsTrendPoint{Date: date}
			trend[date] = p
		}
		p.Answered += n
		p.Correct += c
	}

	tq := `SELECT date(a.answered_at, 'localtime'), COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM practice_answers a JOIN practice_sessions s ON s.id = a.session_id
		WHERE s.user_id = ?`
	tArgs := []any{user.ID}
	if startDay != "" {
		tq += " AND date(a.answered_at, 'localtime') >= ?"
		tArgs = append(tArgs, startDay)
	}
	tq += " GROUP BY 1"
	if err := s.scanTrend(tq, tArgs, addPoint); err != nil {
		return nil, err
	}

	xq := `SELECT date(t.submitted_at, 'localtime'), COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM exam_answers a
		JOIN exam_attempts t ON t.id = a.attempt_id
		JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'submitted' AND a.is_correct IS NOT NULL`
	xArgs := []any{}
	if startDay != "" {
		xq += " AND date(t.submitted_at, 'localtime') >= ?"
		xArgs = append(xArgs, startDay)
	}
	if !admin {
		xq += " AND e.owner_id = ?"
		xArgs = append(xArgs, user.ID)
	}
	xq += " GROUP BY 1"
	if err := s.scanTrend(xq, xArgs, addPoint); err != nil {
		return nil, err
	}

	if days == 0 {
		if len(trend) == 0 {
			out.Trend = append(out.Trend, StatsTrendPoint{Date: today.Format("2006-01-02")})
		} else {
			earliest := today
			for k := range trend {
				d, err := time.ParseInLocation("2006-01-02", k, now.Location())
				if err != nil {
					continue
				}
				if d.Before(earliest) {
					earliest = d
				}
			}
			for d := earliest; !d.After(today); d = d.AddDate(0, 0, 1) {
				out.Trend = append(out.Trend, trendPoint(trend, d.Format("2006-01-02")))
			}
		}
	} else {
		for i := days - 1; i >= 0; i-- {
			out.Trend = append(out.Trend, trendPoint(trend, today.AddDate(0, 0, -i).Format("2006-01-02")))
		}
	}

	banks, err := s.statsBanks(user, admin, startDay)
	if err != nil {
		return nil, err
	}
	out.Banks = banks
	return out, nil
}

func statsRange(rangeKey string) (string, int, string) {
	days := 7
	switch rangeKey {
	case "30d":
		days = 30
	case "all":
		days = 0
	default:
		rangeKey = "7d"
	}
	if days == 0 {
		return rangeKey, 0, ""
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return rangeKey, days, today.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
}

// StatsTags 统计带标签题目的作答正确率，范围与 Stats 一致。
func (s *Store) StatsTags(user *User, rangeKey string) ([]StatsTag, error) {
	_, _, startDay := statsRange(rangeKey)

	pq := `SELECT a.question_id, a.is_correct
		FROM practice_answers a JOIN practice_sessions s ON s.id = a.session_id
		WHERE s.user_id = ?`
	pArgs := []any{user.ID}
	if startDay != "" {
		pq += " AND date(a.answered_at, 'localtime') >= ?"
		pArgs = append(pArgs, startDay)
	}

	xq := `SELECT a.question_id, a.is_correct
		FROM exam_answers a
		JOIN exam_attempts t ON t.id = a.attempt_id
		JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'submitted' AND a.is_correct IS NOT NULL`
	xArgs := []any{}
	if startDay != "" {
		xq += " AND date(t.submitted_at, 'localtime') >= ?"
		xArgs = append(xArgs, startDay)
	}
	if user.Role != "admin" {
		xq += " AND e.owner_id = ?"
		xArgs = append(xArgs, user.ID)
	}

	query := `SELECT t.id, t.name, COUNT(*), COALESCE(SUM(x.is_correct), 0)
		FROM (` + pq + ` UNION ALL ` + xq + `) x
		JOIN question_tags qt ON qt.question_id = x.question_id
		JOIN tags t ON t.id = qt.tag_id
		GROUP BY t.id, t.name
		ORDER BY COUNT(*) DESC, t.name`
	rows, err := s.DB.Query(query, append(pArgs, xArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StatsTag{}
	for rows.Next() {
		var t StatsTag
		if err := rows.Scan(&t.TagID, &t.TagName, &t.Answered, &t.Correct); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) scanTrend(query string, args []any, add func(date string, n, c int)) error {
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var n, c int
		if err := rows.Scan(&date, &n, &c); err != nil {
			return err
		}
		add(date, n, c)
	}
	return rows.Err()
}

func trendPoint(trend map[string]*StatsTrendPoint, date string) StatsTrendPoint {
	if p := trend[date]; p != nil {
		return *p
	}
	return StatsTrendPoint{Date: date}
}

func (s *Store) statsBanks(user *User, admin bool, startDay string) ([]StatsBank, error) {
	merged := map[int64]*StatsBank{}
	add := func(id int64, name string, n, c int) {
		b := merged[id]
		if b == nil {
			b = &StatsBank{BankID: id, BankName: name}
			merged[id] = b
		}
		b.Answered += n
		b.Correct += c
	}

	pq := `SELECT COALESCE(q.bank_id, s.bank_id, 0),
			COALESCE(b.name, eb.name, '题库已删除'),
			COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM practice_answers a
		JOIN practice_sessions s ON s.id = a.session_id
		LEFT JOIN questions q ON q.id = a.question_id
		LEFT JOIN banks b ON b.id = COALESCE(q.bank_id, s.bank_id)
		LEFT JOIN banks eb ON eb.id = s.bank_id
		WHERE s.user_id = ?`
	pArgs := []any{user.ID}
	if startDay != "" {
		pq += " AND date(a.answered_at, 'localtime') >= ?"
		pArgs = append(pArgs, startDay)
	}
	pq += " GROUP BY 1, 2"
	if err := s.scanBanks(pq, pArgs, add); err != nil {
		return nil, err
	}

	xq := `SELECT COALESCE(q.bank_id, e.bank_id, 0),
			COALESCE(b.name, eb.name, '题库已删除'),
			COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM exam_answers a
		JOIN exam_attempts t ON t.id = a.attempt_id
		JOIN exams e ON e.id = t.exam_id
		LEFT JOIN questions q ON q.id = a.question_id
		LEFT JOIN banks b ON b.id = COALESCE(q.bank_id, e.bank_id)
		LEFT JOIN banks eb ON eb.id = e.bank_id
		WHERE t.status = 'submitted' AND a.is_correct IS NOT NULL`
	xArgs := []any{}
	if startDay != "" {
		xq += " AND date(t.submitted_at, 'localtime') >= ?"
		xArgs = append(xArgs, startDay)
	}
	if !admin {
		xq += " AND e.owner_id = ?"
		xArgs = append(xArgs, user.ID)
	}
	xq += " GROUP BY 1, 2"
	if err := s.scanBanks(xq, xArgs, add); err != nil {
		return nil, err
	}

	out := make([]StatsBank, 0, len(merged))
	for _, b := range merged {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Answered != out[j].Answered {
			return out[i].Answered > out[j].Answered
		}
		return out[i].BankName < out[j].BankName
	})
	return out, nil
}

func (s *Store) scanBanks(query string, args []any, add func(id int64, name string, n, c int)) error {
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		var n, c int
		if err := rows.Scan(&id, &name, &n, &c); err != nil {
			return err
		}
		add(id, name, n, c)
	}
	return rows.Err()
}
