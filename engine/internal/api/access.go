package api

import (
	"net/http"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

// 题库身份。admin / owner 可管理题库本身，editor 只能改题目内容，viewer 只读。
const (
	levelAdmin  = "admin"
	levelOwner  = "owner"
	levelEditor = store.LevelEditor
	levelViewer = store.LevelViewer
)

// bankAccess 表示调用方对题库的操作要求。
type bankAccess int

const (
	accessRead bankAccess = iota
	accessEdit
	accessManage
)

func userFrom(r *http.Request) *store.User {
	if v, ok := r.Context().Value(userKey).(*store.User); ok && v != nil {
		return v
	}
	return &store.User{Username: "local", Role: "admin"}
}

// applyBankAccess 按当前用户填充题库的 memberLevel / canEdit / canManage。
func applyBankAccess(u *store.User, b *store.Bank, memberLevel string) {
	level := memberLevel
	switch {
	case u.Role == "admin":
		level = levelAdmin
	case b.OwnerID != nil && *b.OwnerID == u.ID:
		level = levelOwner
	}
	b.MemberLevel = level
	b.CanEdit = level == levelAdmin || level == levelOwner || level == levelEditor
	b.CanManage = level == levelAdmin || level == levelOwner
}

// canReadBank 判断题库是否对已填充权限的用户可读：
// 公共题库所有人可读；个人题库看「对所有登录用户开放」或是否被加入共享。
func canReadBank(b *store.Bank) bool {
	if b.OwnerID == nil || b.Visibility == store.VisibilityPublic {
		return true
	}
	return b.MemberLevel != ""
}

// decorateBanks 批量填充列表的权限字段。
func (s *Server) decorateBanks(u *store.User, banks []store.Bank) error {
	levels := map[int64]string{}
	if u.Role != "admin" {
		got, err := s.store.BankMemberLevels(u.ID)
		if err != nil {
			return err
		}
		levels = got
	}
	for i := range banks {
		applyBankAccess(u, &banks[i], levels[banks[i].ID])
	}
	return nil
}

// bankFor 载入题库并校验权限，失败时已写出响应。
func (s *Server) bankFor(w http.ResponseWriter, r *http.Request, id int64, need bankAccess) (*store.Bank, bool) {
	bank, err := s.store.GetBank(id)
	if err != nil {
		response.NotFound(w, "题库不存在")
		return nil, false
	}
	user := userFrom(r)
	level := ""
	if user.Role != "admin" && (bank.OwnerID == nil || *bank.OwnerID != user.ID) {
		level, err = s.store.BankMemberLevel(id, user.ID)
		if err != nil {
			response.Internal(w, err)
			return nil, false
		}
	}
	applyBankAccess(user, bank, level)
	switch need {
	case accessManage:
		if !bank.CanManage {
			response.Forbidden(w, "只有题库所有者或管理员可以管理该题库")
			return nil, false
		}
	case accessEdit:
		if !bank.CanEdit {
			response.Forbidden(w, "没有权限修改该题库的题目")
			return nil, false
		}
	default:
		if !canReadBank(bank) {
			response.Forbidden(w, "没有权限查看该题库")
			return nil, false
		}
	}
	return bank, true
}

// requireExportImport 校验「导出/导入题库」能力位。
func requireExportImport(w http.ResponseWriter, r *http.Request) bool {
	if !userFrom(r).MayExportImport() {
		response.Forbidden(w, "管理员未开放导出与导入题库的权限")
		return false
	}
	return true
}

func canManageExam(u *store.User, ownerID int64) bool {
	return u.Role == "admin" || (ownerID != 0 && ownerID == u.ID)
}
