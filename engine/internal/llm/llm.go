// Package llm 封装 OpenAI 兼容的 Chat Completions 协议。
//
// 覆盖阿里云百炼（通义千问）、DeepSeek、智谱 GLM、月之暗面、硅基流动、
// OpenAI 以及任何自称兼容的自建网关。这些服务商的请求与响应结构一致，
// 差异只在 baseURL 的版本段（/v1、/v4、/compatible-mode/v1）。
//
// 与 bicqa 包的分工：bicqa 走厂商私有协议，会话存在服务端、有独立额度接口；
// 本包是无状态协议，多轮上下文必须由调用方每次随 messages 一起带上。
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"syscall"
	"time"
)

// Message 是 Chat Completions 协议里的一条消息。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client 是 OpenAI 兼容服务的最小客户端。
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewClient 构造客户端。baseURL 约定填到版本段（.../v1、.../v4），
// 末尾斜杠会被清掉，避免拼出双斜杠。
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		model:   strings.TrimSpace(model),
		http: &http.Client{
			// 流式回答可能持续数分钟，整体时限交给调用方的 context 控制。
			Timeout: 0,
		},
	}
}

// Endpoint 返回实际请求地址。baseURL 若已经贴到 /chat/completions 就不再重复拼接，
// 省得用户按别家文档填了完整路径后拼成 .../chat/completions/chat/completions。
func (c *Client) Endpoint() string {
	if strings.HasSuffix(c.baseURL, "/chat/completions") {
		return c.baseURL
	}
	return c.baseURL + "/chat/completions"
}

func (c *Client) newRequest(ctx context.Context, body any) (*http.Request, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// ChatStream 发起流式问答，返回 SSE 响应体，由调用方交给 aistream 解析。
func (c *Client) ChatStream(ctx context.Context, messages []Message) (io.ReadCloser, error) {
	body := map[string]any{
		"model":    c.model,
		"messages": messages,
		"stream":   true,
	}
	req, err := c.newRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		return nil, apiError(resp.StatusCode, raw)
	}
	// 少数网关无视 stream 参数直接回一整个 JSON，提前识别，免得前端一直空等。
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "application/json") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		return nil, fmt.Errorf("服务商未按流式返回：%s", truncate(string(raw), 200))
	}
	return resp.Body, nil
}

// Ping 用一次极小的非流式请求做连通性与鉴权自检。
// 不用流式是因为流式在鉴权失败时也可能先回 200 再在流里报错，判断会绕远。
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	body := map[string]any{
		"model":      c.model,
		"messages":   []Message{{Role: "user", Content: "ping"}},
		"stream":     false,
		"max_tokens": 1,
	}
	req, err := c.newRequest(ctx, body)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return friendlyNetErr(err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return fmt.Errorf("读取服务商响应失败：%w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return apiError(resp.StatusCode, raw)
	}

	// 部分兼容网关在 HTTP 200 的载荷里塞 error，不能只看状态码。
	var probe struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Choices []json.RawMessage `json:"choices"`
	}
	if json.Unmarshal(raw, &probe) == nil {
		if probe.Error != nil && strings.TrimSpace(probe.Error.Message) != "" {
			return errors.New(strings.TrimSpace(probe.Error.Message))
		}
		if len(probe.Choices) == 0 {
			return fmt.Errorf("服务商返回内容异常：%s", truncate(string(raw), 160))
		}
	}
	return nil
}

// apiError 把非 2xx 响应翻译成能指导用户操作的中文提示。
func apiError(status int, raw []byte) error {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	msg := ""
	if json.Unmarshal(raw, &e) == nil {
		for _, s := range []string{e.Error.Message, e.Message, e.Detail} {
			if strings.TrimSpace(s) != "" {
				msg = strings.TrimSpace(s)
				break
			}
		}
	}
	if msg == "" {
		msg = truncate(string(raw), 200)
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("鉴权失败（HTTP %d），请检查 API Key：%s", status, msg)
	case http.StatusNotFound:
		return fmt.Errorf("接口不存在（HTTP %d），请确认服务地址填到版本段（如 /v1）：%s", status, msg)
	case http.StatusTooManyRequests:
		return fmt.Errorf("请求过于频繁或额度不足（HTTP %d）：%s", status, msg)
	}
	return fmt.Errorf("服务商返回 HTTP %d：%s", status, msg)
}

func friendlyNetErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return errors.New("请求已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("连接大模型服务超时")
	}
	msg := err.Error()
	switch {
	// 按错误码判断最可靠：Windows 报的是 "actively refused"，
	// Linux/macOS 报的是 "connection refused"，只匹配字符串会漏掉一边。
	case errors.Is(err, syscall.ECONNREFUSED),
		strings.Contains(msg, "connection refused"),
		strings.Contains(msg, "actively refused"):
		return errors.New("连接被拒绝，请检查服务地址、网络或代理设置")
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return errors.New("网络不可达，请检查网络或代理设置")
	case strings.Contains(msg, "no such host"):
		return errors.New("无法解析服务地址，请检查网络、代理或地址是否写错")
	case strings.Contains(msg, "certificate"):
		return errors.New("证书校验失败，请检查网络环境或代理设置")
	case strings.Contains(msg, "timeout"):
		return errors.New("连接大模型服务超时")
	}
	return fmt.Errorf("连接大模型服务失败：%w", err)
}

func truncate(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= max {
		return string(r)
	}
	return string(r[:max]) + "…"
}
