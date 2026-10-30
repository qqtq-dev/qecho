package api

import (
	"database/sql"
	"errors"
	"net/http"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

func validNewPassword(password string) bool {
	return len(password) >= 8 && len(password) <= 72
}

// changePassword 供当前登录用户修改自己的密码；不注销当前会话。
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	user := userFrom(r)
	rec, err := s.store.GetUserByID(user.ID)
	if err != nil {
		response.Internal(w, err)
		return
	}
	if rec.PasswordHash == "" {
		if in.OldPassword != "" {
			response.BadRequest(w, "当前密码不正确")
			return
		}
	} else if !store.VerifyPassword(rec.PasswordSalt, rec.PasswordHash, in.OldPassword) {
		response.BadRequest(w, "当前密码不正确")
		return
	}
	if !validNewPassword(in.NewPassword) {
		response.BadRequest(w, "新密码长度需为 8–72 位")
		return
	}
	if in.NewPassword == in.OldPassword {
		response.BadRequest(w, "新密码不能与当前密码相同")
		return
	}
	if err := s.store.UpdateUserPassword(user.ID, in.NewPassword); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理用户")
		return
	}
	users, err := s.store.ListUsers()
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": users})
}

func (s *Server) updateUserRole(w http.ResponseWriter, r *http.Request) {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理用户")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "用户编号无效")
		return
	}
	var in struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &in); err != nil || (in.Role != "admin" && in.Role != "user") {
		response.BadRequest(w, "角色需为 admin 或 user")
		return
	}
	rec, err := s.store.GetUserByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(w, "用户不存在")
		return
	}
	if err != nil {
		response.Internal(w, err)
		return
	}
	if rec.Username == "local" {
		response.BadRequest(w, "本机账号固定为管理员，不能修改角色")
		return
	}
	if rec.Role == "admin" && in.Role == "user" {
		admins, err := s.store.CountAdmins()
		if err != nil {
			response.Internal(w, err)
			return
		}
		if admins <= 1 {
			response.BadRequest(w, "至少需要保留一名管理员")
			return
		}
	}
	if err := s.store.SetUserRole(id, in.Role); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

func (s *Server) resetUserPassword(w http.ResponseWriter, r *http.Request) {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理用户")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "用户编号无效")
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if !validNewPassword(in.Password) {
		response.BadRequest(w, "密码长度需为 8–72 位")
		return
	}
	rec, err := s.store.GetUserByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(w, "用户不存在")
		return
	}
	if err != nil {
		response.Internal(w, err)
		return
	}
	if rec.Username == "local" {
		response.BadRequest(w, "本机账号请通过“修改密码”自行设置")
		return
	}
	if err := s.store.UpdateUserPassword(id, in.Password); err != nil {
		response.Internal(w, err)
		return
	}
	if id != userFrom(r).ID {
		_ = s.store.RevokeUserSessions(id)
	}
	response.OK(w, nil)
}
