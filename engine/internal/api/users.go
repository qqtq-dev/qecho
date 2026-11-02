package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

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

// updateUserPermissions 设置普通用户的「建题库」「导出导入题库」能力位。
func (s *Server) updateUserPermissions(w http.ResponseWriter, r *http.Request) {
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
		CanCreateBank   bool `json:"canCreateBank"`
		CanExportImport bool `json:"canExportImport"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if _, err := s.store.GetUserByID(id); errors.Is(err, sql.ErrNoRows) {
		response.NotFound(w, "用户不存在")
		return
	} else if err != nil {
		response.Internal(w, err)
		return
	}
	if err := s.store.SetUserPermissions(id, in.CanCreateBank, in.CanExportImport); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

// setUserStatus 停用或启用账号。停用会踢掉已有会话，题库与历史数据都保留；
// 与删除不同，停用是可逆的，用于「先冻结、观察一段时间再决定是否删除」。
func (s *Server) setUserStatus(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r)
	if me.Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理用户")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "用户编号无效")
		return
	}
	var in struct {
		Disabled bool `json:"disabled"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if id == me.ID {
		response.BadRequest(w, "不能停用当前登录的账号")
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
		response.BadRequest(w, "本机账号不能停用")
		return
	}
	if in.Disabled && rec.Role == "admin" {
		admins, err := s.store.CountAdmins()
		if err != nil {
			response.Internal(w, err)
			return
		}
		if admins <= 1 {
			response.BadRequest(w, "至少需要保留一名可用的管理员")
			return
		}
	}
	if err := s.store.SetUserDisabled(id, in.Disabled); err != nil {
		response.Internal(w, err)
		return
	}
	if in.Disabled {
		_ = s.store.RevokeUserSessions(id)
	}
	response.OK(w, nil)
}

// deleteUser 删除账号；删除前自动备份，个人题库一并软删、外联考试关闭。
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	me := userFrom(r)
	if me.Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理用户")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "用户编号无效")
		return
	}
	if id == me.ID {
		response.BadRequest(w, "不能删除当前登录的账号")
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
		response.BadRequest(w, "本机账号不能删除")
		return
	}
	if rec.Role == "admin" {
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
	if _, err := s.store.CreateBackup(store.BackupPreDelete); err != nil {
		response.BadRequest(w, "删除前自动备份失败，已取消删除："+err.Error())
		return
	}
	if err := s.store.DeleteUser(id); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}

// searchUsers 供题库所有者在共享时挑选用户，只暴露编号、用户名与角色。
// 共享题库时挑选用户的搜索下限与条数上限。
// 下限取 2 是为了不让「输入一个字就列出大半账号」成为可用能力；
// 共享场景下正常用户本来就会打出两个以上的字符。
const (
	userSearchMinRunes = 2
	userSearchLimit    = 10
)

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if utf8.RuneCountInString(keyword) < userSearchMinRunes {
		response.OK(w, map[string]any{"items": []any{}})
		return
	}
	if utf8.RuneCountInString(keyword) > 20 {
		keyword = string([]rune(keyword)[:20])
	}
	users, err := s.store.SearchUsers(keyword, userSearchLimit)
	if err != nil {
		response.Internal(w, err)
		return
	}
	items := make([]map[string]any, 0, len(users))
	for _, u := range users {
		items = append(items, map[string]any{"id": u.ID, "username": u.Username, "role": u.Role})
	}
	response.OK(w, map[string]any{"items": items})
}
