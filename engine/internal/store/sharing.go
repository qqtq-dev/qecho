package store

import (
	"database/sql"
	"fmt"
)

// 题库可见范围（banks.visibility）。
const (
	VisibilityPrivate = "private"
	VisibilityPublic  = "public"
)

// 共享级别（bank_members.level）。
const (
	LevelViewer = "viewer"
	LevelEditor = "editor"
)

// bankVisibleClause 返回「题库对 viewer 可见」的 SQL 条件及参数，alias 为题库表别名。
func bankVisibleClause(alias string, viewer *User) (string, []any) {
	if viewer == nil || viewer.Role == "admin" {
		return "1=1", nil
	}
	cond := fmt.Sprintf(`(%[1]s.owner_id IS NULL OR %[1]s.owner_id = ?
		OR %[1]s.visibility = 'public'
		OR EXISTS (SELECT 1 FROM bank_members bm
		            WHERE bm.bank_id = %[1]s.id AND bm.user_id = ?))`, alias)
	return cond, []any{viewer.ID, viewer.ID}
}

type BankMember struct {
	UserID    int64  `json:"userId"`
	Username  string `json:"username"`
	Level     string `json:"level"`
	CreatedAt string `json:"createdAt"`
}

func (s *Store) ListBankMembers(bankID int64) ([]BankMember, error) {
	rows, err := s.DB.Query(`
		SELECT m.user_id, u.username, m.level, m.created_at
		FROM bank_members m JOIN users u ON u.id = m.user_id
		WHERE m.bank_id = ? ORDER BY m.created_at, u.username`, bankID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BankMember{}
	for rows.Next() {
		var m BankMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.Level, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpsertBankMember 新增或调整共享级别，返回最新成员列表。
func (s *Store) UpsertBankMember(bankID, userID int64, level string) error {
	if level != LevelViewer && level != LevelEditor {
		return fmt.Errorf("共享级别只能是 viewer 或 editor")
	}
	_, err := s.DB.Exec(`
		INSERT INTO bank_members(bank_id, user_id, level, created_at)
		VALUES(?, ?, ?, ?)
		ON CONFLICT(bank_id, user_id) DO UPDATE SET level = excluded.level`,
		bankID, userID, level, nowString())
	return err
}

func (s *Store) RemoveBankMember(bankID, userID int64) error {
	_, err := s.DB.Exec("DELETE FROM bank_members WHERE bank_id = ? AND user_id = ?", bankID, userID)
	return err
}

func (s *Store) SetBankVisibility(bankID int64, visibility string) error {
	if visibility != VisibilityPrivate && visibility != VisibilityPublic {
		return fmt.Errorf("可见范围只能是 private 或 public")
	}
	_, err := s.DB.Exec(
		"UPDATE banks SET visibility = ? WHERE id = ? AND is_deleted = 0", visibility, bankID)
	return err
}

// BankMemberLevel 返回用户在题库中的共享级别，非成员返回空字符串。
func (s *Store) BankMemberLevel(bankID, userID int64) (string, error) {
	var level string
	err := s.DB.QueryRow(
		"SELECT level FROM bank_members WHERE bank_id = ? AND user_id = ?", bankID, userID).Scan(&level)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return level, err
}

// BankMemberLevels 返回用户在全部题库的共享级别，供列表批量填充权限。
func (s *Store) BankMemberLevels(userID int64) (map[int64]string, error) {
	rows, err := s.DB.Query(
		"SELECT bank_id, level FROM bank_members WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var bankID int64
		var level string
		if err := rows.Scan(&bankID, &level); err != nil {
			return nil, err
		}
		out[bankID] = level
	}
	return out, rows.Err()
}

// VisibleBankIDs 返回该用户当前可读的题库编号集合，供统计页屏蔽已失权题库的名称。
func (s *Store) VisibleBankIDs(viewer *User) (map[int64]bool, error) {
	cond, args := bankVisibleClause("b", viewer)
	rows, err := s.DB.Query("SELECT b.id FROM banks b WHERE b.is_deleted = 0 AND "+cond, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
