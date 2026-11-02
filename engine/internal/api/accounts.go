package api

import (
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"quizbox/engine/internal/authx"
	"quizbox/engine/internal/response"
	"quizbox/engine/internal/store"
)

const sessionTTL = 7 * 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_.-]{2,20}$`)

type captchaEntry struct {
	code    string
	expires time.Time
}

type rateBucket struct {
	count int
	reset time.Time
}

func (s *Server) issueSession(w http.ResponseWriter, user *store.User) {
	token, err := authx.NewToken()
	if err != nil {
		response.Internal(w, err)
		return
	}
	if err := s.store.CreateSession(token, user, time.Now().Add(sessionTTL)); err != nil {
		response.Internal(w, err)
		return
	}
	_ = s.store.PruneSessions()
	response.OK(w, map[string]any{
		"token": token, "userId": user.ID, "username": user.Username, "role": user.Role,
		"canCreateBank": user.MayCreateBank(), "canExportImport": user.MayExportImport(),
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&in)
	}
	in.Username = strings.TrimSpace(in.Username)

	// 桌面壳的本机静默登录：只在 --desktop 且来源为回环地址时生效，
	// 避免反向代理部署后被人用 local/空密码绕过。
	if s.desktop && isLoopback(r.RemoteAddr) &&
		(in.Username == "" || in.Username == "local") && in.Password == "" {
		user, err := s.store.EnsureLocalUser()
		if err != nil {
			response.Internal(w, err)
			return
		}
		s.issueSession(w, user)
		return
	}

	if !s.allowRate("login:"+clientIP(r), 10, 5*time.Minute) {
		response.TooManyRequests(w, "尝试过于频繁，请稍后再试")
		return
	}
	user, err := s.store.Authenticate(in.Username, in.Password)
	if err != nil {
		response.Unauthorized(w, 40101, err.Error())
		return
	}
	s.issueSession(w, user)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.allowRate("register:"+clientIP(r), 5, 10*time.Minute) {
		response.TooManyRequests(w, "注册过于频繁，请稍后再试")
		return
	}
	var in struct {
		Username      string `json:"username"`
		Password      string `json:"password"`
		CaptchaID     string `json:"captchaId"`
		CaptchaAnswer string `json:"captchaAnswer"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	in.Username = strings.TrimSpace(in.Username)
	if !s.checkCaptcha(in.CaptchaID, in.CaptchaAnswer) {
		response.BadRequest(w, "验证码不正确或已过期")
		return
	}
	if strings.EqualFold(in.Username, "local") {
		response.BadRequest(w, "该用户名为系统保留")
		return
	}
	if !usernamePattern.MatchString(in.Username) {
		response.BadRequest(w, "用户名需为 2–20 位字母、数字、汉字或 _ . -")
		return
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		response.BadRequest(w, "密码长度需为 8–72 位")
		return
	}
	role := "user"
	if admins, err := s.store.CountAdmins(); err == nil && admins == 0 {
		role = "admin"
	}
	user, err := s.store.CreateUser(in.Username, in.Password, role)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	s.issueSession(w, user)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r)
	response.OK(w, map[string]any{
		"id": user.ID, "username": user.Username, "role": user.Role, "desktop": s.desktop,
		"canCreateBank": user.MayCreateBank(), "canExportImport": user.MayExportImport(),
	})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if token := r.Header.Get("X-Token"); token != "" {
		_ = s.store.DeleteSession(token)
	}
	response.OK(w, nil)
}

func (s *Server) allowRate(key string, limit int, window time.Duration) bool {
	now := time.Now()
	s.rateMu.Lock()
	defer s.rateMu.Unlock()
	item, ok := s.rates[key]
	if !ok || now.After(item.reset) {
		s.rates[key] = &rateBucket{count: 1, reset: now.Add(window)}
		return true
	}
	if item.count >= limit {
		return false
	}
	item.count++
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && isLoopback(r.RemoteAddr) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}
	}
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
