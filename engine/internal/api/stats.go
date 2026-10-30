package api

import (
	"net/http"

	"quizbox/engine/internal/response"
)

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Stats(userFrom(r), r.URL.Query().Get("range"))
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, data)
}

func (s *Server) statsTags(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.StatsTags(userFrom(r), r.URL.Query().Get("range"))
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": items})
}
