package api

import (
	_ "embed"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed web/e.html
var participantPage []byte

// pageOverridePath 返回参与页的外部覆盖文件路径。
// 优先级：环境变量 QUIZBOX_PAGE > 数据目录下的 e.html；两者都没有就只用内嵌版本。
func (s *Server) pageOverridePath() string {
	if p := os.Getenv("QUIZBOX_PAGE"); p != "" {
		return p
	}
	if s.dataDir == "" {
		return ""
	}
	return filepath.Join(s.dataDir, "e.html")
}

// participantPage 输出答题参与者页。
// 内嵌版本保证开箱即用；若存在外部文件则优先读它，改版式只要换文件、不必重编引擎。
func (s *Server) participantPage(w http.ResponseWriter, r *http.Request) {
	body := participantPage
	if p := s.pageOverridePath(); p != "" {
		data, err := os.ReadFile(p)
		switch {
		case err == nil && len(data) > 0:
			body = data
		case err != nil && !os.IsNotExist(err):
			log.Printf("participant page override %s unreadable, fallback to embedded: %v", p, err)
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(body)
}
