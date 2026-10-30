package api

import (
	"net/http"

	"quizbox/engine/internal/response"
)

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Overview(userFrom(r))
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, data)
}
