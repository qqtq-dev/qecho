package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

func (s *Server) createPractice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BankID         int64    `json:"bankId"`
		BankIDs        []int64  `json:"bankIds"`
		Count          int      `json:"count"`
		Order          string   `json:"order"`
		StartSeq       int      `json:"startSeq"`
		EndSeq         int      `json:"endSeq"`
		Source         string   `json:"source"`
		QuestionType   string   `json:"questionType"`
		Difficulty     int      `json:"difficulty"`
		Tags           []string `json:"tags"`
		ShuffleOptions bool     `json:"shuffleOptions"`
		QuestionIDs    []int64  `json:"questionIds"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if in.Source != "wrong" {
		in.Source = "bank"
	}
	if in.Source == "bank" && in.BankID > 0 && len(in.BankIDs) == 0 {
		in.BankIDs = []int64{in.BankID}
	}
	if in.Difficulty < 0 || in.Difficulty > 5 {
		response.BadRequest(w, "难度筛选不正确")
		return
	}
	for _, id := range in.BankIDs {
		if id <= 0 {
			response.BadRequest(w, "题库编号无效")
			return
		}
		if _, ok := s.bankFor(w, r, id, accessRead); !ok {
			return
		}
	}
	if in.Source == "bank" && len(in.BankIDs) == 0 && len(in.QuestionIDs) == 0 {
		response.BadRequest(w, "请选择题库")
		return
	}

	user := userFrom(r)
	session, items, truncated, err := s.store.CreatePracticeSession(user.ID, store.PracticeInput{
		BankIDs:        in.BankIDs,
		QuestionIDs:    in.QuestionIDs,
		Count:          in.Count,
		Order:          in.Order,
		StartSeq:       in.StartSeq,
		EndSeq:         in.EndSeq,
		Source:         in.Source,
		QuestionType:   in.QuestionType,
		Difficulty:     in.Difficulty,
		Tags:           in.Tags,
		ShuffleOptions: in.ShuffleOptions,
		Admin:          user.Role == "admin",
	})
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{
		"session":   session,
		"questions": items,
		"truncated": truncated,
	})
}

// previewPractice 返回当前筛选下的可练习题数，供练习设置页提示题序范围上限。
func (s *Server) previewPractice(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BankID       int64    `json:"bankId"`
		BankIDs      []int64  `json:"bankIds"`
		Source       string   `json:"source"`
		QuestionType string   `json:"questionType"`
		Difficulty   int      `json:"difficulty"`
		Tags         []string `json:"tags"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if in.Source != "wrong" {
		in.Source = "bank"
	}
	if in.Source == "bank" && in.BankID > 0 && len(in.BankIDs) == 0 {
		in.BankIDs = []int64{in.BankID}
	}
	for _, id := range in.BankIDs {
		if id <= 0 {
			response.BadRequest(w, "题库编号无效")
			return
		}
		if _, ok := s.bankFor(w, r, id, accessRead); !ok {
			return
		}
	}
	user := userFrom(r)
	total, err := s.store.CountPracticeCandidates(user.ID, store.PracticeInput{
		BankIDs:      in.BankIDs,
		Source:       in.Source,
		QuestionType: in.QuestionType,
		Difficulty:   in.Difficulty,
		Tags:         in.Tags,
		Admin:        user.Role == "admin",
	})
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"candidates": total})
}

func (s *Server) listPracticeSessions(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	items, err := s.store.ListPracticeSessions(user.ID)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": items})
}

func (s *Server) getPracticeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "练习编号无效")
		return
	}
	user := userFrom(r)
	detail, err := s.store.GetPracticeSession(user.ID, id)
	if err != nil {
		if errors.Is(err, store.ErrPracticeNotFound) {
			response.NotFound(w, "练习不存在")
			return
		}
		response.Internal(w, err)
		return
	}
	response.OK(w, detail)
}

func (s *Server) answerPractice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "练习编号无效")
		return
	}
	var in struct {
		QuestionID  int64    `json:"questionId"`
		Chosen      []string `json:"chosen"`
		TimeSpentMs int      `json:"timeSpentMs"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if in.QuestionID <= 0 {
		response.BadRequest(w, "题目编号无效")
		return
	}
	user := userFrom(r)
	result, err := s.store.AnswerPractice(user.ID, id, in.QuestionID, in.Chosen, in.TimeSpentMs)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrPracticeNotFound):
			response.NotFound(w, "练习不存在")
		case errors.Is(err, store.ErrQuestionNotInSet):
			response.BadRequest(w, "题目不在本次练习中")
		default:
			response.BadRequest(w, err.Error())
		}
		return
	}
	response.OK(w, result)
}

func (s *Server) updatePracticePosition(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "练习编号无效")
		return
	}
	var in struct {
		Seq    int     `json:"seq"`
		Marked []int64 `json:"marked"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	user := userFrom(r)
	if err := s.store.UpdatePracticePosition(user.ID, id, in.Seq, in.Marked); err != nil {
		if errors.Is(err, store.ErrPracticeNotFound) {
			response.NotFound(w, "练习不存在")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}

func (s *Server) submitPractice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "练习编号无效")
		return
	}
	user := userFrom(r)
	summary, err := s.store.FinishPractice(user.ID, id)
	if err != nil {
		if errors.Is(err, store.ErrPracticeNotFound) {
			response.NotFound(w, "练习不存在")
			return
		}
		response.Internal(w, err)
		return
	}
	response.OK(w, summary)
}

func (s *Server) deletePracticeSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "练习编号无效")
		return
	}
	user := userFrom(r)
	if err := s.store.DeletePracticeSession(user.ID, id); err != nil {
		if errors.Is(err, store.ErrPracticeNotFound) {
			response.NotFound(w, "练习不存在")
			return
		}
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

func (s *Server) listWrong(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "pending" && status != "mastered" {
		status = "all"
	}
	user := userFrom(r)
	items, total, stats, err := s.store.ListWrong(user.ID, status, page, size)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": items, "total": total, "stats": stats})
}

func (s *Server) wrongStats(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	stats, err := s.store.WrongStatsOf(user.ID)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, stats)
}

func (s *Server) removeWrong(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题目编号无效")
		return
	}
	user := userFrom(r)
	if err := s.store.RemoveWrong(user.ID, id); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}
