package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"quizbox/engine/internal/aistream"
	"quizbox/engine/internal/bicqa"
	"quizbox/engine/internal/llm"
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

// 通用大模型没有题库领域的先验，角色和输出格式得在系统提示里交代清楚，
// 否则容易把题干原样复述一遍或者写成长篇教程。
const systemPromptOpenAI = "你是一名题库答疑助手。用户做题时会带着题干与选项追问知识点，" +
	"请结合题型作答：判断题干与各选项的正误并说明原因，必要时补充相关知识点。" +
	"结论先行，使用简体中文，不要复述题目原文。"

const (
	// maxHistoryMessages 限制带入的上下文条数。通用模型按 token 计费，
	// 长对话原样回传会迅速把成本推高，也会挤占回答本身的额度。
	maxHistoryMessages = 20
	// maxHistoryRunes 限制单条历史消息长度。
	maxHistoryRunes = 4000
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

// channelView 是单条通道回显给前端的形态。Key 只给掩码，明文永不出本机。
func channelView(ch bicqa.Channel) map[string]any {
	return map[string]any{
		"enabled":    ch.Enabled,
		"hasKey":     strings.TrimSpace(ch.APIKey) != "",
		"keyMask":    ch.MaskedKey(),
		"baseUrl":    ch.BaseURL,
		"model":      ch.Model,
		"timeoutSec": ch.TimeoutSec,
	}
}

// aiConfigView 一次回显两条通道。前端切换服务商时直接用本地已有的数据，
// 不必再请求一次，也不会因为切换而丢掉另一条通道的配置。
func aiConfigView(cfg bicqa.Config) map[string]any {
	return map[string]any{
		"active": cfg.Active,
		"ready":  cfg.Ready(),
		"bicqa":  channelView(cfg.BicQA),
		"openai": channelView(cfg.OpenAI),
	}
}

func (s *Server) aiConfig(w http.ResponseWriter, r *http.Request) {
	response.OK(w, aiConfigView(s.aiConfigOf(userFrom(r).ID)))
}

func (s *Server) updateAIConfig(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Active  string `json:"active"`
		Channel struct {
			Enabled    *bool   `json:"enabled"`
			BaseURL    string  `json:"baseUrl"`
			APIKey     string  `json:"apiKey"`
			Model      *string `json:"model"`
			TimeoutSec int     `json:"timeoutSec"`
			ClearKey   bool    `json:"clearKey"`
		} `json:"channel"`
	}
	if err := decodeJSON(r, &in); err != nil {
		response.BadRequest(w, "请求格式不正确")
		return
	}
	userID := userFrom(r).ID
	cfg := s.aiConfigOf(userID)

	switch strings.TrimSpace(in.Active) {
	case bicqa.ProviderBicQA:
		cfg.Active = bicqa.ProviderBicQA
	case bicqa.ProviderOpenAI:
		cfg.Active = bicqa.ProviderOpenAI
	case "":
		// 不传表示只改当前通道，不改变生效的是哪一条。
	default:
		response.BadRequest(w, "服务商类型不正确")
		return
	}

	// 只改当前生效的那条通道，另一条原样保留，切回去不用重填。
	ch := cfg.Current()
	if in.Channel.Enabled != nil {
		ch.Enabled = *in.Channel.Enabled
	}
	if strings.TrimSpace(in.Channel.BaseURL) != "" {
		base := strings.TrimRight(strings.TrimSpace(in.Channel.BaseURL), "/")
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			response.BadRequest(w, "服务地址需以 http:// 或 https:// 开头")
			return
		}
		ch.BaseURL = base
	}
	// 留空表示沿用已保存的 Key，避免前端拿不到明文又无法只改其他字段。
	if in.Channel.ClearKey {
		ch.APIKey = ""
	} else if key := strings.TrimSpace(in.Channel.APIKey); key != "" {
		ch.APIKey = key
	}
	// 用指针区分「没传这个字段」和「传了空串」：前者保持原值，后者才是清空。
	// 否则只改服务地址的请求会把模型名一起抹掉——前端全量提交时看不出来，
	// 但接口单独调用就会莫名其妙丢配置。
	if in.Channel.Model != nil {
		ch.Model = strings.TrimSpace(*in.Channel.Model)
	}
	if in.Channel.TimeoutSec > 0 {
		ch.TimeoutSec = in.Channel.TimeoutSec
	}

	cfg = cfg.WithCurrent(ch)
	if err := s.saveAIConfigOf(userID, cfg); err != nil {
		response.Internal(w, err)
		return
	}
	// 保存会补全默认值，回显也得用补全后的副本，否则前端拿到空的服务地址。
	response.OK(w, aiConfigView(cfg.Normalized()))
}

// aiNotReadyHint 按服务商给出能直接照做的提示，避免用户对着「未配置」无从下手。
func aiNotReadyHint(cfg bicqa.Config) string {
	ch := cfg.Current()
	if !cfg.IsOpenAI() {
		return "请先填写 BIC-QA API Key 并打开开关"
	}
	if strings.TrimSpace(ch.BaseURL) == "" {
		return "请先选择服务商并填写服务地址"
	}
	if strings.TrimSpace(ch.Model) == "" {
		return "请先填写模型名称"
	}
	return "请先填写 API Key 并打开开关"
}

// aiTest 做一次真实的连通性与鉴权自检：BIC-QA 靠建会话验证，
// 通用模型靠一次极小的非流式请求验证，两者都能顺带确认 Key 是否有效。
func (s *Server) aiTest(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiConfigOf(userFrom(r).ID)
	if !cfg.Ready() {
		response.BadRequest(w, aiNotReadyHint(cfg))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	ch := cfg.Current()
	if cfg.IsOpenAI() {
		if err := llm.NewClient(ch.BaseURL, ch.APIKey, ch.Model).Ping(ctx); err != nil {
			response.BadRequest(w, err.Error())
			return
		}
		response.OK(w, map[string]any{
			"ok":      true,
			"baseUrl": ch.BaseURL,
			"model":   ch.Model,
		})
		return
	}

	id, err := bicqa.NewClient(ch).CreateSession(ctx, "题鸣连接测试")
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.OK(w, map[string]any{
		"ok":             true,
		"conversationId": id,
		"baseUrl":        ch.BaseURL,
	})
}

func (s *Server) aiUsage(w http.ResponseWriter, r *http.Request) {
	cfg := s.aiConfigOf(userFrom(r).ID)
	if !cfg.Ready() {
		response.BadRequest(w, aiNotReadyHint(cfg))
		return
	}
	if cfg.IsOpenAI() {
		// 各家控制台的额度口径和鉴权方式都不一样，没有可统一调用的接口。
		response.BadRequest(w, "通用大模型请在服务商控制台查看用量")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	data, err := bicqa.NewClient(cfg.Current()).Usage(ctx)
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

// aiChat 代理 AI 流式问答，把统一后的 meta / thinking / delta / error / done
// 事件推给前端。走引擎代理而不是前端直连：绕开 CORS，并让 API Key 只留在本机。
//
// 两条通道共用同一套事件契约，前端不必感知当前走的是哪一家：
// BIC-QA 靠服务端会话维持多轮，通用大模型靠请求体里的 history。
func (s *Server) aiChat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Question       string        `json:"question"`
		ConversationID string        `json:"conversationId"`
		Title          string        `json:"title"`
		Mode           string        `json:"mode"`
		History        []chatMessage `json:"history"`
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
		response.BadRequest(w, aiNotReadyHint(cfg))
		return
	}
	// 提问会消耗额度，按用户限流，避免误触发把额度打空。
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

	ch := cfg.Current()
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(ch.TimeoutSec)*time.Second)
	defer cancel()

	if cfg.IsOpenAI() {
		s.chatOpenAI(w, flusher, ctx, ch, question, in.History)
		return
	}

	client := bicqa.NewClient(ch)
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
	// 会话号只有 BIC-QA 需要，它把上下文存在服务端。通用模型不发这个事件，
	// 前端拿不到 id 也不影响多轮，历史由请求体里的 history 携带。
	writeSSE(w, flusher, "meta", map[string]string{"conversationId": convID})

	relayStream(ctx, w, flusher, func(emit func(aistream.Event) error) error {
		return askBICQA(ctx, client, question, convID, emit)
	})
}

// chatMessage 是多轮上下文里的一条消息，与前端答疑抽屉的 turns 一一对应。
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildMessages 把系统提示、历史轮次与当前问题拼成 Chat Completions 的消息数组。
// 历史只取最近若干条并逐条裁剪，避免长对话把 token 和费用推高。
func buildMessages(question string, history []chatMessage) []llm.Message {
	msgs := make([]llm.Message, 0, len(history)+2)
	msgs = append(msgs, llm.Message{Role: "system", Content: systemPromptOpenAI})

	if len(history) > maxHistoryMessages {
		history = history[len(history)-maxHistoryMessages:]
	}
	for _, h := range history {
		role := strings.TrimSpace(h.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(h.Content)
		if content == "" {
			continue
		}
		if r := []rune(content); len(r) > maxHistoryRunes {
			content = string(r[:maxHistoryRunes])
		}
		msgs = append(msgs, llm.Message{Role: role, Content: content})
	}
	msgs = append(msgs, llm.Message{Role: "user", Content: question})
	return msgs
}

// chatOpenAI 走 OpenAI 兼容协议。该协议本身无状态，多轮完全依赖请求里的
// messages，因此这里不发 meta 事件，前端也拿不到会话号。
func (s *Server) chatOpenAI(
	w http.ResponseWriter,
	flusher http.Flusher,
	ctx context.Context,
	ch bicqa.Channel,
	question string,
	history []chatMessage,
) {
	body, err := llm.NewClient(ch.BaseURL, ch.APIKey, ch.Model).
		ChatStream(ctx, buildMessages(question, history))
	if err != nil {
		writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
		writeSSE(w, flusher, "done", map[string]any{})
		return
	}
	defer body.Close()

	relayStream(ctx, w, flusher, func(emit func(aistream.Event) error) error {
		return aistream.Parse(body, emit)
	})
}

// relayStream 把厂商流按统一事件转发给前端，并保证一定有 error / done 收尾。
// 两个通道的解析器不同，但收尾规则必须一致，否则前端会一直转圈。
func relayStream(
	ctx context.Context,
	w http.ResponseWriter,
	flusher http.Flusher,
	parse func(emit func(aistream.Event) error) error,
) {
	sentDone := false
	finish := func(message string) {
		if message != "" {
			writeSSE(w, flusher, "error", map[string]string{"message": message})
		}
		writeSSE(w, flusher, "done", map[string]any{})
		sentDone = true
	}
	emit := func(ev aistream.Event) error {
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
	}

	streamErr := parse(emit)
	if sentDone {
		return
	}
	if streamErr != nil && !errors.Is(streamErr, errStopStream) && ctx.Err() == nil {
		finish(streamErr.Error())
		return
	}
	finish("")
}

// askBICQA 发起一次流式问答并把事件交给 emit。
func askBICQA(
	ctx context.Context,
	client *bicqa.Client,
	question, sessionID string,
	emit func(aistream.Event) error,
) error {
	body, err := client.ChatStream(ctx, question, sessionID)
	if err != nil {
		return err
	}
	defer body.Close()
	return bicqa.ParseStream(body, emit)
}
