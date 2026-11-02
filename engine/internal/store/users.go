package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	CreatedAt   string `json:"createdAt"`
	LastLoginAt string `json:"lastLoginAt"`
	// 能力位由管理员授予，管理员始终具备（见 MayCreateBank / MayExportImport）。
	CanCreateBank   bool `json:"canCreateBank"`
	CanExportImport bool `json:"canExportImport"`
}

// MayCreateBank 表示是否允许新建个人题库。
func (u *User) MayCreateBank() bool {
	return u.Role == "admin" || u.CanCreateBank
}

// MayExportImport 表示是否允许导出、导入题库内容。
func (u *User) MayExportImport() bool {
	return u.Role == "admin" || u.CanExportImport
}

// applyEffective 让管理员在接口里也表现为具备能力位，前端无需再判断角色。
func (u *User) applyEffective() {
	if u.Role == "admin" {
		u.CanCreateBank = true
		u.CanExportImport = true
	}
}

type UserRecord struct {
	User
	PasswordSalt string
	PasswordHash string
	Disabled     bool
}

func (s *Store) EnsureLocalUser() (*User, error) {
	rec, err := s.GetUserByUsername("local")
	if err == nil {
		return &rec.User, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return s.CreateUser("local", "", "admin")
}

func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	return n, err
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.DB.Query(`
		SELECT id, username, role, created_at, COALESCE(last_login_at, ''),
		       can_create_bank, can_export_import
		FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.LastLoginAt,
			&u.CanCreateBank, &u.CanExportImport); err != nil {
			return nil, err
		}
		u.applyEffective()
		out = append(out, u)
	}
	return out, rows.Err()
}

// SearchUsers 供共享题库时挑选用户，只返回启用账号的编号与用户名。
func (s *Store) SearchUsers(keyword string, limit int) ([]User, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return []User{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.DB.Query(`
		SELECT id, username, role, created_at, COALESCE(last_login_at, ''),
		       can_create_bank, can_export_import
		FROM users
		WHERE is_disabled = 0 AND username LIKE ? ESCAPE '\'
		ORDER BY username LIMIT ?`, "%"+escapeLike(keyword)+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt, &u.LastLoginAt,
			&u.CanCreateBank, &u.CanExportImport); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// escapeLike 转义 LIKE 通配符，避免用户名里的 % 和 _ 变成模糊匹配。
func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)
	return replacer.Replace(s)
}

// SetUserPermissions 保存管理员授予的能力位。
func (s *Store) SetUserPermissions(id int64, canCreateBank, canExportImport bool) error {
	res, err := s.DB.Exec(
		"UPDATE users SET can_create_bank = ?, can_export_import = ? WHERE id = ?",
		canCreateBank, canExportImport, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) SetUserRole(id int64, role string) error {
	res, err := s.DB.Exec("UPDATE users SET role = ? WHERE id = ?", role, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateUserPassword(id int64, password string) error {
	salt, hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	res, err := s.DB.Exec(
		"UPDATE users SET password_salt = ?, password_hash = ? WHERE id = ?", salt, hash, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RevokeUserSessions(id int64) error {
	_, err := s.DB.Exec("DELETE FROM sessions WHERE user_id = ?", id)
	return err
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND is_disabled = 0").Scan(&n)
	return n, err
}

func (s *Store) CreateUser(username, password, role string) (*User, error) {
	username = strings.TrimSpace(username)
	salt, hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	now := nowString()
	res, err := s.DB.Exec(`
		INSERT INTO users(username, password_salt, password_hash, role, created_at)
		VALUES(?, ?, ?, ?, ?)`, username, salt, hash, role, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("用户名已被注册")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, Role: role, CreatedAt: now}, nil
}

func (s *Store) GetUserByUsername(username string) (*UserRecord, error) {
	return s.scanUser(`
		SELECT id, username, role, is_disabled, created_at, COALESCE(last_login_at,''),
		       password_salt, password_hash, can_create_bank, can_export_import
		FROM users WHERE username = ?`, strings.TrimSpace(username))
}

func (s *Store) GetUserByID(id int64) (*UserRecord, error) {
	return s.scanUser(`
		SELECT id, username, role, is_disabled, created_at, COALESCE(last_login_at,''),
		       password_salt, password_hash, can_create_bank, can_export_import
		FROM users WHERE id = ?`, id)
}

func (s *Store) scanUser(query string, arg any) (*UserRecord, error) {
	var rec UserRecord
	err := s.DB.QueryRow(query, arg).Scan(
		&rec.ID, &rec.Username, &rec.Role, &rec.Disabled, &rec.CreatedAt, &rec.LastLoginAt,
		&rec.PasswordSalt, &rec.PasswordHash, &rec.CanCreateBank, &rec.CanExportImport,
	)
	if err != nil {
		return nil, err
	}
	rec.applyEffective()
	return &rec, nil
}

func (s *Store) TouchLogin(id int64) error {
	_, err := s.DB.Exec("UPDATE users SET last_login_at = ? WHERE id = ?", nowString(), id)
	return err
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	rec, err := s.GetUserByUsername(username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("用户名或密码错误")
		}
		return nil, err
	}
	if rec.Disabled {
		return nil, fmt.Errorf("账号已被停用")
	}
	if rec.PasswordHash == "" || !VerifyPassword(rec.PasswordSalt, rec.PasswordHash, password) {
		return nil, fmt.Errorf("用户名或密码错误")
	}
	_ = s.TouchLogin(rec.ID)
	return &rec.User, nil
}

func (s *Store) CreateSession(token string, user *User, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.Exec(
		"INSERT INTO sessions(token, user_id, username, created_at, expires_at) VALUES(?, ?, ?, ?, ?)",
		token, user.ID, user.Username, now, expiresAt.UTC().Format(time.RFC3339Nano),
	)
	return err
}

// DeleteUser 删除账号并清理其个人数据：个人题库软删、外联考试关闭、练习与错题清除。
// 考试与成绩记录保留，便于管理员事后查阅。
func (s *Store) DeleteUser(id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowString()
	steps := []struct {
		query string
		args  []any
	}{
		{`UPDATE questions SET is_deleted = 1, updated_at = ?
		  WHERE bank_id IN (SELECT id FROM banks WHERE owner_id = ?)`, []any{now, id}},
		{`UPDATE banks SET is_deleted = 1, updated_at = ? WHERE owner_id = ?`, []any{now, id}},
		{`UPDATE exams SET status = 'closed', updated_at = ?
		  WHERE owner_id = ? AND status <> 'closed'`, []any{now, id}},
		{`DELETE FROM practice_sessions WHERE user_id = ?`, []any{id}},
		{`DELETE FROM wrong_book WHERE user_id = ?`, []any{id}},
		{`DELETE FROM sessions WHERE user_id = ?`, []any{id}},
		{`DELETE FROM bank_members WHERE user_id = ?`, []any{id}},
		{`DELETE FROM users WHERE id = ?`, []any{id}},
	}
	for _, step := range steps {
		if _, err := tx.Exec(step.query, step.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SessionUser(token string) (*User, bool) {
	var user User
	var expires string
	err := s.DB.QueryRow(`
		SELECT u.id, u.username, u.role, u.created_at, COALESCE(u.last_login_at,''), s.expires_at,
		       u.can_create_bank, u.can_export_import
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND u.is_disabled = 0`, token,
	).Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt, &user.LastLoginAt, &expires,
		&user.CanCreateBank, &user.CanExportImport)
	if err != nil {
		return nil, false
	}
	exp, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || time.Now().After(exp) {
		_, _ = s.DB.Exec("DELETE FROM sessions WHERE token = ?", token)
		return nil, false
	}
	user.applyEffective()
	return &user, true
}
