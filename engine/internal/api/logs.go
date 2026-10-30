package api

import (
	"io"
	"net/http"
	"strings"
	"sync"

	"quizbox/engine/internal/response"
)

const logBufferLines = 400

type logBuffer struct {
	mu    sync.Mutex
	lines []string
}

var engineLogs = &logBuffer{}

// LogWriter 挂到 log.SetOutput，按行缓存最近的引擎日志供诊断页读取。
func LogWriter() io.Writer { return engineLogs }

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, line := range strings.Split(strings.TrimRight(string(p), "\r\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.lines = append(b.lines, truncateRunes(line, 2000))
	}
	if len(b.lines) > logBufferLines {
		b.lines = append([]string(nil), b.lines[len(b.lines)-logBufferLines:]...)
	}
	return len(p), nil
}

func (b *logBuffer) snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.lines...)
}

func (s *Server) systemLogs(w http.ResponseWriter, r *http.Request) {
	if userFrom(r).Role != "admin" {
		response.Forbidden(w, "只有管理员可以查看诊断日志")
		return
	}
	response.OK(w, map[string]any{
		"version": s.version,
		"dataDir": s.dataDir,
		"port":    s.port,
		"lines":   engineLogs.snapshot(),
	})
}
