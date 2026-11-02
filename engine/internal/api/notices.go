package api

import (
	"net/http"

	"quizbox/engine/internal/response"
)

func (s *Server) listNotices(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	items, unread, err := s.store.ListNotices(user.ID, 20)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": items, "unread": unread})
}

func (s *Server) readNotices(w http.ResponseWriter, r *http.Request) {
	if err := s.store.MarkNoticesRead(userFrom(r).ID); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

// notify 写一条站内通知；失败不影响主流程。
func (s *Server) notify(userID int64, kind, text string, bankID *int64) {
	_ = s.store.AddNotice(userID, kind, text, bankID)
}
