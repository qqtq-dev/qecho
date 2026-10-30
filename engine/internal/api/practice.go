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
		BankID       int64  `json:"bankId"`
		Count        int    `json:"count"`
		Order        string `json:"order"`
		Source       string `json:"source"`
		QuestionType string `json:"questionType"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	user := userFrom(r)
	if in.Source != "wrong" {
		in.Source = "bank"
		if in.BankID <= 0 {
			response.BadRequest(w, "请选择题库")
			return
		}
		if _, ok := s.bankFor(w, r, in.BankID, false); !ok {
			return
		}
	} else if in.BankID > 0 {
		if _, ok := s.bankFor(w, r, in.BankID, false); !ok {
			return
		}
	}
	if in.Count <= 0 {
		in.Count = 20
	}
	session, items, err := s.store.CreatePracticeSession(user.ID, in.BankID, in.Count, in.Order, in.Source, in.QuestionType)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{
		"session":   session,
		"questions": items,
	})
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

func (s *Server) answerPractice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "练习编号无效")
		return
	}
	var in struct {
		QuestionID int64    `json:"questionId"`
		Chosen     []string `json:"chosen"`
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
	result, err := s.store.AnswerPractice(user.ID, id, in.QuestionID, in.Chosen)
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
