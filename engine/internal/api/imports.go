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
	"strconv"
	"strings"
	"time"

	"quizbox/engine/internal/importer"
	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

const (
	maxImportBytes = 20 << 20
	pendingTTL     = 2 * time.Hour
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
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes+1<<20)
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
	if _, ok := s.bankFor(w, r, bankID, true); !ok {
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
		"token":     token,
		"fileName":  fileName,
		"format":    result.Format,
		"sheet":     result.Sheet,
		"sheets":    result.Sheets,
		"headerRow": result.HeaderRow,
		"headers":   result.Headers,
		"mapping":   result.Mapping,
		"columns":   result.Columns,
		"total":     len(result.Rows),
		"valid":     valid,
		"duplicate": duplicate,
		"invalid":   invalid,
		"sample":    sample,
		"errors":    errorsOut,
		"warnings":  result.Warnings,
	})
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
	if _, ok := s.bankFor(w, r, bankID, true); !ok {
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
	if _, ok := s.bankFor(w, r, bankID, true); !ok {
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
	if _, ok := s.bankFor(w, r, bankID, false); !ok {
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
	if _, ok := s.bankFor(w, r, bankID, false); !ok {
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
