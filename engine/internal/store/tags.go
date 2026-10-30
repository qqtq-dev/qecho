package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Tag struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	QuestionCount int    `json:"questionCount"`
}

const maxQuestionTags = 10

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := map[string]bool{}
	for _, raw := range tags {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if len([]rune(name)) > 20 {
			name = string([]rune(name)[:20])
		}
		key := strings.ToLower(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
		if len(out) >= maxQuestionTags {
			break
		}
	}
	return out
}

func (s *Store) ListTags(viewer *User) ([]Tag, error) {
	query := `
		SELECT t.id, t.name, COUNT(DISTINCT q.id)
		FROM tags t
		LEFT JOIN question_tags qt ON qt.tag_id = t.id
		LEFT JOIN questions q ON q.id = qt.question_id AND q.is_deleted = 0
		LEFT JOIN banks b ON b.id = q.bank_id`
	where := []string{"1=1"}
	args := []any{}
	if viewer.Role != "admin" {
		where = append(where, "(b.id IS NULL OR b.owner_id IS NULL OR b.owner_id = ?)")
		args = append(args, viewer.ID)
	}
	query += " WHERE " + strings.Join(where, " AND ") + " GROUP BY t.id, t.name ORDER BY COUNT(DISTINCT q.id) DESC, t.name"
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.QuestionCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ListBankTags(bankID int64) ([]Tag, error) {
	rows, err := s.DB.Query(`
		SELECT t.id, t.name, COUNT(q.id)
		FROM tags t
		JOIN question_tags qt ON qt.tag_id = t.id
		JOIN questions q ON q.id = qt.question_id AND q.is_deleted = 0 AND q.bank_id = ?
		GROUP BY t.id, t.name ORDER BY COUNT(q.id) DESC, t.name`, bankID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.QuestionCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) setQuestionTags(tx *sql.Tx, questionID int64, names []string) error {
	if _, err := tx.Exec("DELETE FROM question_tags WHERE question_id = ?", questionID); err != nil {
		return err
	}
	for _, name := range normalizeTags(names) {
		var id int64
		err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&id)
		if err == sql.ErrNoRows {
			res, err := tx.Exec("INSERT INTO tags(name) VALUES(?)", name)
			if err != nil {
				return err
			}
			id, _ = res.LastInsertId()
		} else if err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT OR IGNORE INTO question_tags(question_id, tag_id) VALUES(?, ?)", questionID, id,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadQuestionTags(q *Question) error {
	rows, err := s.DB.Query(`
		SELECT t.name FROM question_tags qt JOIN tags t ON t.id = qt.tag_id
		WHERE qt.question_id = ? ORDER BY t.name`, q.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	q.Tags = []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		q.Tags = append(q.Tags, name)
	}
	return rows.Err()
}

// deleteOrphanTags 清理已没有任何题目的标签。
func (s *Store) deleteOrphanTags() {
	_, _ = s.DB.Exec(`DELETE FROM tags WHERE id NOT IN (SELECT DISTINCT tag_id FROM question_tags)`)
}

func (s *Store) RenameTag(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("标签名称不能为空")
	}
	if len([]rune(name)) > 20 {
		return fmt.Errorf("标签名称不能超过 20 个字")
	}
	var existing int64
	err := s.DB.QueryRow("SELECT id FROM tags WHERE name = ? COLLATE NOCASE", name).Scan(&existing)
	if err == nil && existing != id {
		return fmt.Errorf("已存在同名标签")
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	res, err := s.DB.Exec("UPDATE tags SET name = ? WHERE id = ?", name, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("已存在同名标签")
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func tagExists(tx *sql.Tx, id int64) error {
	var got int64
	if err := tx.QueryRow("SELECT id FROM tags WHERE id = ?", id).Scan(&got); err != nil {
		return err
	}
	return nil
}

// MergeTag 把 source 下的题目改挂到 target，再删除 source。
func (s *Store) MergeTag(sourceID, targetID int64) error {
	if sourceID == targetID {
		return fmt.Errorf("不能合并到自身")
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tagExists(tx, sourceID); err != nil {
		return err
	}
	if err := tagExists(tx, targetID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO question_tags(question_id, tag_id)
		SELECT question_id, ? FROM question_tags WHERE tag_id = ?`, targetID, sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM question_tags WHERE tag_id = ?", sourceID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tags WHERE id = ?", sourceID); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteTag 只解除题目标签关联，不影响题目本身。
func (s *Store) DeleteTag(id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tagExists(tx, id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM question_tags WHERE tag_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM tags WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
