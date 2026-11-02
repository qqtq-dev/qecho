package store

// Notice 是站内通知，目前只在「共享关系变化」时写入。
type Notice struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Text      string `json:"text"`
	BankID    *int64 `json:"bankId"`
	CreatedAt string `json:"createdAt"`
	Read      bool   `json:"read"`
}

// noticeKeepPerUser 每个用户最多保留的通知条数，避免长期使用后无限增长。
const noticeKeepPerUser = 50

// AddNotice 写入一条通知，并顺带清理该用户的旧通知。
func (s *Store) AddNotice(userID int64, kind, text string, bankID *int64) error {
	if userID <= 0 {
		return nil
	}
	if _, err := s.DB.Exec(`
		INSERT INTO user_notices(user_id, kind, text, bank_id, created_at)
		VALUES(?, ?, ?, ?, ?)`, userID, kind, text, bankID, nowString()); err != nil {
		return err
	}
	_, err := s.DB.Exec(`
		DELETE FROM user_notices
		WHERE user_id = ? AND id NOT IN (
			SELECT id FROM user_notices WHERE user_id = ? ORDER BY id DESC LIMIT ?)`,
		userID, userID, noticeKeepPerUser)
	return err
}

// ListNotices 返回最近的通知与未读数。
func (s *Store) ListNotices(userID int64, limit int) ([]Notice, int, error) {
	if limit <= 0 || limit > noticeKeepPerUser {
		limit = noticeKeepPerUser
	}
	rows, err := s.DB.Query(`
		SELECT id, kind, text, bank_id, created_at, read_at IS NOT NULL
		FROM user_notices WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Notice{}
	for rows.Next() {
		var n Notice
		var bankID interface{}
		if err := rows.Scan(&n.ID, &n.Kind, &n.Text, &bankID, &n.CreatedAt, &n.Read); err != nil {
			return nil, 0, err
		}
		if v, ok := bankID.(int64); ok {
			id := v
			n.BankID = &id
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var unread int
	if err := s.DB.QueryRow(
		"SELECT COUNT(*) FROM user_notices WHERE user_id = ? AND read_at IS NULL", userID).Scan(&unread); err != nil {
		return nil, 0, err
	}
	return out, unread, nil
}

func (s *Store) MarkNoticesRead(userID int64) error {
	_, err := s.DB.Exec(
		"UPDATE user_notices SET read_at = ? WHERE user_id = ? AND read_at IS NULL", nowString(), userID)
	return err
}
