package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxImageBytes 是单张图片的上限，与导入规范保持一致。
const MaxImageBytes = 8 << 20

var imageFilenameRe = regexp.MustCompile(`^[0-9a-f]{64}\.(png|jpg|jpeg|webp|gif)$`)

var imageExtByMime = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type Image struct {
	Filename     string `json:"filename"`
	OriginalName string `json:"originalName"`
	Mime         string `json:"mime"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	CreatedAt    string `json:"createdAt"`
	Deduped      bool   `json:"deduped"`
}

// SaveImage 按内容寻址存储图片：同名内容只落一份文件、只记一行。
func (s *Store) SaveImage(data []byte, originalName string) (*Image, error) {
	if len(data) == 0 {
		return nil, errors.New("图片内容为空")
	}
	if len(data) > MaxImageBytes {
		return nil, fmt.Errorf("图片不能超过 %d MB", MaxImageBytes>>20)
	}
	mime := http.DetectContentType(data)
	ext, ok := imageExtByMime[mime]
	if !ok {
		return nil, errors.New("只支持 PNG、JPEG、WebP、GIF 图片")
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	filename := digest + ext
	dir := s.imagesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建图片目录失败：%w", err)
	}

	img, err := s.imageBySHA(digest)
	if err == nil {
		if err := writeImageFile(filepath.Join(dir, img.Filename), data); err != nil {
			return nil, err
		}
		img.Deduped = true
		return img, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if err := writeImageFile(filepath.Join(dir, filename), data); err != nil {
		return nil, err
	}
	res, err := s.DB.Exec(`
		INSERT INTO images(filename, original_name, sha256, mime, size, created_at)
		VALUES(?, ?, ?, ?, ?, ?)
		ON CONFLICT(sha256) DO NOTHING`,
		filename, cleanOriginalName(originalName), digest, mime, int64(len(data)), nowString(),
	)
	if err != nil {
		return nil, err
	}
	img, err = s.imageBySHA(digest)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		img.Deduped = true
	}
	return img, nil
}

type BankImage struct {
	QuestionID int64  `json:"questionId"`
	Location   string `json:"location"`
	Filename   string `json:"filename"`
}

// BankImages 返回题库中引用的本地图片文件，按题目顺序排列。
func (s *Store) BankImages(bankID int64) ([]BankImage, error) {
	rows, err := s.DB.Query(`
		SELECT q.id, '题干', q.stem_image FROM questions q
		WHERE q.bank_id = ? AND q.is_deleted = 0 AND q.stem_image IS NOT NULL AND q.stem_image <> ''
		UNION ALL
		SELECT q.id, '选项 ' || o.key, o.image FROM questions q
		JOIN question_options o ON o.question_id = q.id
		WHERE q.bank_id = ? AND q.is_deleted = 0 AND o.image IS NOT NULL AND o.image <> ''
		ORDER BY 1`, bankID, bankID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BankImage{}
	for rows.Next() {
		var item BankImage
		if err := rows.Scan(&item.QuestionID, &item.Location, &item.Filename); err != nil {
			return nil, err
		}
		if strings.HasPrefix(item.Filename, "http://") || strings.HasPrefix(item.Filename, "https://") {
			continue
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) imageBySHA(digest string) (*Image, error) {
	var img Image
	err := s.DB.QueryRow(`
		SELECT filename, original_name, sha256, mime, size, created_at
		FROM images WHERE sha256 = ?`, digest,
	).Scan(&img.Filename, &img.OriginalName, &img.SHA256, &img.Mime, &img.Size, &img.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// ImagePath 只接受 SHA-256 命名的位图文件，避免路径穿越。
func (s *Store) ImagePath(filename string) (string, error) {
	if !imageFilenameRe.MatchString(filename) {
		return "", errors.New("图片不存在")
	}
	path := filepath.Join(s.imagesDir(), filename)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", errors.New("图片不存在")
	}
	return path, nil
}

func (s *Store) imagesDir() string {
	return filepath.Join(s.Dir, "images")
}

func writeImageFile(path string, data []byte) error {
	if info, err := os.Stat(path); err == nil {
		if !info.IsDir() && info.Size() == int64(len(data)) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("替换图片失败：%w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("写入图片失败：%w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存图片失败：%w", err)
	}
	return nil
}

func cleanOriginalName(name string) string {
	name = filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	if name == "." || name == ".." {
		return ""
	}
	if utf8.RuneCountInString(name) > 120 {
		name = string([]rune(name)[:120])
	}
	return name
}

// NormalizeImageRef 允许空值、历史 http(s) 链接，或安全的文件名。
func NormalizeImageRef(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	if len(v) > 500 {
		return "", errors.New("图片地址过长")
	}
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v, nil
	}
	v = strings.ReplaceAll(v, "\\", "/")
	if strings.Contains(v, "/") || strings.Contains(v, "..") {
		return "", errors.New("图片只接受文件名，不能包含路径")
	}
	return v, nil
}
