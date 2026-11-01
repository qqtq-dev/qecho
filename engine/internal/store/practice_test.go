package store

import (
	"reflect"
	"testing"
)

func practiceTestStore(t *testing.T) (*Store, *User, int64, int64, map[string]*Question) {
	t.Helper()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	user, err := st.CreateUser("practice-user", "pw", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	bank1, err := st.CreateBank("银行一", "", nil)
	if err != nil {
		t.Fatalf("create bank1: %v", err)
	}
	bank2, err := st.CreateBank("银行二", "", nil)
	if err != nil {
		t.Fatalf("create bank2: %v", err)
	}
	mustQuestion := func(bankID int64, stem, typ string, answer []string, difficulty int) *Question {
		t.Helper()
		q, err := st.CreateQuestion(bankID, QuestionInput{
			Type:       typ,
			Stem:       stem,
			Answer:     answer,
			Analysis:   "解析-" + stem,
			Difficulty: difficulty,
			Options: []QuestionOption{
				{Key: "A", Content: "甲"},
				{Key: "B", Content: "乙"},
				{Key: "C", Content: "丙"},
			},
		})
		if err != nil {
			t.Fatalf("create question: %v", err)
		}
		return q
	}
	questions := map[string]*Question{
		"q1": mustQuestion(bank1.ID, "单选一", "single", []string{"B"}, 1),
		"q2": mustQuestion(bank1.ID, "多选二", "multiple", []string{"A", "C"}, 2),
		"q3": mustQuestion(bank2.ID, "单选三", "single", []string{"A"}, 3),
	}
	return st, user, bank1.ID, bank2.ID, questions
}

func optionLabelOf(t *testing.T, item PracticeItem, key string) string {
	t.Helper()
	for _, opt := range item.Options {
		if opt.Key == key {
			return opt.Label
		}
	}
	t.Fatalf("option %s not found in %v", key, item.Options)
	return ""
}

func TestCreatePracticeFiltersAndOptionOrder(t *testing.T) {
	st, user, bank1, bank2, questions := practiceTestStore(t)

	session, items, truncated, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs:        []int64{bank1, bank2},
		Count:          0,
		Order:          "fixed",
		ShuffleOptions: true,
	})
	if err != nil {
		t.Fatalf("create practice: %v", err)
	}
	if truncated {
		t.Fatal("did not expect truncation")
	}
	if session.TotalCount != 3 || len(items) != 3 {
		t.Fatalf("total=%d items=%d, want 3", session.TotalCount, len(items))
	}
	if session.BankName != "银行一、银行二" {
		t.Fatalf("bank name = %q", session.BankName)
	}
	if session.Status != "ongoing" || session.LastSeq != 1 {
		t.Fatalf("unexpected session state: %+v", session)
	}
	for _, item := range items {
		if len(item.Options) != 3 {
			t.Fatalf("options = %d", len(item.Options))
		}
		seen := map[string]bool{}
		for i, opt := range item.Options {
			if opt.Label != optionLabel(i) {
				t.Fatalf("label %q at %d", opt.Label, i)
			}
			seen[opt.Key] = true
		}
		if !seen["A"] || !seen["B"] || !seen["C"] {
			t.Fatalf("option keys lost: %+v", item.Options)
		}
	}

	detail, err := st.GetPracticeSession(user.ID, session.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if len(detail.Questions) != 3 || len(detail.Answers) != 0 || len(detail.Marked) != 0 {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	for i := range items {
		got := detail.Questions[i]
		if !reflect.DeepEqual(items[i].Options, got.Options) {
			t.Fatalf("option order not persisted for question %d: %+v vs %+v",
				items[i].ID, items[i].Options, got.Options)
		}
	}

	onlyMultiple, items2, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank1, bank2}, QuestionType: "multiple", Order: "fixed",
	})
	if err != nil {
		t.Fatalf("filter type: %v", err)
	}
	if onlyMultiple.TotalCount != 1 || items2[0].ID != questions["q2"].ID {
		t.Fatalf("type filter failed: %+v", items2)
	}

	onlyHard, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank1, bank2}, Difficulty: 3, Order: "fixed",
	})
	if err != nil {
		t.Fatalf("filter difficulty: %v", err)
	}
	if onlyHard.TotalCount != 1 {
		t.Fatalf("difficulty filter total = %d", onlyHard.TotalCount)
	}

	limited, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank2}, Count: 1, Order: "fixed",
	})
	if err != nil {
		t.Fatalf("count session: %v", err)
	}
	if limited.TotalCount != 1 {
		t.Fatalf("count total = %d", limited.TotalCount)
	}

	if _, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		Source: "wrong", Order: "fixed",
	}); err == nil {
		t.Fatal("expected empty wrong book error")
	}
}

func TestPracticeResumeAndIdempotentAnswers(t *testing.T) {
	st, user, bank1, _, questions := practiceTestStore(t)
	q1, q2 := questions["q1"], questions["q2"]

	session, items, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank1}, Count: 0, Order: "fixed", ShuffleOptions: true,
	})
	if err != nil {
		t.Fatalf("create practice: %v", err)
	}

	var item1, item2 PracticeItem
	for _, item := range items {
		switch item.ID {
		case q1.ID:
			item1 = item
		case q2.ID:
			item2 = item
		}
	}

	wrong := []string{"A"}
	first, err := st.AnswerPractice(user.ID, session.ID, q1.ID, wrong, 1500)
	if err != nil {
		t.Fatalf("answer wrong: %v", err)
	}
	if first.IsCorrect || first.WrongCount != 1 || first.DurationSec != 1 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	if want := []string{optionLabelOf(t, item1, "B")}; !reflect.DeepEqual(first.Answer, want) {
		t.Fatalf("answer labels = %v, want %v", first.Answer, want)
	}

	again, err := st.AnswerPractice(user.ID, session.ID, q1.ID, []string{"B"}, 999)
	if err != nil {
		t.Fatalf("repeat answer: %v", err)
	}
	if again.IsCorrect || again.WrongCount != 1 {
		t.Fatalf("repeat should return stored result: %+v", again)
	}
	if again.DurationSec != 1 {
		t.Fatalf("repeat should not extend duration: %d", again.DurationSec)
	}

	if err := st.UpdatePracticePosition(user.ID, session.ID, 2, []int64{q2.ID}); err != nil {
		t.Fatalf("position: %v", err)
	}
	detail, err := st.GetPracticeSession(user.ID, session.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.LastSeq != 2 || !reflect.DeepEqual(detail.Marked, []int64{q2.ID}) {
		t.Fatalf("resume state: %+v", detail)
	}
	if len(detail.Answers) != 1 {
		t.Fatalf("answers = %d", len(detail.Answers))
	}
	saved := detail.Answers[0]
	if saved.QuestionID != q1.ID || !reflect.DeepEqual(saved.Chosen, wrong) || saved.IsCorrect {
		t.Fatalf("saved answer: %+v", saved)
	}
	if want := []string{optionLabelOf(t, item1, "B")}; !reflect.DeepEqual(saved.Answer, want) {
		t.Fatalf("saved answer labels = %v", saved.Answer)
	}

	correct, err := st.AnswerPractice(user.ID, session.ID, q2.ID, []string{"A", "C"}, 2500)
	if err != nil {
		t.Fatalf("answer correct: %v", err)
	}
	if !correct.IsCorrect {
		t.Fatalf("expected correct: %+v", correct)
	}
	expected := []string{}
	for _, opt := range item2.Options {
		if opt.Key == "A" || opt.Key == "C" {
			expected = append(expected, opt.Label)
		}
	}
	if !reflect.DeepEqual(correct.Answer, expected) {
		t.Fatalf("correct labels = %v, want %v", correct.Answer, expected)
	}
	if correct.DurationSec != 4 {
		t.Fatalf("duration = %d, want 4", correct.DurationSec)
	}

	summary, err := st.FinishPractice(user.ID, session.ID)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if summary.Status != "finished" || summary.AnsweredCount != 2 || summary.CorrectCount != 1 || summary.DurationSec != 4 {
		t.Fatalf("summary: %+v", summary)
	}
	if _, err := st.AnswerPractice(user.ID, session.ID, q1.ID, []string{"B"}, 10); err == nil {
		t.Fatal("answer after finish should fail")
	}
	if err := st.UpdatePracticePosition(user.ID, session.ID, 1, nil); err == nil {
		t.Fatal("position after finish should fail")
	}
	againSummary, err := st.FinishPractice(user.ID, session.ID)
	if err != nil {
		t.Fatalf("finish again: %v", err)
	}
	if !reflect.DeepEqual(summary, againSummary) {
		t.Fatalf("finish not idempotent: %+v vs %+v", summary, againSummary)
	}

	list, err := st.ListPracticeSessions(user.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Status != "finished" || list[0].Answered != 2 || list[0].Correct != 1 {
		t.Fatalf("list: %+v", list)
	}
}

func TestAnswerTimeIsClamped(t *testing.T) {
	st, user, bank1, _, questions := practiceTestStore(t)
	session, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank1}, Count: 1, Order: "fixed",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	result, err := st.AnswerPractice(user.ID, session.ID, questions["q1"].ID, []string{"B"}, maxAnswerTimeMs*3)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if result.TimeSpentMs != maxAnswerTimeMs || result.DurationSec != maxAnswerTimeMs/1000 {
		t.Fatalf("time not clamped: %+v", result)
	}
}

func TestPracticeRangeSkipsLeadingQuestions(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	user, err := st.CreateUser("range-user", "pw", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	bank, err := st.CreateBank("范围题库", "", nil)
	if err != nil {
		t.Fatalf("create bank: %v", err)
	}
	ids := make([]int64, 0, 10)
	for i := 1; i <= 10; i++ {
		q, err := st.CreateQuestion(bank.ID, QuestionInput{
			Type: "single", Stem: "题" + string(rune('0'+i%10)), Answer: []string{"A"},
			Options: []QuestionOption{{Key: "A", Content: "甲"}, {Key: "B", Content: "乙"}},
		})
		if err != nil {
			t.Fatalf("create question %d: %v", i, err)
		}
		ids = append(ids, q.ID)
	}

	cases := []struct {
		name       string
		start, end int
		wantFirst  int64
		wantTotal  int
	}{
		{"从头开始", 0, 0, ids[0], 10},
		{"第3到第5", 3, 5, ids[2], 3},
		{"只给起始", 8, 0, ids[7], 3},
		{"范围超过末尾", 8, 99, ids[7], 3},
		{"整段覆盖", 1, 10, ids[0], 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			session, items, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
				BankIDs: []int64{bank.ID}, Order: "fixed", StartSeq: c.start, EndSeq: c.end,
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if session.TotalCount != c.wantTotal || len(items) != c.wantTotal {
				t.Fatalf("total=%d items=%d, want %d", session.TotalCount, len(items), c.wantTotal)
			}
			if items[0].ID != c.wantFirst {
				t.Fatalf("first question = %d, want %d", items[0].ID, c.wantFirst)
			}
			if wantStart := c.start; wantStart >= 1 && session.RangeStart != wantStart {
				t.Fatalf("rangeStart = %d, want %d", session.RangeStart, wantStart)
			}
			if c.end >= 1 && session.RangeEnd != c.end {
				t.Fatalf("rangeEnd = %d, want %d", session.RangeEnd, c.end)
			}
		})
	}

	if _, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank.ID}, Order: "fixed", StartSeq: 50, EndSeq: 60,
	}); err == nil {
		t.Fatal("expected out-of-range start to fail")
	}
	if _, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank.ID}, Order: "fixed", StartSeq: 5, EndSeq: 3,
	}); err == nil {
		t.Fatal("expected inverted range to fail")
	}

	total, err := st.CountPracticeCandidates(user.ID, PracticeInput{BankIDs: []int64{bank.ID}})
	if err != nil || total != 10 {
		t.Fatalf("candidates = %d, err = %v, want 10", total, err)
	}

	random, items2, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank.ID}, Order: "random", Count: 4, StartSeq: 5, EndSeq: 6,
	})
	if err != nil {
		t.Fatalf("random create: %v", err)
	}
	if random.TotalCount != 4 || len(items2) != 4 {
		t.Fatalf("random total=%d, want 4 (range must be ignored)", random.TotalCount)
	}
	if random.RangeStart != 0 || random.RangeEnd != 0 {
		t.Fatalf("random range should be cleared: %+v", random)
	}
}

func TestWrongBookExcludesDeletedQuestions(t *testing.T) {
	st, user, bank1, _, questions := practiceTestStore(t)
	q1, q2 := questions["q1"], questions["q2"]

	session, _, _, err := st.CreatePracticeSession(user.ID, PracticeInput{
		BankIDs: []int64{bank1}, Count: 0, Order: "fixed",
	})
	if err != nil {
		t.Fatalf("create practice: %v", err)
	}
	// 两题都答错：q1 正确答案 B，q2 正确答案 A+C
	if _, err := st.AnswerPractice(user.ID, session.ID, q1.ID, []string{"A"}, 1000); err != nil {
		t.Fatalf("answer q1: %v", err)
	}
	if _, err := st.AnswerPractice(user.ID, session.ID, q2.ID, []string{"B"}, 1000); err != nil {
		t.Fatalf("answer q2: %v", err)
	}

	stats, err := st.WrongStatsOf(user.ID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	items, total, listStats, err := st.ListWrong(user.ID, "pending", 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if stats.Pending != 2 || stats.Total != 2 || total != 2 || len(items) != 2 {
		t.Fatalf("初始应全部可见：stats=%+v total=%d items=%d", stats, total, len(items))
	}
	if stats != listStats {
		t.Fatalf("stats 与列表不一致：%+v vs %+v", stats, listStats)
	}

	// 软删除其中一道题后，统计与列表都必须同步排除它
	if err := st.DeleteQuestion(q1.ID); err != nil {
		t.Fatalf("delete question: %v", err)
	}
	stats, err = st.WrongStatsOf(user.ID)
	if err != nil {
		t.Fatalf("stats after delete: %v", err)
	}
	items, total, listStats, err = st.ListWrong(user.ID, "pending", 1, 20)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if stats.Pending != 1 || stats.Total != 1 {
		t.Fatalf("统计应排除已删题目：%+v", stats)
	}
	if total != 1 || len(items) != 1 || items[0].QuestionID != q2.ID {
		t.Fatalf("列表应只剩 q2：total=%d items=%+v", total, items)
	}
	if stats != listStats {
		t.Fatalf("stats 与列表不一致：%+v vs %+v", stats, listStats)
	}

	ov, err := st.Overview(user)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if ov.WrongPending != 1 {
		t.Fatalf("概览待复习 = %d, want 1", ov.WrongPending)
	}
}
