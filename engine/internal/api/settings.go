package api

import (
	"net/http"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.GetSettings()
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, settings)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以修改系统设置")
		return
	}
	var in store.Settings
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	settings, err := s.store.UpdateSettings(in)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, settings)
}
