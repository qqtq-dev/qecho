package api

import (
	"net/http"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

func userFrom(r *http.Request) *store.User {
	if v, ok := r.Context().Value(userKey).(*store.User); ok && v != nil {
		return v
	}
	return &store.User{Username: "local", Role: "admin"}
}

func canReadBank(u *store.User, b *store.Bank) bool {
	if u.Role == "admin" || b.OwnerID == nil {
		return true
	}
	return *b.OwnerID == u.ID
}

func canWriteBank(u *store.User, b *store.Bank) bool {
	if u.Role == "admin" {
		return true
	}
	return b.OwnerID != nil && *b.OwnerID == u.ID
}

// bankFor 载入题库并校验可见/可写权限，失败时已写出响应。
func (s *Server) bankFor(w http.ResponseWriter, r *http.Request, id int64, write bool) (*store.Bank, bool) {
	bank, err := s.store.GetBank(id)
	if err != nil {
		response.NotFound(w, "题库不存在")
		return nil, false
	}
	user := userFrom(r)
	if write {
		if !canWriteBank(user, bank) {
			response.Forbidden(w, "没有权限修改该题库")
			return nil, false
		}
	} else if !canReadBank(user, bank) {
		response.Forbidden(w, "没有权限查看该题库")
		return nil, false
	}
	return bank, true
}

func canManageExam(u *store.User, ownerID int64) bool {
	return u.Role == "admin" || (ownerID != 0 && ownerID == u.ID)
}
