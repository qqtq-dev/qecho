package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"quizbox/engine/internal/bicqa"
	"quizbox/engine/internal/imaweb"
	"quizbox/engine/internal/response"
)

var errStopStream = errors.New("stop stream")

// 回答长度约束。BIC-QA 是推理型模型，思考时长与「要求它讲多少」强相关：
// 同一道复杂题，要求 200 字以内时思考量约为不限长度的四成，首字时间从 70 秒降到 15 秒左右。
// 因此默认走简答，需要展开时由前端切到详答。
const (
	answerHintBrief = "\n\n回答要求：请用不超过 200 字简要回答，先给出结论，再对每个选项用一句话说明对错原因，不要展开推导过程。"
	answerHintFull  = "\n\n回答要求：请逐一说明每个选项为什么对或错，并补充相关知识点。"
)

// withAnswerMode 按回答长度模式追加约束。mode 为空或 brief 时走简答。
func withAnswerMode(question, mode string) string {
	if mode == "full" {
		return question + answerHintFull
	}
	return question + answerHintBrief
}

// aiConfigOf 读取指定用户自己的 BIC-QA 配置（含明文 Key，仅限服务端使用）。
// Key 按账号隔离存放，普通用户无需管理员身份即可配置自己的额度。
func (s *Server) aiConfigOf(userID int64) bicqa.Config {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()
	return bicqa.Load(s.dataDir, userID)
}

func (s *Server) saveAIConfigOf(userID int64, cfg bicqa.Config) error {
	s.aiMu.Lock()
	defer s.aiMu.Unlock()
	return bicqa.Save(s.dataDir, userID, cfg)
}

func aiConfigView(cfg bicqa.Config) map[string]any {
	return map[string]any{
		"provider":   cfg.ProviderName(),
		"enabled":    cfg.Enabled,
		"hasKey":     strings.TrimSpace(cfg.APIKey) != "",
		"keyMask":    cfg.MaskedKey(),
		"hasCookie":  strings.TrimSpace(cfg.Cookie) != "",
		"cookieMask": cfg.MaskedCookie(),
		"baseUrl":    cfg.BaseURL,
		"model":      cfg.Model,
		"timeoutSec": cfg.TimeoutSec,
		"ready":      cfg.Ready(),
	}
}

func (s *Server) aiConfig(w http.ResponseWriter, r *http.Request) {
	response.OK(w, aiConfigView(s.aiConfigOf(userFrom(r).ID)))
}

func (s *Server) updateAIConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Provider    string `json:"provider"`
		Enabled     *bool  `json:"enabled"`
		BaseURL     string `json:"baseUrl"`
		APIKey      string `json:"apiKey"`
		Model       string `json:"model"`
		TimeoutSec  int    `json:"timeoutSec"`
		Cookie      string `json:"cookie"`
		ClearKey    bool   `json:"clearKey"`
		ClearCookie bool   `json:"clearCookie"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	userID := userFrom(r).ID
	cfg := s.aiConfigOf(userID)
	prevProvider := cfg.ProviderName()

	if p := strings.ToLower(strings.TrimSpace(in.Provider)); p != "" {
		switch p {
		case bicqa.ProviderBICQA:
			cfg.Provider = bicqa.ProviderBICQA
		case bicqa.ProviderImaWeb, "ima", "ima-web":
			cfg.Provider = bicqa.ProviderImaWeb
		default:
			response.BadRequest(w, "不支持的接入方式")
			return
		}
	}
	// 换接入方式时，如果服务地址还是上一个接入方式的默认值，就一并换掉；
	// 用户自己填过的地址（例如走代理）保持不动。
	if cfg.ProviderName() != prevProvider && strings.TrimSpace(in.BaseURL) == "" && cfg.BaseURL == providerDefaultBaseURL(prevProvider) {
		cfg.BaseURL = ""
	}
	if in.Enabled != nil {
		cfg.Enabled = *in.Enabled
	}
	if strings.TrimSpace(in.BaseURL) != "" {
		base := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			response.BadRequest(w, "服务地址需以 http:// 或 https:// 开头")
			return
		}
		cfg.BaseURL = base
	}
	// 留空表示沿用已保存的凭据，避免前端拿不到明文又无法只改其他字段。
	if in.ClearKey {
		cfg.APIKey = ""
	} else if key := strings.TrimSpace(in.APIKey); key != "" {
		cfg.APIKey = key
	}
	if in.ClearCookie {
		cfg.Cookie = ""
	} else if ck := normalizeCookie(in.Cookie); ck != "" {
		cfg.Cookie = ck
	}
	cfg.Model = strings.TrimSpace(in.Model)
	if in.TimeoutSec > 0 {
		cfg.TimeoutSec = in.TimeoutSec
	}
	if err := s.saveAIConfigOf(userID, cfg); err != nil {
		response.Internal(w, err)
		return
	}
	// 保存会补全默认值，回显也得用补全后的副本，否则前端拿到空的服务地址。
	response.OK(w, aiConfigView(cfg.Normalized()))
}

// normalizeCookie 清洗从浏览器复制来的 Cookie 请求头：
// 去掉可能带上的 "Cookie:" 前缀、换行和多余空格，否则会原样发给服务端导致鉴权失败。
func normalizeCookie(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if len(s) > 7 && strings.EqualFold(s[:7], "cookie:") {
		s = strings.TrimSpace(s[7:])
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

func providerDefaultBaseURL(provider string) string {
	if provider == bicqa.ProviderImaWeb {
		return bicqa.ImaWebDefaultBaseURL
	}
	return bicqa.DefaultBaseURL
}

// aiSession 把两种接入方式的差异收敛成三个函数：建会话、发起流式问答、
// 解析流。这样限流、SSE 转发、错误兜底这些上层逻辑只写一份，
// 将来再加第三种接入方式也只需补一个构造函数分支。
type aiSession struct {
	provider string
	label    string
	open     func(ctx context.Context, title string) (string, error)
	stream   func(ctx context.Context, question, sessionID string) (io.ReadCloser, error)
	parse    func(body io.Reader, emit func(bicqa.Event) error) error
}

func aiSessionOf(cfg bicqa.Config) aiSession {
	if cfg.ProviderName() == bicqa.ProviderImaWeb {
		client := imaweb.NewClient(cfg.BaseURL, cfg.Cookie)
		return aiSession{
			provider: bicqa.ProviderImaWeb,
			label:    "ima 网页版",
			// ima 的会话标题由服务端决定，这里忽略 title。
			open:   func(ctx context.Context, _ string) (string, error) { return client.CreateSession(ctx) },
			stream: client.ChatStream,
			parse:  imaweb.ParseStream,
		}
	}
	client := bicqa.NewClient(cfg)
	return aiSession{
		provider: bicqa.ProviderBICQA,
		label:    "BIC-QA",
		open:     client.CreateSession,
		stream:   client.ChatStream,
		parse:    bicqa.ParseStream,
	}
}

// aiTest 通过建会话做一次真实连通性与鉴权自检。
func (s *Server) aiTest(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiConfigOf(userFrom(r).ID)
	sess := aiSessionOf(cfg)
	if !cfg.Ready() {
		if sess.provider == bicqa.ProviderImaWeb {
			response.BadRequest(w, "请先填写 ima 网页版 Cookie 并打开开关")
		} else {
			response.BadRequest(w, "请先填写 BIC-QA API Key")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	id, err := sess.open(ctx, "题鸣连接测试")
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{
		"ok":             true,
		"provider":       sess.provider,
		"conversationId": id,
		"baseUrl":        cfg.BaseURL,
	})
}

func (s *Server) aiUsage(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiConfigOf(userFrom(r).ID)
	if cfg.ProviderName() == bicqa.ProviderImaWeb {
		// ima 网页版没有额度查询接口，额度在 ima 自己的产品页里看。
		response.BadRequest(w, "ima 网页版不提供额度查询，请到 ima 客户端或网页查看")
		return
	}
	if !cfg.Ready() {
		response.BadRequest(w, "请先在设置页配置并启用 BIC-QA")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	data, err := bicqa.NewClient(cfg).Usage(ctx)
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if len(data) == 0 {
		response.OK(w, map[string]any{})
		return
	}
	response.OK(w, data)
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, kind string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{}`)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, raw)
	flusher.Flush()
}

// aiChat 代理 BIC-QA 的流式问答，把统一后的 delta / error / done 事件推给前端。
// 走引擎代理而不是前端直连的原因：绕开 CORS，并让 API Key 只留在本机。
func (s *Server) aiChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Question       string `json:"question"`
		ConversationID string `json:"conversationId"`
		Title          string `json:"title"`
		Mode           string `json:"mode"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	question := strings.TrimSpace(in.Question)
	if question == "" {
		response.BadRequest(w, "提问内容不能为空")
		return
	}
	if runes := []rune(question); len(runes) > bicqa.MaxQuestionRunes {
		question = string(runes[:bicqa.MaxQuestionRunes])
	}
	question = withAnswerMode(question, in.Mode)

	cfg := s.aiConfigOf(userFrom(r).ID)
	sess := aiSessionOf(cfg)
	if !cfg.Ready() {
		if sess.provider == bicqa.ProviderImaWeb {
			response.BadRequest(w, "尚未启用 ima 网页版：请在设置页填入 Cookie 并打开开关")
		} else {
			response.BadRequest(w, "尚未启用 BIC-QA：请在设置页填写 API Key 并打开开关")
		}
		return
	}
	// 提问会消耗额度（BIC-QA 按 token，ima 按账号次数），按用户限流，
	// 避免误触发把额度打空。
	if user := userFrom(r); !s.allowRate(fmt.Sprintf("ai-chat:%d", user.ID), 20, 5*time.Minute) {
		response.TooManyRequests(w, "提问过于频繁，请稍后再试")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		response.Internal(w, errors.New("当前连接不支持流式响应"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.TimeoutSec)*time.Second)
	defer cancel()

	client := sess
	convID := strings.TrimSpace(in.ConversationID)
	if convID == "" {
		id, err := client.open(ctx, in.Title)
		if err != nil {
			writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
			writeSSE(w, flusher, "done", map[string]any{})
			return
		}
		convID = id
	}
	writeSSE(w, flusher, "meta", map[string]string{"conversationId": convID, "provider": sess.provider})

	body, err := client.stream(ctx, question, convID)
	if err != nil {
		writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
		writeSSE(w, flusher, "done", map[string]any{})
		return
	}
	defer body.Close()

	sentDone := false
	finish := func(message string) {
		if message != "" {
			writeSSE(w, flusher, "error", map[string]string{"message": message})
		}
		writeSSE(w, flusher, "done", map[string]any{})
		sentDone = true
	}
	streamErr := sess.parse(body, func(ev bicqa.Event) error {
		switch ev.Kind {
		case "delta":
			writeSSE(w, flusher, "delta", map[string]string{"text": ev.Text})
		case "thinking":
			writeSSE(w, flusher, "thinking", map[string]string{"text": ev.Text})
		case "error":
			finish(ev.Text)
			return errStopStream
		case "done":
			finish("")
			return errStopStream
		}
		return nil
	})
	if !sentDone {
		if streamErr != nil && !errors.Is(streamErr, errStopStream) && ctx.Err() == nil {
			finish(streamErr.Error())
		} else {
			finish("")
		}
	}
}
