package api

import (
	"context"
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

type Server struct {
	store          *store.Store
	version        string
	dataDir        string
	shutdownToken  string
	port           int
	desktop        bool
	webDir         string
	onShutdown     func()
	importsMu      sync.Mutex
	pendingImports map[string]*pendingImport
	captchaMu      sync.Mutex
	captchas       map[string]captchaEntry
	rateMu         sync.Mutex
	rates          map[string]*rateBucket
	aiMu           sync.Mutex
	updateMu       sync.Mutex
	updateSnap     *updateSnapshot
}

func New(st *store.Store, version, dataDir, shutdownToken string, port int, desktop bool, webDir string, onShutdown func()) http.Handler {
	s := &Server{
		store:          st,
		version:        version,
		dataDir:        dataDir,
		shutdownToken:  shutdownToken,
		port:           port,
		desktop:        desktop,
		webDir:         webDir,
		onShutdown:     onShutdown,
		pendingImports: map[string]*pendingImport{},
		captchas:       map[string]captchaEntry{},
		rates:          map[string]*rateBucket{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ping", s.health)
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/ping", s.health)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/register", s.register)
	mux.HandleFunc("GET /api/v1/auth/captcha", s.captcha)
	mux.HandleFunc("POST /api/v1/auth/logout", s.auth(s.logout))
	mux.HandleFunc("GET /api/v1/auth/me", s.auth(s.me))
	mux.HandleFunc("PUT /api/v1/auth/password", s.auth(s.changePassword))
	mux.HandleFunc("GET /api/v1/users", s.auth(s.listUsers))
	mux.HandleFunc("GET /api/v1/users/search", s.auth(s.searchUsers))
	mux.HandleFunc("PUT /api/v1/users/{id}/role", s.auth(s.updateUserRole))
	mux.HandleFunc("PUT /api/v1/users/{id}/permissions", s.auth(s.updateUserPermissions))
	mux.HandleFunc("PUT /api/v1/users/{id}/status", s.auth(s.setUserStatus))
	mux.HandleFunc("POST /api/v1/users/{id}/password", s.auth(s.resetUserPassword))
	mux.HandleFunc("DELETE /api/v1/users/{id}", s.auth(s.deleteUser))
	mux.HandleFunc("GET /api/v1/system/status", s.auth(s.status))
	mux.HandleFunc("GET /api/v1/system/logs", s.auth(s.systemLogs))
	mux.HandleFunc("GET /api/v1/update/check", s.auth(s.checkUpdate))
	mux.HandleFunc("POST /api/v1/images", s.auth(s.uploadImage))
	mux.HandleFunc("GET /api/v1/images/{filename}", s.serveImage)
	mux.HandleFunc("GET /images/{filename}", s.serveImage)
	mux.HandleFunc("GET /api/v1/overview", s.auth(s.overview))
	mux.HandleFunc("GET /api/v1/stats", s.auth(s.stats))
	mux.HandleFunc("GET /api/v1/stats/tags", s.auth(s.statsTags))
	mux.HandleFunc("GET /api/v1/backups", s.auth(s.listBackups))
	mux.HandleFunc("POST /api/v1/backups", s.auth(s.createBackup))
	mux.HandleFunc("DELETE /api/v1/backups", s.auth(s.deleteBackup))
	mux.HandleFunc("POST /api/v1/backups/restore", s.auth(s.restoreBackup))
	mux.HandleFunc("DELETE /api/v1/backups/restore", s.auth(s.cancelRestore))
	mux.HandleFunc("POST /api/v1/system/shutdown", s.shutdown)

	mux.HandleFunc("GET /api/v1/banks", s.auth(s.listBanks))
	mux.HandleFunc("GET /api/v1/banks/{id}", s.auth(s.getBank))
	mux.HandleFunc("POST /api/v1/banks", s.auth(s.createBank))
	mux.HandleFunc("PUT /api/v1/banks/{id}", s.auth(s.updateBank))
	mux.HandleFunc("DELETE /api/v1/banks/{id}", s.auth(s.deleteBank))
	mux.HandleFunc("GET /api/v1/banks/{id}/export", s.auth(s.downloadBank))
	mux.HandleFunc("POST /api/v1/banks/{id}/export", s.auth(s.exportBank))
	mux.HandleFunc("GET /api/v1/banks/{id}/sharing", s.auth(s.getBankSharing))
	mux.HandleFunc("PUT /api/v1/banks/{id}/sharing", s.auth(s.updateBankSharing))
	mux.HandleFunc("PUT /api/v1/banks/{id}/members", s.auth(s.saveBankMember))
	mux.HandleFunc("DELETE /api/v1/banks/{id}/members/{userId}", s.auth(s.deleteBankMember))
	mux.HandleFunc("GET /api/v1/banks/{id}/export-images", s.auth(s.downloadBankImages))
	mux.HandleFunc("POST /api/v1/banks/{id}/export-images", s.auth(s.exportBankImages))
	mux.HandleFunc("GET /api/v1/banks/{id}/questions", s.auth(s.listQuestions))
	mux.HandleFunc("POST /api/v1/banks/{id}/questions", s.auth(s.createQuestion))
	mux.HandleFunc("GET /api/v1/banks/{id}/tags", s.auth(s.listBankTags))
	mux.HandleFunc("GET /api/v1/tags", s.auth(s.listTags))
	mux.HandleFunc("PUT /api/v1/tags/{id}", s.auth(s.renameTag))
	mux.HandleFunc("GET /api/v1/tags/{id}/impact", s.auth(s.tagImpact))
	mux.HandleFunc("POST /api/v1/tags/{id}/merge", s.auth(s.mergeTag))
	mux.HandleFunc("GET /api/v1/notices", s.auth(s.listNotices))
	mux.HandleFunc("POST /api/v1/notices/read", s.auth(s.readNotices))
	mux.HandleFunc("DELETE /api/v1/tags/{id}", s.auth(s.deleteTag))
	mux.HandleFunc("GET /api/v1/import-template", s.auth(s.downloadImportTemplate))
	mux.HandleFunc("POST /api/v1/import-template", s.auth(s.saveImportTemplate))
	mux.HandleFunc("POST /api/v1/banks/{id}/imports/preview", s.auth(s.previewImport))
	mux.HandleFunc("POST /api/v1/banks/{id}/imports", s.auth(s.commitImport))
	mux.HandleFunc("GET /api/v1/banks/{id}/imports", s.auth(s.listImportJobs))
	mux.HandleFunc("GET /api/v1/imports/{id}/errors", s.auth(s.importErrors))
	mux.HandleFunc("POST /api/v1/imports/{id}/revert", s.auth(s.revertImport))
	mux.HandleFunc("GET /api/v1/settings", s.auth(s.getSettings))
	mux.HandleFunc("PUT /api/v1/settings", s.auth(s.updateSettings))
	mux.HandleFunc("GET /api/v1/questions/{id}", s.auth(s.getQuestion))
	mux.HandleFunc("PUT /api/v1/questions/{id}", s.auth(s.updateQuestion))
	mux.HandleFunc("PATCH /api/v1/questions/{id}/analysis", s.auth(s.updateQuestionAnalysis))
	mux.HandleFunc("DELETE /api/v1/questions/{id}", s.auth(s.deleteQuestion))

	mux.HandleFunc("GET /api/v1/exams", s.auth(s.listExams))
	mux.HandleFunc("POST /api/v1/exams", s.auth(s.createExam))
	mux.HandleFunc("GET /api/v1/exams/{id}", s.auth(s.getExam))
	mux.HandleFunc("POST /api/v1/exams/{id}/close", s.auth(s.closeExam))
	mux.HandleFunc("DELETE /api/v1/exams/{id}", s.auth(s.deleteExam))
	mux.HandleFunc("GET /api/v1/exams/{id}/attempts", s.auth(s.listExamAttempts))
	mux.HandleFunc("GET /api/v1/exams/{id}/attempts/export", s.auth(s.downloadExamResults))
	mux.HandleFunc("POST /api/v1/exams/{id}/attempts/export", s.auth(s.exportExamResults))
	mux.HandleFunc("GET /api/v1/share", s.auth(s.shareInfo))
	mux.HandleFunc("PUT /api/v1/share", s.auth(s.updateShareSettings))

	mux.HandleFunc("GET /api/v1/practice/sessions", s.auth(s.listPracticeSessions))
	mux.HandleFunc("POST /api/v1/practice/preview", s.auth(s.previewPractice))
	mux.HandleFunc("POST /api/v1/practice/sessions", s.auth(s.createPractice))
	mux.HandleFunc("GET /api/v1/practice/sessions/{id}", s.auth(s.getPracticeSession))
	mux.HandleFunc("DELETE /api/v1/practice/sessions/{id}", s.auth(s.deletePracticeSession))
	mux.HandleFunc("POST /api/v1/practice/sessions/{id}/answers", s.auth(s.answerPractice))
	mux.HandleFunc("POST /api/v1/practice/sessions/{id}/position", s.auth(s.updatePracticePosition))
	mux.HandleFunc("POST /api/v1/practice/sessions/{id}/submit", s.auth(s.submitPractice))
	mux.HandleFunc("GET /api/v1/wrong", s.auth(s.listWrong))
	mux.HandleFunc("GET /api/v1/wrong/stats", s.auth(s.wrongStats))
	mux.HandleFunc("DELETE /api/v1/wrong/{id}", s.auth(s.removeWrong))

	mux.HandleFunc("GET /api/v1/ai/config", s.auth(s.aiConfig))
	mux.HandleFunc("PUT /api/v1/ai/config", s.auth(s.updateAIConfig))
	mux.HandleFunc("POST /api/v1/ai/test", s.auth(s.aiTest))
	mux.HandleFunc("GET /api/v1/ai/usage", s.auth(s.aiUsage))
	mux.HandleFunc("POST /api/v1/ai/chat", s.auth(s.aiChat))

	mux.HandleFunc("GET /api/public/exams/{token}", s.publicExamInfo)
	mux.HandleFunc("POST /api/public/exams/{token}/start", s.publicStart)
	mux.HandleFunc("POST /api/public/exams/{token}/answers", s.publicSaveAnswer)
	mux.HandleFunc("POST /api/public/exams/{token}/submit", s.publicSubmit)
	mux.HandleFunc("GET /api/public/exams/{token}/result", s.publicResult)
	mux.HandleFunc("GET /api/public/exams/{token}/attempt", s.publicResume)
	mux.HandleFunc("GET /e/{token}", s.participantPage)

	if s.webDir != "" {
		mux.Handle("GET /", spaHandler(s.webDir))
	}

	return withLogging(withCORS(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	sqliteOK := s.store.Ping() == nil
	status := "ok"
	if !sqliteOK {
		status = "degraded"
	}
	response.OK(w, map[string]any{
		"app":     "quizbox",
		"version": s.version,
		"status":  status,
		"sqlite":  sqliteOK,
	})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]any{
		"version":     s.version,
		"dataDir":     s.dataDir,
		"maintenance": false,
	})
}

func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	if s.shutdownToken == "" {
		response.NotFound(w, "未启用关闭接口")
		return
	}
	got := r.Header.Get("X-Shutdown-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(s.shutdownToken)) != 1 {
		response.Unauthorized(w, 40100, "关闭令牌无效")
		return
	}
	response.OK(w, nil)
	if s.onShutdown != nil {
		time.AfterFunc(50*time.Millisecond, s.onShutdown)
	}
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Token")
		if token == "" {
			response.Unauthorized(w, 40100, "未登录或会话已过期")
			return
		}
		user, ok := s.store.SessionUser(token)
		if !ok {
			response.Unauthorized(w, 40100, "未登录或会话已过期")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && s.store.RestorePending() {
			path := r.URL.Path
			if path != "/api/v1/auth/logout" && path != "/api/v1/backups/restore" {
				response.Forbidden(w, "数据恢复将在重启后生效，请先重启应用或取消恢复")
				return
			}
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	}
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func withCORS(next http.Handler) http.Handler {
	allowed := map[string]bool{
		"tauri://localhost":       true,
		"http://tauri.localhost":  true,
		"https://tauri.localhost": true,
		"http://localhost:54311":  true,
		"http://127.0.0.1:54311":  true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Token, X-Shutdown-Token")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Flush 透传给底层 ResponseWriter，SSE 流式响应依赖它把数据实时推给前端。
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, `{"code":50000,"message":"内部错误"}`, http.StatusInternalServerError)
			}
			elapsed := time.Since(start)
			if elapsed > 500*time.Millisecond {
				log.Printf("slow %s %s %d %s", r.Method, r.URL.Path, sw.status, elapsed)
			}
		}()
		next.ServeHTTP(sw, r)
	})
}

type ctxKey string

const userKey ctxKey = "user"
