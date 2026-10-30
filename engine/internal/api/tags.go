package api

import (
	"database/sql"
	"errors"
	"net/http"

	"quizbox/engine/internal/response"
)

func (s *Server) tagAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理标签")
		return 0, false
	}
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "标签编号无效")
		return 0, false
	}
	return id, true
}

func (s *Server) renameTag(w http.ResponseWriter, r *http.Request) {
	id, ok := s.tagAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if err := s.store.RenameTag(id, in.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(w, "标签不存在")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}

func (s *Server) mergeTag(w http.ResponseWriter, r *http.Request) {
	id, ok := s.tagAdmin(w, r)
	if !ok {
		return
	}
	var in struct {
		TargetID int64 `json:"targetId"`
	}
	if err := decodeJSON(r, &in); err != nil || in.TargetID <= 0 {
		response.BadRequest(w, "请选择要合并到的标签")
		return
	}
	if err := s.store.MergeTag(id, in.TargetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(w, "标签不存在")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}

func (s *Server) deleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := s.tagAdmin(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteTag(id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(w, "标签不存在")
			return
		}
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}
