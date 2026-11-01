// Package imaweb 封装腾讯 ima（ima.qq.com）网页版的私有接口。
//
// 这不是官方 Open API，而是网页前端自身在用的 CGI。接口契约由前端 bundle
// 逆向而来，随时可能变动，因此本包只依赖三件最稳定的事实：
//
//   - POST {base}/cgi-bin/session_logic/init_session
//     {"envInfo":{"robotType":10000,"interactType":2},"msgsLimit":10,"sessionType":0}
//     -> {"code":0,"sessionId":"..."}
//   - POST {base}/cgi-bin/assistant/qa   （Accept: text/event-stream）
//     {"sessionId":"...","robotType":10000,"question":"...","questionType":1,"clientId":""}
//   - 鉴权失败一律用 code 41 表示：HTTP 401 时在 Trpc-Func-Ret 头里，
//     HTTP 200 时在响应体的 code 字段里。
//
// 鉴权只靠浏览器的登录 Cookie，不做请求签名：前端的 x-ima-cm / x-ima-ckey /
// x-ima-ctk 只在 hasNativeBridge()（桌面客户端内置加密桥）为真时才启用。
package imaweb

import (
	"bytes"
	"context"
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
	// DefaultBaseURL 是 ima 网页版地址，与 bicqa.ImaWebDefaultBaseURL 保持一致。
	DefaultBaseURL = "https://ima.qq.com"

	// 前端枚举值：Xs.Main / e9.ChatPage / Ad.Child。
	robotMain        = 10000
	interactChatPage = 2
	sessionChild     = 0

	// MaxQuestionRunes 与 BIC-QA 侧保持一致，避免超长题目被打断。
	MaxQuestionRunes = 4000

	codeSuccess     = 0
	codeLoginFailed = 41

	ctkHeader     = "Trpc-Func-Ret"
	userAgentText = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// ErrNotConfigured 表示没有填写登录 Cookie。
var ErrNotConfigured = errors.New("未配置 ima 登录信息")

// ErrLoginRequired 表示 Cookie 已失效，需要重新从浏览器复制。
// 单独定义是为了让上层能给出「去重新取一次 Cookie」这种可操作提示，
// 而不是把 code 41 这种内部错误码直接抛给用户。
var ErrLoginRequired = errors.New("ima 登录已失效，请重新登录 ima.qq.com 后复制新的 Cookie")

// Client 是 ima 网页版的最小客户端。
type Client struct {
	baseURL string
	cookie  string
	http    *http.Client
}

// NewClient 用指定地址与 Cookie 构造客户端；地址留空时用官方域名。
func NewClient(baseURL, cookie string) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{
		baseURL: base,
		cookie:  strings.TrimSpace(cookie),
		http: &http.Client{
			// 流式问答可能持续数分钟，整体时限交给调用方的 context 控制。
			Timeout: 0,
		},
	}
}

func (c *Client) ready() error {
	if c.cookie == "" {
		return ErrNotConfigured
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", c.baseURL)
	req.Header.Set("Referer", c.baseURL+"/")
	req.Header.Set("User-Agent", userAgentText)
	req.Header.Set("Cookie", c.cookie)
	return req, nil
}

// ima 的成功与否同时体现在 HTTP 状态码和响应体 code 上，两处都要看。
type envelope struct {
	Code        int             `json:"code"`
	Msg         string          `json:"msg"`
	Message     string          `json:"message"`
	SessionID   string          `json:"sessionId"`
	SessionInfo json.RawMessage `json:"sessionInfo"`
	Data        json.RawMessage `json:"data"`
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

// CreateSession 建一次问答会话并返回 sessionId，同时充当登录态自检。
func (c *Client) CreateSession(ctx context.Context) (string, error) {
	if err := c.ready(); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	body := map[string]any{
		"envInfo": map[string]any{
			"robotType":    robotMain,
			"interactType": interactChatPage,
		},
		"msgsLimit":   10,
		"sessionType": sessionChild,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/cgi-bin/session_logic/init_session", body)
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
	if resp.StatusCode == http.StatusUnauthorized || headerCode(resp) == codeLoginFailed {
		return "", ErrLoginRequired
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", fmt.Errorf("ima 返回了无法解析的内容：%s", truncate(string(raw), 200))
	}
	if err := codeError(env.Code, env.errText(), resp.StatusCode); err != nil {
		return "", err
	}
	if sid := pickSessionID(env); sid != "" {
		return sid, nil
	}
	return "", errors.New("ima 未返回会话编号")
}

// pickSessionID 兼容几种可能的返回形状：顶层 sessionId、data.sessionId、
// sessionInfo.id，以及 data.sessionInfo.id。
func pickSessionID(env envelope) string {
	if s := strings.TrimSpace(env.SessionID); s != "" {
		return s
	}
	if id := idFromJSON(env.SessionInfo); id != "" {
		return id
	}
	if len(env.Data) == 0 {
		return ""
	}
	var d struct {
		SessionID   string          `json:"sessionId"`
		SessionInfo json.RawMessage `json:"sessionInfo"`
	}
	if json.Unmarshal(env.Data, &d) != nil {
		return ""
	}
	if s := strings.TrimSpace(d.SessionID); s != "" {
		return s
	}
	return idFromJSON(d.SessionInfo)
}

func idFromJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v struct {
		ID     string `json:"id"`
		IDNum  int64  `json:"idNum"`
		TaskID string `json:"taskId"`
	}
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	if strings.TrimSpace(v.ID) != "" {
		return v.ID
	}
	if v.IDNum > 0 {
		return fmt.Sprintf("%d", v.IDNum)
	}
	return ""
}

func headerCode(resp *http.Response) int {
	v := strings.TrimSpace(resp.Header.Get(ctkHeader))
	if v == "" {
		return -1
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return -1
	}
	return n
}

// ChatStream 发起流式问答，返回未经解析的 SSE 响应体。
func (c *Client) ChatStream(ctx context.Context, question, sessionID string) (io.ReadCloser, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	body := map[string]any{
		"sessionId":    sessionID,
		"robotType":    robotMain,
		"question":     question,
		"questionType": 1,
		"clientId":     "",
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/cgi-bin/assistant/qa", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	if resp.StatusCode == http.StatusUnauthorized || headerCode(resp) == codeLoginFailed {
		resp.Body.Close()
		return nil, ErrLoginRequired
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		return nil, fmt.Errorf("ima 返回 HTTP %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	// 少数失败场景（例如会话不存在）会以 JSON 而非 SSE 返回，提前识别，
	// 免得前端对着一个永远不出字的流干等。
	if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		var env envelope
		if json.Unmarshal(raw, &env) == nil {
			if err := codeError(env.Code, env.errText(), resp.StatusCode); err != nil {
				return nil, err
			}
		}
		return nil, fmt.Errorf("ima 未返回流式响应：%s", truncate(string(raw), 200))
	}
	return resp.Body, nil
}

// ParseStream 解析 ima 的 SSE 响应并归一化成 delta / thinking / error / done。
//
// ima 的事件名（add / block / done 等）相对前端 bundle 而言是内部约定，
// 这里不做白名单，统一交给 aistream 的宽松提取器：能取到正文就发 delta，
// 取不到就丢弃，避免出现空帧刷屏。
func ParseStream(body io.Reader, emit func(aistream.Event) error) error {
	return aistream.Parse(body, emit)
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

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
