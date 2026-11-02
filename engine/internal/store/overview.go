package store

import "time"

type OverviewPractice struct {
	ID            int64  `json:"id"`
	BankName      string `json:"bankName"`
	Source        string `json:"source"`
	AnsweredCount int    `json:"answeredCount"`
	TotalCount    int    `json:"totalCount"`
	CreatedAt     string `json:"createdAt"`
}

type OverviewExam struct {
	ID               int64  `json:"id"`
	Title            string `json:"title"`
	TotalCount       int    `json:"totalCount"`
	CloseAt          string `json:"closeAt"`
	ParticipantCount int    `json:"participantCount"`
}

type Overview struct {
	QuestionCount   int                `json:"questionCount"`
	TodayAnswered   int                `json:"todayAnswered"`
	AnsweredTotal   int                `json:"answeredTotal"`
	CorrectTotal    int                `json:"correctTotal"`
	WrongPending    int                `json:"wrongPending"`
	WrongMastered   int                `json:"wrongMastered"`
	UnfinishedCount int                `json:"unfinishedCount"`
	Practices       []OverviewPractice `json:"practices"`
	Exams           []OverviewExam     `json:"exams"`
}

// Overview 汇总概览页需要的实时数据，按用户可见范围统计。
func (s *Store) Overview(user *User) (*Overview, error) {
	out := &Overview{Practices: []OverviewPractice{}, Exams: []OverviewExam{}}
	admin := user.Role == "admin"
	local := time.Now()
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	today := dayStart.UTC().Format(time.RFC3339Nano)

	q := `SELECT COUNT(*) FROM questions q JOIN banks b ON b.id = q.bank_id
		WHERE q.is_deleted = 0 AND b.is_deleted = 0`
	args := []any{}
	if !admin {
		cond, condArgs := bankVisibleClause("b", user)
		q += " AND " + cond
		args = append(args, condArgs...)
	}
	if err := s.DB.QueryRow(q, args...).Scan(&out.QuestionCount); err != nil {
		return nil, err
	}

	var practiceToday, practiceCorrectToday int
	if err := s.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM practice_answers a JOIN practice_sessions s ON s.id = a.session_id
		WHERE s.user_id = ? AND datetime(a.answered_at) >= datetime(?)`,
		user.ID, today,
	).Scan(&practiceToday, &practiceCorrectToday); err != nil {
		return nil, err
	}

	examScope := ""
	examArgs := []any{}
	if !admin {
		examScope = " AND e.owner_id = ?"
		examArgs = append(examArgs, user.ID)
	}

	var examToday int
	if err := s.DB.QueryRow(`
		SELECT COUNT(*)
		FROM exam_answers a
		JOIN exam_attempts t ON t.id = a.attempt_id
		JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'submitted' AND a.is_correct IS NOT NULL
		  AND datetime(t.submitted_at) >= datetime(?)`+examScope,
		append([]any{today}, examArgs...)...,
	).Scan(&examToday); err != nil {
		return nil, err
	}
	out.TodayAnswered = practiceToday + examToday

	var answered, correct int
	if err := s.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM practice_answers a JOIN practice_sessions s ON s.id = a.session_id
		WHERE s.user_id = ?`, user.ID,
	).Scan(&answered, &correct); err != nil {
		return nil, err
	}
	var examAnswered, examCorrect int
	if err := s.DB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(a.is_correct), 0)
		FROM exam_answers a
		JOIN exam_attempts t ON t.id = a.attempt_id
		JOIN exams e ON e.id = t.exam_id
		WHERE t.status = 'submitted' AND a.is_correct IS NOT NULL`+examScope,
		examArgs...,
	).Scan(&examAnswered, &examCorrect); err != nil {
		return nil, err
	}
	out.AnsweredTotal = answered + examAnswered
	out.CorrectTotal = correct + examCorrect

	if err := s.DB.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN w.mastered = 0 THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN w.mastered = 1 THEN 1 ELSE 0 END), 0)
		`+wrongVisibleFrom+`
		WHERE w.user_id = ?`, user.ID,
	).Scan(&out.WrongPending, &out.WrongMastered); err != nil {
		return nil, err
	}

	rows, err := s.DB.Query(`
		SELECT s.id, COALESCE(NULLIF(s.bank_name_snapshot, ''), b.name,
		                     CASE WHEN s.source = 'wrong' THEN '错题本' ELSE '题库已删除' END),
		       s.source, s.total_count, s.created_at,
		       (SELECT COUNT(*) FROM practice_answers a WHERE a.session_id = s.id)
		FROM practice_sessions s LEFT JOIN banks b ON b.id = s.bank_id
		WHERE s.user_id = ? AND s.status = 'ongoing'
		  AND (SELECT COUNT(*) FROM practice_answers a WHERE a.session_id = s.id) < s.total_count
		  AND datetime(s.created_at) >= datetime('now', '-30 days')
		ORDER BY s.id DESC LIMIT 3`, user.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p OverviewPractice
		if err := rows.Scan(&p.ID, &p.BankName, &p.Source, &p.TotalCount, &p.CreatedAt,
			&p.AnsweredCount); err != nil {
			rows.Close()
			return nil, err
		}
		out.Practices = append(out.Practices, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out.UnfinishedCount = len(out.Practices)

	examQuery := `
		SELECT e.id, e.title, e.total_count, e.close_at,
		       (SELECT COUNT(*) FROM exam_attempts a
		        WHERE a.exam_id = e.id AND a.status = 'ongoing')
		FROM exams e
		WHERE e.status = 'published'
		  AND datetime(e.open_at) <= datetime('now')
		  AND datetime(e.close_at) > datetime('now')`
	if !admin {
		examQuery += " AND e.owner_id = ?"
	}
	examQuery += " ORDER BY e.close_at ASC LIMIT 3"
	erows, err := s.DB.Query(examQuery, examArgs...)
	if err != nil {
		return nil, err
	}
	for erows.Next() {
		var e OverviewExam
		if err := erows.Scan(&e.ID, &e.Title, &e.TotalCount, &e.CloseAt, &e.ParticipantCount); err != nil {
			erows.Close()
			return nil, err
		}
		out.Exams = append(out.Exams, e)
	}
	erows.Close()
	return out, erows.Err()
}
