package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"quizbox/engine/internal/exporter"
	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

const maxJSONBody = 1 << 20

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody))
	return dec.Decode(v)
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) listBanks(w http.ResponseWriter, r *http.Request) {
	banks, err := s.store.ListBanks(userFrom(r))
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": banks})
}

func (s *Server) getBank(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	bank, ok := s.bankFor(w, r, id, false)
	if !ok {
		return
	}
	response.OK(w, bank)
}

func (s *Server) createBank(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Public      bool   `json:"public"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	user := userFrom(r)
	var owner *int64
	if in.Public {
		if user.Role != "admin" {
			response.Forbidden(w, "只有管理员可以创建公共题库")
			return
		}
	} else {
		owner = &user.ID
	}
	bank, err := s.store.CreateBank(in.Name, in.Description, owner)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, bank)
}

func (s *Server) updateBank(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	if _, ok := s.bankFor(w, r, id, true); !ok {
		return
	}
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if err := s.store.UpdateBank(id, in.Name, in.Description); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	bank, err := s.store.GetBank(id)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, bank)
}

func (s *Server) deleteBank(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	if _, ok := s.bankFor(w, r, id, true); !ok {
		return
	}
	if err := s.store.DeleteBank(id); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}

// exportBank 由本机桌面端调用：题目写入用户选择的路径。
func (s *Server) exportBank(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	bank, ok := s.bankFor(w, r, id, false)
	if !ok {
		return
	}
	if !isLoopback(r.RemoteAddr) {
		response.Forbidden(w, "只能在安装本软件的电脑上导出，浏览器访问请直接下载")
		return
	}
	var in struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		response.BadRequest(w, "请选择保存位置")
		return
	}
	if !strings.HasSuffix(strings.ToLower(path), ".xlsx") {
		path += ".xlsx"
	}
	questions, err := s.store.ExportQuestions(id)
	if err != nil {
		response.Internal(w, err)
		return
	}
	if len(questions) == 0 {
		response.BadRequest(w, "这个题库还没有题目")
		return
	}
	data, err := exporter.BankXLSX(questions)
	if err != nil {
		response.Internal(w, err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		response.BadRequest(w, "写入文件失败："+err.Error())
		return
	}
	response.OK(w, map[string]any{"path": path, "count": len(questions), "name": bank.Name})
}

// downloadBank 供浏览器 / 服务端部署时直接下载导出文件。
func (s *Server) downloadBank(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	bank, ok := s.bankFor(w, r, id, false)
	if !ok {
		return
	}
	questions, err := s.store.ExportQuestions(id)
	if err != nil {
		response.Internal(w, err)
		return
	}
	if len(questions) == 0 {
		response.BadRequest(w, "这个题库还没有题目")
		return
	}
	data, err := exporter.BankXLSX(questions)
	if err != nil {
		response.Internal(w, err)
		return
	}
	fileName := safeFileName(bank.Name) + ".xlsx"
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="bank-%d.xlsx"; filename*=UTF-8''%s`, id, url.PathEscape(fileName)))
	w.Header().Set("X-Export-Count", strconv.Itoa(len(questions)))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	replacer := strings.NewReplacer(
		`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_",
		`"`, "_", "<", "_", ">", "_", "|", "_",
	)
	name = replacer.Replace(name)
	if name == "" {
		name = "题库"
	}
	return truncateRunes(name, 60)
}

func (s *Server) listQuestions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	if _, ok := s.bankFor(w, r, id, false); !ok {
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	items, total, err := s.store.ListQuestions(id, page, size)
	if err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, map[string]any{"items": items, "total": total})
}

func (s *Server) createQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题库编号无效")
		return
	}
	if _, ok := s.bankFor(w, r, id, true); !ok {
		return
	}
	var in store.QuestionInput
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	q, err := s.store.CreateQuestion(id, in)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, q)
}

func (s *Server) updateQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题目编号无效")
		return
	}
	q, err := s.store.GetQuestion(id)
	if err != nil {
		response.NotFound(w, "题目不存在")
		return
	}
	if _, ok := s.bankFor(w, r, q.BankID, true); !ok {
		return
	}
	var in store.QuestionInput
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	q, err = s.store.UpdateQuestion(id, in)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, q)
}

func (s *Server) deleteQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		response.BadRequest(w, "题目编号无效")
		return
	}
	q, err := s.store.GetQuestion(id)
	if err != nil {
		response.NotFound(w, "题目不存在")
		return
	}
	if _, ok := s.bankFor(w, r, q.BankID, true); !ok {
		return
	}
	if err := s.store.DeleteQuestion(id); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}
