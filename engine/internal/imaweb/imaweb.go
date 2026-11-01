// Package imaweb 封装腾讯 ima（ima.qq.com）网页版的私有接口。
//
// 接口契约由网页前端 bundle 逆向 + 真实抓包确认，核心事实只有三条：
//
//   - 鉴权不走 Cookie 头。网页端把整串 cookie 镜像成自定义头
//     `x-ima-cookie`（前端 buildCookie() 拼装 PLATFORM / IMA-UID / IMA-TOKEN
//     / IMA-REFRESH-TOKEN 等），另带 `from_browser_ima: 1`。所以没有 HttpOnly
//     限制，用户把这一整行头复制出来即可，不需要任何签名。
//   - POST /cgi-bin/session_logic/init_session
//     {"name":"…","sessionType":0,"envInfo":{"robotType":10000,"interactType":2}}
//     -> {"code":0,"session_id":"…"}
//   - POST /cgi-bin/assistant/qa（请求与响应都是 text/event-stream）
//     {"session_id","robot_type","question","question_type","client_id",
//     "history_info","client_tools","model_info"}
//
// 响应是带事件名的 SSE，正文在 STRUCTURED_BLOCK / Type=blockMessage 的
// Data.text_message.Text 里，按片段追加；COMPLETED 的 Code 非 0 才是业务失败，
// CLOSE 帧的 data 为 [DONE]。
package imaweb

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"quizbox/engine/internal/aistream"
)

const (
	// DefaultBaseURL 是 ima 网页版地址。
	DefaultBaseURL = "https://ima.qq.com"

	// 前端枚举值：Xs.Main / e9.ChatPage / Ad.Child。
	robotMain        = 10000
	interactChatPage = 2
	sessionChild     = 0

	// 网页端默认模型：official_3（通用问答），model_type 与前端一致。
	modelID   = "official_3"
	modelType = 3

	// 知识库问答的枚举：robotType 5（知识库页的机器人）+
	// commandInfo.type 14（前端 CommandType.KnowledgeQa）。
	robotKnowledge = 5
	commandKQA     = 14

	// MaxQuestionRunes 与 BIC-QA 侧保持一致，避免超长题目被打断。
	MaxQuestionRunes = 4000

	// narrationLimit 是工具旁白的最大长度：超过就当作正文，宁可多显示也不吞答案。
	narrationLimit = 300

	codeSuccess     = 0
	codeLoginFailed = 41

	headerCode    = "Trpc-Func-Ret"
	headerCookie  = "x-ima-cookie"
	headerBrowser = "from_browser_ima"

	userAgentText = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36 Edg/156.0.0.0"
)

// ErrNotConfigured 表示没有填写鉴权串。
var ErrNotConfigured = errors.New("未配置 ima 鉴权信息")

// ErrLoginRequired 表示鉴权串已失效，需要重新复制。
// 单独定义是为了让上层能给出「去重新取一次」这种可操作提示，
// 而不是把 code 41 这种内部错误码直接抛给用户。
var ErrLoginRequired = errors.New("ima 登录已失效，请在浏览器重新登录 ima.qq.com 后复制新的鉴权串")

// Client 是 ima 网页版的最小客户端。
type Client struct {
	baseURL string
	cookie  string
	http    *http.Client
	// kbID 非空时，提问会带上知识库检索参数，答案以该库内容为依据。
	kbID string
}

// NewClient 用指定地址与鉴权串构造客户端；地址留空时用官方域名。
func NewClient(baseURL, cookie string) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{
		baseURL: base,
		cookie:  NormalizeCookie(cookie),
		http: &http.Client{
			// 流式问答可能持续数分钟，整体时限交给调用方的 context 控制。
			Timeout: 0,
		},
	}
}

// WithKnowledgeBase 返回绑定了知识库的副本。空 id 表示不限定知识库（通用问答）。
// 用副本而不是改指针，避免同一个 Client 被不同知识库共用时互相影响。
func (c *Client) WithKnowledgeBase(id string) *Client {
	cp := *c
	cp.kbID = strings.TrimSpace(id)
	return &cp
}

// NormalizeCookie 清洗用户粘贴的鉴权串。它接受的输入很杂，因为用户实际会
// 粘各种东西进来：整行请求头、"Copy as cURL" 复制的整条命令、带引号的片段、
// 中间夹着换行的 HAR 片段。统一收敛成一段可以直接放进 x-ima-cookie 的值。
//
// 只修剪各段两端的空白，段内的空格保持原样（IMA-IUA 就是一段带空格的 UA）。
func NormalizeCookie(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 1) 从整条 cURL / 整行请求头里截出 x-ima-cookie 的值。
	fromHeader := false
	if v, ok := cutHeaderValue(s, "x-ima-cookie:"); ok {
		s, fromHeader = v, true
	} else if v, ok := cutHeaderValue(s, "cookie:"); ok {
		s, fromHeader = v, true
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	// 2) 从请求头里截出来的值可能后面还跟着 cURL 的其他参数，截到第一个引号为止。
	//    直接粘一整串 cookie 的情况不动它，避免误伤值里的合法字符。
	if fromHeader {
		if i := strings.IndexAny(s, `'"`); i >= 0 {
			s = s[:i]
		}
	}
	parts := strings.Split(s, ";")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.TrimSpace(strings.Join(parts, "; "))
}

// cutHeaderValue 在 s 中查找 header（如 "x-ima-cookie:"），返回其后的值。
// 兼容 cURL 里的 `-H 'x-ima-cookie: xxx'` 与 DevTools 里 `x-ima-cookie: xxx` 两种写法。
func cutHeaderValue(s, header string) (string, bool) {
	idx := strings.Index(strings.ToLower(s), strings.ToLower(header))
	if idx < 0 {
		return "", false
	}
	rest := strings.TrimSpace(s[idx+len(header):])
	rest = strings.TrimLeft(rest, `'" `)
	return rest, true
}

// robot 返回本次会话使用的机器人类型：绑了知识库就走知识库机器人。
func (c *Client) robot() int {
	if c.kbID != "" {
		return robotKnowledge
	}
	return robotMain
}

func (c *Client) ready() error {
	if c.cookie == "" {
		return ErrNotConfigured
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, path, contentType string, body any) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/")
	req.Header.Set("User-Agent", userAgentText)
	req.Header.Set(headerCookie, c.cookie)
	req.Header.Set(headerBrowser, "1")
	return req, nil
}

// envelope 覆盖 ima 成功与失败的两种返回形状。
type envelope struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	Message        string `json:"message"`
	SessionID      string `json:"session_id"`
	SessionIDCamel string `json:"sessionId"`
}

func (e envelope) errText() string {
	for _, s := range []string{e.Msg, e.Message} {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// codeError 把业务码翻译成用户能看懂的 error。
func codeError(code int, msg string, status int) error {
	if code == codeSuccess {
		return nil
	}
	if code == codeLoginFailed {
		return ErrLoginRequired
	}
	if strings.TrimSpace(msg) != "" {
		return errors.New(msg)
	}
	return fmt.Errorf("ima 返回失败（code %d，HTTP %d）", code, status)
}

// CreateSession 建一次问答会话并返回 sessionID，同时充当登录态自检。
func (c *Client) CreateSession(ctx context.Context) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	body := map[string]any{
		"name":        "题鸣答疑",
		"envInfo":     map[string]any{"robotType": c.robot(), "interactType": interactChatPage},
		"msgsLimit":   10,
		"sessionType": sessionChild,
	}
	req, err := c.newRequest(ctx, "/cgi-bin/session_logic/init_session", "application/json", body)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", friendlyNetErr(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取 ima 响应失败：%w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || retCode(resp) == codeLoginFailed {
		return "", ErrLoginRequired
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", fmt.Errorf("ima 返回了无法解析的内容：%s", truncate(string(raw), 200))
	}
	if err := codeError(env.Code, env.errText(), resp.StatusCode); err != nil {
		return "", err
	}
	if sid := sessionOf(env); sid != "" {
		return sid, nil
	}
	return "", errors.New("ima 未返回会话编号")
}

// sessionOf 兼容 snake_case 与 camelCase 两种字段风格。
func sessionOf(env envelope) string {
	for _, s := range []string{env.SessionID, env.SessionIDCamel} {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func retCode(resp *http.Response) int {
	v := strings.TrimSpace(resp.Header.Get(headerCode))
	if v == "" {
		return -1
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return -1
	}
	return n
}

// Ask 发起流式问答，把 ima 的事件归一化成 aistream.Event 交给 emit。
// 参数顺序与 bicqa.Client.ChatStream 保持一致（question 在前），
// 上层用同一个函数类型分派两种接入方式，顺序不一致会导致问题与会话号互换。
func (c *Client) Ask(ctx context.Context, question, sessionID string, emit func(aistream.Event) error) error {
	if err := c.ready(); err != nil {
		return err
	}
	if strings.TrimSpace(question) == "" {
		return errors.New("提问内容为空")
	}
	if r := []rune(question); len(r) > MaxQuestionRunes {
		question = string(r[:MaxQuestionRunes])
	}
	body := map[string]any{
		"session_id":    sessionID,
		"robot_type":    c.robot(),
		"question":      question,
		"question_type": 1,
		"client_id":     clientID(),
		"history_info":  map[string]any{},
		"client_tools":  []any{},
		"model_info": map[string]any{
			"model_id":           modelID,
			"model_type":         modelType,
			"enable_enhancement": false,
		},
	}
	// 指定知识库时切到知识库问答：robot_type 5 + command_info(Kind=KnowledgeQa)。
	// knowledge_ids 放库 id，空 tags 与空 media_id_infos 表示「全库检索」。
	if c.kbID != "" {
		body["command_info"] = map[string]any{
			"type": commandKQA,
			"knowledge_qa_info": map[string]any{
				"knowledge_ids":  []string{c.kbID},
				"tags":           []string{},
				"media_id_infos": []any{},
			},
		}
	}
	// 提问接口的 Content-Type 用的就是 text/event-stream，前端亦如此。
	req, err := c.newRequest(ctx, "/cgi-bin/assistant/qa", "text/event-stream", body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return friendlyNetErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || retCode(resp) == codeLoginFailed {
		return ErrLoginRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("ima 返回 HTTP %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	// 少数失败场景会以 JSON 而非 SSE 返回，提前识别，免得前端对着不出字的流干等。
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		var env envelope
		if json.Unmarshal(raw, &env) == nil {
			if err := codeError(env.Code, env.errText(), resp.StatusCode); err != nil {
				return err
			}
		}
		return fmt.Errorf("ima 未返回流式响应：%s", truncate(string(raw), 200))
	}
	return c.parseStream(resp.Body, emit)
}

// parseStream 解析 ima 的命名事件 SSE。
//
// 事件名是稳定契约，正文位置也稳定：STRUCTURED_BLOCK / blockMessage /
// Data.text_message.Text 是**增量片段**，需要由前端累积显示。其余帧
// （QA_START、SESSION_START、ATTACHED_BLOCK、SUGGEST_QUESTION）与正文无关，
// 直接忽略；COMPLETED 带业务码，CLOSE 表示流结束。
func (c *Client) parseStream(body io.Reader, emit func(aistream.Event) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var eventName string
	var dataLines []string
	total := 0
	finished := false
	// 知识库问答会先吐一段「I'll search the knowledge base…」式的工具旁白，
	// 再吐 fileList（检索到的文件），最后才是答案正文。旁白与正文不仅 block id
	// 不同，语言也不同（旁白固定英文、正文跟随提问语言）。这里用「检索前 + 无中文
	// + 累计不超过 300 字」三条约束把它归到思考过程，避免英文句子插在中文答案开头。
	sawRetrieval := false
	narrationRunes := 0
	narrationOn := true

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		name := strings.ToUpper(strings.TrimSpace(eventName))
		dataLines = dataLines[:0]
		eventName = ""

		switch name {
		case "CLOSE":
			finished = true
			return emit(aistream.Event{Kind: "done"})
		case "COMPLETED":
			var c struct {
				Code int    `json:"Code"`
				Msg  string `json:"Msg"`
			}
			if json.Unmarshal([]byte(payload), &c) == nil && c.Code != codeSuccess {
				if c.Code == codeLoginFailed {
					return ErrLoginRequired
				}
				msg := strings.TrimSpace(c.Msg)
				if msg == "" {
					msg = fmt.Sprintf("ima 回答失败（code %d）", c.Code)
				}
				return emit(aistream.Event{Kind: "error", Text: msg})
			}
			return nil
		case "STRUCTURED_BLOCK":
			block := decodeBlock(payload)
			if block.Type == "fileList" {
				sawRetrieval = true
				return nil
			}
			kind, text := block.event(c.kbID != "")
			if text == "" {
				return nil
			}
			if kind == "delta" && narrationOn && !sawRetrieval {
				if hasCJK(text) || narrationRunes >= narrationLimit {
					narrationOn = false
				} else {
					// 检索前的纯英文短句是模型的工具旁白，不算答案正文。
					narrationRunes += len([]rune(text))
					kind = "thinking"
				}
			}
			if kind == "delta" {
				total += len([]rune(text))
				if total > aistream.MaxRunes {
					return emit(aistream.Event{Kind: "done"})
				}
			}
			return emit(aistream.Event{Kind: kind, Text: text})
		case "QA_START", "SESSION_START", "ATTACHED_BLOCK", "SUGGEST_QUESTION", "":
			// 与正文无关：图标、消息序号、下载引导、推荐追问。
			return nil
		}
		// 未知事件名不丢：尽量从载荷里捞正文，捞不到就当没有。
		if kind, text := aistream.Extract(payload); text != "" {
			return emit(aistream.Event{Kind: kind, Text: text})
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return err
			}
			if finished {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

// structuredBlock 是 STRUCTURED_BLOCK 帧的结构。Type 取值有 loading /
// blockMessage / fileList / 其它（卡片、引导等，与正文无关）。
type structuredBlock struct {
	Type       string `json:"Type"`
	UpdateType string `json:"UpdateType"`
	Data       struct {
		TextMessage struct {
			Text string `json:"Text"`
		} `json:"text_message"`
		ThinkMessage struct {
			Text string `json:"Text"`
		} `json:"think_message"`
	} `json:"Data"`
}

func decodeBlock(payload string) structuredBlock {
	var blk structuredBlock
	_ = json.Unmarshal([]byte(payload), &blk)
	return blk
}

// event 把单个块翻译成统一事件；空文本表示这个块与正文无关。
func (blk structuredBlock) event(kbBound bool) (kind, text string) {
	switch blk.Type {
	case "blockMessage":
		if t := blk.Data.ThinkMessage.Text; strings.TrimSpace(t) != "" {
			return "thinking", t
		}
		return "delta", blk.Data.TextMessage.Text
	case "loading":
		// loading 只是占位转圈，create 时给一句提示，delete 时无事可做。
		if strings.EqualFold(blk.UpdateType, "create") {
			return "thinking", ""
		}
	}
	return "", ""
}

// clientID 生成一个 uuid v4 形态的请求号，供上游做去重与链路追踪。
func clientID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 随机源不可用时退回时间戳，重复概率可接受，不影响提问本身。
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (8 * (i % 8)))
		}
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func friendlyNetErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return errors.New("请求已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("连接 ima 超时")
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host"):
		return errors.New("无法解析 ima.qq.com，请检查网络或代理设置")
	case strings.Contains(msg, "connection refused"):
		return errors.New("连接被拒绝，请检查网络或代理设置")
	}
	return fmt.Errorf("连接 ima 失败：%w", err)
}

// hasCJK 判断文本里有没有中日韩字符，用来区分「工具旁白」和「答案正文」。
func hasCJK(s string) bool {
	for _, r := range s {
		if (r >= 0x4e00 && r <= 0x9fff) || (r >= 0x3040 && r <= 0x30ff) || (r >= 0xac00 && r <= 0xd7af) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
