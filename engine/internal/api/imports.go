package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"quizbox/engine/internal/importer"
	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

const (
	maxImportBytes        = 20 << 20
	maxImportImagesBytes  = 100 << 20
	maxImportImageCount   = 300
	maxImportRequestBytes = maxImportBytes + maxImportImagesBytes + 1<<20
	pendingTTL            = 2 * time.Hour
)

type pendingImport struct {
	bankID   int64
	fileName string
	format   string
	rows     []importer.Row
	expires  time.Time
}

type previewRow struct {
	Row     int                    `json:"row"`
	Type    string                 `json:"type"`
	Stem    string                 `json:"stem"`
	Options []store.QuestionOption `json:"options"`
	Answer  []string               `json:"answer"`
}

func newImportToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func readImportFile(w http.ResponseWriter, r *http.Request) (string, []byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportRequestBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return "", nil, fmt.Errorf("文件过大或上传格式不正确")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return "", nil, fmt.Errorf("请选择要导入的文件")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImportBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("读取文件失败")
	}
	if len(data) > maxImportBytes {
		return "", nil, fmt.Errorf("文件不能超过 20 MB")
	}
	return header.Filename, data, nil
}

func previewOptions(r *http.Request) importer.Options {
	opts := importer.Options{}
	if raw := strings.TrimSpace(r.FormValue("options")); raw != "" {
		_ = json.Unmarshal([]byte(raw), &opts)
	}
	return opts
}

func (s *Server) previewImport(w http.ResponseWriter, r *http.Request) {
	bankID, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	if _, ok := s.bankFor(w, r, bankID, accessEdit); !ok {
		return
	}
	if !requireExportImport(w, r) {
		return
	}
	fileName, data, err := readImportFile(w, r)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	result, err := importer.ParseWithOptions(fileName, data, previewOptions(r))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	imageUploaded, imageMatched, imageWarnings, err := s.attachImportImages(r, result)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	result.Warnings = append(result.Warnings, imageWarnings...)
	hashes, err := s.store.QuestionHashes(bankID)
	if err != nil {
		response.Internal(w, err)
		return
	}

	type rowError struct {
		Row     int    `json:"row"`
		Message string `json:"message"`
	}
	errorsOut := []rowError{}
	sample := []previewRow{}
	seen := map[string]bool{}
	valid, duplicate, invalid := 0, 0, 0
	for _, row := range result.Rows {
		if row.Err != "" {
			invalid++
			if len(errorsOut) < 200 {
				errorsOut = append(errorsOut, rowError{Row: row.Number, Message: row.Err})
			}
			continue
		}
		hash := store.HashQuestion(&row.Input)
		if hashes[hash] || seen[hash] {
			duplicate++
			continue
		}
		seen[hash] = true
		valid++
		if len(sample) < 5 {
			sample = append(sample, previewRow{
				Row:     row.Number,
				Type:    row.Input.Type,
				Stem:    row.Input.Stem,
				Options: row.Input.Options,
				Answer:  row.Input.Answer,
			})
		}
	}

	token, err := newImportToken()
	if err != nil {
		response.Internal(w, err)
		return
	}
	s.importsMu.Lock()
	for key, item := range s.pendingImports {
		if time.Now().After(item.expires) {
			delete(s.pendingImports, key)
		}
	}
	s.pendingImports[token] = &pendingImport{
		bankID:   bankID,
		fileName: truncateRunes(fileName, 200),
		format:   result.Format,
		rows:     result.Rows,
		expires:  time.Now().Add(pendingTTL),
	}
	s.importsMu.Unlock()

	response.OK(w, map[string]any{
		"token":         token,
		"fileName":      fileName,
		"format":        result.Format,
		"sheet":         result.Sheet,
		"sheets":        result.Sheets,
		"headerRow":     result.HeaderRow,
		"headers":       result.Headers,
		"mapping":       result.Mapping,
		"columns":       result.Columns,
		"total":         len(result.Rows),
		"valid":         valid,
		"duplicate":     duplicate,
		"invalid":       invalid,
		"sample":        sample,
		"errors":        errorsOut,
		"warnings":      result.Warnings,
		"imageUploaded": imageUploaded,
		"imageMatched":  imageMatched,
	})
}

// attachImportImages 保存随表格一起上传的题图，并按文件名把题干的图片引用替换为存储名。
func (s *Server) attachImportImages(r *http.Request, result *importer.Result) (uploaded, matched int, warnings []string, err error) {
	imageMap := map[string]string{}
	if r.MultipartForm != nil {
		files := r.MultipartForm.File["images"]
		if len(files) > maxImportImageCount {
			return 0, 0, nil, fmt.Errorf("一次最多上传 %d 张题图", maxImportImageCount)
		}
		var totalBytes int64
		for _, fh := range files {
			if fh.Size > store.MaxImageBytes {
				return 0, 0, nil, fmt.Errorf("题图 %s 超过 8 MB", fh.Filename)
			}
			totalBytes += fh.Size
			if totalBytes > maxImportImagesBytes {
				return 0, 0, nil, fmt.Errorf("题图总大小不能超过 100 MB")
			}
			file, openErr := fh.Open()
			if openErr != nil {
				return 0, 0, nil, fmt.Errorf("读取题图 %s 失败", fh.Filename)
			}
			data, readErr := io.ReadAll(io.LimitReader(file, store.MaxImageBytes+1))
			_ = file.Close()
			if readErr != nil || len(data) > store.MaxImageBytes {
				return 0, 0, nil, fmt.Errorf("读取题图 %s 失败", fh.Filename)
			}
			img, saveErr := s.store.SaveImage(data, fh.Filename)
			if saveErr != nil {
				return 0, 0, nil, fmt.Errorf("题图 %s：%s", fh.Filename, saveErr.Error())
			}
			uploaded++
			key := importImageKey(fh.Filename)
			if _, exists := imageMap[key]; !exists {
				imageMap[key] = img.Filename
			}
		}
	}

	missingRows := 0
	missingNames := []string{}
	seenMissing := map[string]bool{}
	localRefRows := 0
	for i := range result.Rows {
		ref := strings.TrimSpace(result.Rows[i].Input.StemImage)
		if ref == "" || strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			continue
		}
		localRefRows++
		key := importImageKey(ref)
		if mapped, ok := imageMap[key]; ok {
			result.Rows[i].Input.StemImage = mapped
			matched++
			continue
		}
		missingRows++
		if !seenMissing[key] && len(missingNames) < 5 {
			seenMissing[key] = true
			missingNames = append(missingNames, filepath.Base(strings.ReplaceAll(ref, "\\", "/")))
		}
	}

	if missingRows > 0 {
		hint := "请在导入前选择对应的题图文件"
		if uploaded == 0 {
			hint = "题干图片列引用了本地文件名，但本次没有选择题图文件"
		}
		warnings = append(warnings, fmt.Sprintf(
			"%d 道题的题图未匹配到文件（如 %s）；%s，导入后再补充上传",
			missingRows, strings.Join(missingNames, "、"), hint))
	} else if localRefRows > 0 && matched > 0 {
		warnings = append(warnings, fmt.Sprintf("已按文件名关联 %d 道题的题图", matched))
	}
	return uploaded, matched, warnings, nil
}

func importImageKey(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	return strings.ToLower(filepath.Base(name))
}

func (s *Server) commitImport(w http.ResponseWriter, r *http.Request) {
	bankID, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	var in struct {
		Token  string `json:"token"`
		Policy string `json:"policy"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	s.importsMu.Lock()
	pending := s.pendingImports[in.Token]
	if pending != nil {
		delete(s.pendingImports, in.Token)
	}
	s.importsMu.Unlock()
	if pending == nil || time.Now().After(pending.expires) {
		response.BadRequest(w, "导入预览已过期，请重新上传文件")
		return
	}
	if pending.bankID != bankID {
		response.BadRequest(w, "导入目标与预览时不一致，请重新上传")
		return
	}
	if _, ok := s.bankFor(w, r, bankID, accessEdit); !ok {
		return
	}
	if !requireExportImport(w, r) {
		return
	}
	if _, err := s.store.CreateBackup(store.BackupPreImport); err != nil {
		log.Printf("pre-import backup: %v", err)
		response.BadRequest(w, "导入前自动备份失败，已取消本次导入")
		return
	}
	items := make([]store.ImportItem, len(pending.rows))
	for i, row := range pending.rows {
		items[i] = store.ImportItem{Row: row.Number, Input: row.Input, Err: row.Err}
	}
	outcome, err := s.store.ImportQuestions(bankID, pending.fileName, pending.format, items, in.Policy)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, outcome)
}

func (s *Server) revertImport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "导入编号无效")
		return
	}
	bankID, err := s.store.ImportJobBankID(id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(w, "导入记录不存在")
		return
	}
	if err != nil {
		response.Internal(w, err)
		return
	}
	if _, ok := s.bankFor(w, r, bankID, accessEdit); !ok {
		return
	}
	if !requireExportImport(w, r) {
		return
	}
	count, kept, err := s.store.RevertImport(id)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{"reverted": count, "kept": kept})
}

func (s *Server) listImportJobs(w http.ResponseWriter, r *http.Request) {
	bankID, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	if _, ok := s.bankFor(w, r, bankID, accessRead); !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	jobs, err := s.store.ListImportJobs(bankID, limit)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": jobs})
}

// importErrors 返回某批次的错误明细；format=csv 时直接下载。
func (s *Server) importErrors(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "导入编号无效")
		return
	}
	bankID, err := s.store.ImportJobBankID(id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(w, "导入记录不存在")
		return
	}
	if err != nil {
		response.Internal(w, err)
		return
	}
	if _, ok := s.bankFor(w, r, bankID, accessRead); !ok {
		return
	}
	rows, err := s.store.ImportJobErrors(id)
	if err != nil {
		response.Internal(w, err)
		return
	}
	if r.URL.Query().Get("format") == "csv" {
		var buf strings.Builder
		buf.WriteString("\ufeff")
		cw := csv.NewWriter(&buf)
		_ = cw.Write([]string{"行号", "错误原因"})
		for _, item := range rows {
			_ = cw.Write([]string{strconv.Itoa(item.Row), item.Message})
		}
		cw.Flush()
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(
			`attachment; filename="import-%d-errors.csv"`, id))
		_, _ = io.WriteString(w, buf.String())
		return
	}
	response.OK(w, map[string]any{"items": rows})
}

func truncateRunes(s string, limit int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
