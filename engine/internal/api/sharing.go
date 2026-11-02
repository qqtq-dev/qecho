package api

import (
	"net/http"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

// sharingState 是共享抽屉需要的全部数据。
type sharingState struct {
	BankID     int64              `json:"bankId"`
	Visibility string             `json:"visibility"`
	OwnerID    *int64             `json:"ownerId"`
	OwnerName  string             `json:"ownerName"`
	Members    []store.BankMember `json:"members"`
}

func (s *Server) sharingState(bank *store.Bank) (*sharingState, error) {
	members, err := s.store.ListBankMembers(bank.ID)
	if err != nil {
		return nil, err
	}
	return &sharingState{
		BankID:     bank.ID,
		Visibility: bank.Visibility,
		OwnerID:    bank.OwnerID,
		OwnerName:  bank.OwnerName,
		Members:    members,
	}, nil
}

// sharingBank 载入题库并确认可管理、且不是公共题库（公共题库由管理员直接维护）。
func (s *Server) sharingBank(w http.ResponseWriter, r *http.Request) (*store.Bank, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return nil, false
	}
	bank, ok := s.bankFor(w, r, id, accessManage)
	if !ok {
		return nil, false
	}
	if bank.OwnerID == nil {
		response.BadRequest(w, "公共题库由管理员统一维护，无需共享设置")
		return nil, false
	}
	return bank, true
}

func (s *Server) getBankSharing(w http.ResponseWriter, r *http.Request) {
	bank, ok := s.sharingBank(w, r)
	if !ok {
		return
	}
	state, err := s.sharingState(bank)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, state)
}

func (s *Server) updateBankSharing(w http.ResponseWriter, r *http.Request) {
	bank, ok := s.sharingBank(w, r)
	if !ok {
		return
	}
	var in struct {
		Visibility string `json:"visibility"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if in.Visibility != store.VisibilityPrivate && in.Visibility != store.VisibilityPublic {
		response.BadRequest(w, "可见范围只能是 private 或 public")
		return
	}
	if err := s.store.SetBankVisibility(bank.ID, in.Visibility); err != nil {
		response.Internal(w, err)
		return
	}
	bank.Visibility = in.Visibility
	state, err := s.sharingState(bank)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, state)
}

func (s *Server) saveBankMember(w http.ResponseWriter, r *http.Request) {
	bank, ok := s.sharingBank(w, r)
	if !ok {
		return
	}
	var in struct {
		UserID int64  `json:"userId"`
		Level  string `json:"level"`
	}
	if err := decodeJSON(r, &in); err != nil || in.UserID <= 0 {
		response.BadRequest(w, "请选择要共享的用户")
		return
	}
	if in.Level == "" {
		in.Level = store.LevelViewer
	}
	if in.Level != store.LevelViewer && in.Level != store.LevelEditor {
		response.BadRequest(w, "共享级别只能是 viewer 或 editor")
		return
	}
	if bank.OwnerID != nil && *bank.OwnerID == in.UserID {
		response.BadRequest(w, "题库所有者无需共享给自己")
		return
	}
	target, err := s.store.GetUserByID(in.UserID)
	if err != nil {
		response.NotFound(w, "用户不存在")
		return
	}
	if target.Disabled {
		response.BadRequest(w, "该账号已停用")
		return
	}
	if target.Role == "admin" {
		response.BadRequest(w, "管理员本就可见全部题库，无需共享")
		return
	}
	before, err := s.store.BankMemberLevel(bank.ID, in.UserID)
	if err != nil {
		response.Internal(w, err)
		return
	}
	if err := s.store.UpsertBankMember(bank.ID, in.UserID, in.Level); err != nil {
		response.Internal(w, err)
		return
	}
	levelText := "只读"
	if in.Level == store.LevelEditor {
		levelText = "可编辑"
	}
	bankID := bank.ID
	switch {
	case before == "":
		s.notify(in.UserID, "share-added", "题库《"+bank.Name+"》已共享给你（"+levelText+"）", &bankID)
	case before != in.Level:
		s.notify(in.UserID, "share-changed", "题库《"+bank.Name+"》的共享权限已改为"+levelText, &bankID)
	}
	state, err := s.sharingState(bank)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, state)
}

func (s *Server) deleteBankMember(w http.ResponseWriter, r *http.Request) {
	bank, ok := s.sharingBank(w, r)
	if !ok {
		return
	}
	userID, ok := pathID(r, "userId")
	if !ok {
		response.BadRequest(w, "用户编号无效")
		return
	}
	if err := s.store.RemoveBankMember(bank.ID, userID); err != nil {
		response.Internal(w, err)
		return
	}
	bankID := bank.ID
	s.notify(userID, "share-removed", "题库《"+bank.Name+"》已取消与你共享", &bankID)
	state, err := s.sharingState(bank)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, state)
}
