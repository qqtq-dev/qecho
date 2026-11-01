package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"quizbox/engine/internal/bicqa"
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
		"enabled":    cfg.Enabled,
		"hasKey":     strings.TrimSpace(cfg.APIKey) != "",
		"keyMask":    cfg.MaskedKey(),
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
		Enabled    *bool  `json:"enabled"`
		BaseURL    string `json:"baseUrl"`
		APIKey     string `json:"apiKey"`
		Model      string `json:"model"`
		TimeoutSec int    `json:"timeoutSec"`
		ClearKey   bool   `json:"clearKey"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	userID := userFrom(r).ID
	cfg := s.aiConfigOf(userID)
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
	// 留空表示沿用已保存的 Key，避免前端拿不到明文又无法只改其他字段。
	if in.ClearKey {
		cfg.APIKey = ""
	} else if key := strings.TrimSpace(in.APIKey); key != "" {
		cfg.APIKey = key
	}
	cfg.Model = strings.TrimSpace(in.Model)
	if in.TimeoutSec > 0 {
		cfg.TimeoutSec = in.TimeoutSec
	}
	if err := s.saveAIConfigOf(userID, cfg); err != nil {
		response.Internal(w, err)
		return
	}
	response.OK(w, aiConfigView(cfg))
}

// aiTest 通过建会话做一次真实连通性与鉴权自检。
func (s *Server) aiTest(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiConfigOf(userFrom(r).ID)
	if strings.TrimSpace(cfg.APIKey) == "" {
		response.BadRequest(w, "请先填写 BIC-QA API Key")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	client := bicqa.NewClient(cfg)
	convID, err := client.CreateSession(ctx, "题鸣连接测试")
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{
		"ok":             true,
		"conversationId": convID,
		"baseUrl":        cfg.BaseURL,
	})
}

func (s *Server) aiUsage(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiConfigOf(userFrom(r).ID)
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
	if !cfg.Ready() {
		response.BadRequest(w, "尚未启用 BIC-QA：请在设置页填写 API Key 并打开开关")
		return
	}
	// 提问会消耗 BIC-QA 额度，按用户限流，避免误触发把额度打空。
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

	client := bicqa.NewClient(cfg)
	convID := strings.TrimSpace(in.ConversationID)
	if convID == "" {
		id, err := client.CreateSession(ctx, in.Title)
		if err != nil {
			writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
			writeSSE(w, flusher, "done", map[string]any{})
			return
		}
		convID = id
	}
	writeSSE(w, flusher, "meta", map[string]string{"conversationId": convID})

	body, err := client.ChatStream(ctx, question, convID)
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
	streamErr := bicqa.ParseStream(body, func(ev bicqa.Event) error {
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
