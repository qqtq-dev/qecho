package api

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"quizbox/engine/internal/exporter"
	"quizbox/engine/internal/response"
)

const importTemplateName = "Qecho-导入模板.xlsx"

// downloadImportTemplate 供浏览器 / 服务端部署时直接下载模板。
func (s *Server) downloadImportTemplate(w http.ResponseWriter, r *http.Request) {
	data, err := exporter.TemplateXLSX()
	if err != nil {
		response.Internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="qecho-import-template.xlsx"; filename*=UTF-8''%s`,
		url.PathEscape(importTemplateName)))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// saveImportTemplate 由本机桌面端调用：模板写入用户选择的路径。
func (s *Server) saveImportTemplate(w http.ResponseWriter, r *http.Request) {
	if !isLoopback(r.RemoteAddr) {
		response.Forbidden(w, "只能在安装本软件的电脑上保存模板，浏览器访问请直接下载")
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
	data, err := exporter.TemplateXLSX()
	if err != nil {
		response.Internal(w, err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		response.BadRequest(w, "写入文件失败："+err.Error())
		return
	}
	response.OK(w, map[string]any{"path": path})
}
