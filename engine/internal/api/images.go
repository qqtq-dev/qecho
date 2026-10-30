package api

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, store.MaxImageBytes+1<<20)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		response.BadRequest(w, "上传失败：文件过大或表单格式不正确")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, "请选择图片文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, store.MaxImageBytes+1))
	if err != nil {
		response.BadRequest(w, "读取图片失败")
		return
	}
	if len(data) > store.MaxImageBytes {
		response.BadRequest(w, "图片不能超过 8 MB")
		return
	}
	img, err := s.store.SaveImage(data, header.Filename)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, img)
}

func (s *Server) serveImage(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(strings.TrimSpace(r.PathValue("filename")))
	path, err := s.store.ImagePath(name)
	if err != nil {
		response.NotFound(w, "图片不存在")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}
