package api

import (
	"errors"
	"log"
	"net/http"
	"os"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以管理数据备份")
		return false
	}
	return true
}

func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	items, err := s.store.ListBackups()
	if err != nil {
		response.Internal(w, err)
		return
	}
	var total, auto, manual int64
	for _, item := range items {
		total += item.Size
		if item.Auto {
			auto += item.Size
		} else {
			manual += item.Size
		}
	}
	capBytes, retention := s.store.BackupPolicy()
	response.OK(w, map[string]any{
		"items":          items,
		"totalSize":      total,
		"autoSize":       auto,
		"manualSize":     manual,
		"capBytes":       capBytes,
		"retention":      retention,
		"backupsDir":     s.store.BackupsPath(),
		"pendingRestore": s.store.RestorePending(),
	})
}

func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	info, err := s.store.CreateBackup(store.BackupManual)
	if err != nil {
		log.Printf("create backup: %v", err)
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, info)
}

func (s *Server) deleteBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	name := r.URL.Query().Get("name")
	err := s.store.DeleteBackup(name)
	if errors.Is(err, os.ErrNotExist) {
		response.NotFound(w, "备份不存在")
		return
	}
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, nil)
}

func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	if err := s.store.PrepareRestore(in.Name); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{"pending": true})
}

func (s *Server) cancelRestore(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if err := s.store.CancelPendingRestore(); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, nil)
}
