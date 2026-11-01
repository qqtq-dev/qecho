// Package bicqa 封装 BIC-QA（佰晟智算国产数据库智能问答）Open API v1 的调用。
//
// 接口契约（v1）：
//   - POST {base}/open-api/v1/session/create  {"title":"..."} -> data.conversationId
//   - POST {base}/open-api/v1/chat            {"question","conversationId"}，Accept: text/event-stream
//   - POST {base}/open-api/v1/token-usage/details
//
// 鉴权统一使用 Authorization: Bearer <API Key>。
// 注意：BIC-QA 在鉴权失败等场景仍可能返回 HTTP 200，必须以响应载荷中的
// status / message 判断成败，不能只看状态码。
package bicqa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultBaseURL 是 BIC-QA Open API 的默认地址（不含 /open-api/v1 前缀）。
	DefaultBaseURL = "https://api.bic-qa.com"
	// ImaWebDefaultBaseURL 是 ima 网页版的默认地址。imaweb 包内有一份同值常量，
	// 两处都改才能生效，这里保留是为了让配置归一化不依赖 imaweb 包。
	ImaWebDefaultBaseURL = "https://ima.qq.com"
	configDirName        = "bicqa"

	// MaxQuestionRunes 限制单次提问长度，避免把整份题图或超长文本误发出去。
	MaxQuestionRunes = 4000
)

// ErrNotConfigured 表示尚未启用或缺少 API Key。
var ErrNotConfigured = errors.New("未配置 BIC-QA API Key")

// 支持的接入方式。BIC-QA 是官方的 Open API，走 API Key；
// ima 网页版是私有接口，走浏览器登录态（Cookie）。两者并存，由用户自选。
const (
	ProviderBICQA  = "bicqa"
	ProviderImaWeb = "imaweb"
)

// Config 是 AI 问答的本地配置。密钥类字段只保存在数据目录内，不随备份打包。
//
// 一个结构承载两种接入方式，是为了让旧配置文件在没有 provider 字段时
// 仍被当作 BIC-QA 读取——升级程序不会让已填好的 Key 失效。
type Config struct {
	Provider   string `json:"provider,omitempty"`
	Enabled    bool   `json:"enabled"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	Cookie     string `json:"cookie,omitempty"`
	Model      string `json:"model"`
	TimeoutSec int    `json:"timeoutSec"`
}

// ProviderName 返回归一化后的接入方式，无法识别时按 BIC-QA 处理。
func (c Config) ProviderName() string {
	switch strings.ToLower(strings.TrimSpace(c.Provider)) {
	case ProviderImaWeb, "ima", "ima-web":
		return ProviderImaWeb
	default:
		return ProviderBICQA
	}
}

// Normalized 返回补齐默认值后的副本。读取配置和回显配置前都要过一遍，
// 否则界面上会出现空的服务地址（实际保存时用的是补全后的值）。
func (c Config) Normalized() Config {
	return c.normalized()
}

func (c Config) normalized() Config {
	c.Provider = c.ProviderName()
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	if c.BaseURL == "" {
		if c.Provider == ProviderImaWeb {
			c.BaseURL = ImaWebDefaultBaseURL
		} else {
			c.BaseURL = DefaultBaseURL
		}
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.Cookie = strings.TrimSpace(c.Cookie)
	c.Model = strings.TrimSpace(c.Model)
	if c.TimeoutSec < 30 || c.TimeoutSec > 900 {
		c.TimeoutSec = 300
	}
	return c
}

// Ready 表示当前接入方式所需的最小凭据已具备。
func (c Config) Ready() bool {
	if !c.Enabled {
		return false
	}
	if c.ProviderName() == ProviderImaWeb {
		return c.Cookie != ""
	}
	return c.APIKey != ""
}

// MaskedCookie 只回显登录态的长度，不泄露任何 Cookie 片段。
// Cookie 里往往直接含账号令牌，比 API Key 更敏感，因此不做首尾留字。
func (c Config) MaskedCookie() string {
	n := len([]rune(strings.TrimSpace(c.Cookie)))
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("已保存 %d 字符", n)
}

// MaskedKey 返回脱敏后的 Key，仅用于前端展示，绝不回传明文。
func (c Config) MaskedKey() string {
	key := strings.TrimSpace(c.APIKey)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:3] + strings.Repeat("*", 6) + key[len(key)-4:]
}

// configPath 返回某个用户的配置文件路径。每个账号一份，互不可见。
// 放在数据目录而非数据库里：既不会随备份 zip 导出，也不会被他人读到。
func configPath(dir string, userID int64) string {
	return filepath.Join(dir, configDirName, fmt.Sprintf("user-%d.json", userID))
}

// Load 读取指定用户的配置；文件不存在时返回默认值。
func Load(dir string, userID int64) Config {
	cfg := Config{BaseURL: DefaultBaseURL, TimeoutSec: 300}
	if userID <= 0 {
		return cfg.normalized()
	}
	raw, err := os.ReadFile(configPath(dir, userID))
	if err != nil {
		return cfg.normalized()
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{BaseURL: DefaultBaseURL, TimeoutSec: 300}.normalized()
	}
	return cfg.normalized()
}

// Save 写入指定用户的配置，文件权限固定为 0600（类 Unix 下生效）。
func Save(dir string, userID int64, cfg Config) error {
	if userID <= 0 {
		return errors.New("用户编号无效")
	}
	cfg = cfg.normalized()
	path := configPath(dir, userID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// Client 是 BIC-QA Open API 的最小客户端。
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewClient(cfg Config) *Client {
	cfg = cfg.normalized()
	return &Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		http: &http.Client{
			// 流式问答可能持续数分钟，整体时限交给调用方的 context 控制。
			Timeout: 0,
		},
	}
}

type envelope struct {
	Status  string          `json:"status"`
	Code    any             `json:"code"`
	Message string          `json:"message"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func (e envelope) message() string {
	for _, s := range []string{e.Message, e.Msg} {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func (e envelope) ok() bool {
	switch strings.ToLower(strings.TrimSpace(e.Status)) {
	case "success", "ok", "succeeded", "done":
		return true
	case "error", "failed", "fail", "unauthorized", "forbidden":
		return false
	}
	// status 缺失时按 code 判断：0 或 200 视为成功。
	if e.Code != nil {
		switch v := e.Code.(type) {
		case float64:
			return v == 0 || v == 200
		case string:
			return v == "0" || v == "200" || strings.EqualFold(v, "success")
		}
	}
	return e.Status == ""
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
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// doJSON 发起请求并把响应解析成统一信封；HTTP 非 2xx 与业务失败都转成 error。
func (c *Client) doJSON(ctx context.Context, method, path string, body any) (*envelope, error) {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 BIC-QA 响应失败：%w", err)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("BIC-QA 返回 HTTP %d：%s", resp.StatusCode, truncate(string(raw), 200))
		}
		return nil, fmt.Errorf("BIC-QA 返回了无法解析的内容：%s", truncate(string(raw), 200))
	}
	if !env.ok() {
		msg := env.message()
		if msg == "" {
			msg = fmt.Sprintf("BIC-QA 返回失败（HTTP %d）", resp.StatusCode)
		}
		return nil, errors.New(msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("BIC-QA 返回 HTTP %d：%s", resp.StatusCode, env.message())
	}
	return &env, nil
}

// CreateSession 建立一次问答会话并返回 conversationId，同时用作鉴权自检。
func (c *Client) CreateSession(ctx context.Context, title string) (string, error) {
	if strings.TrimSpace(title) == "" {
		title = "题鸣练习答疑"
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	env, err := c.doJSON(ctx, http.MethodPost, "/open-api/v1/session/create",
		map[string]string{"title": truncate(title, 60)})
	if err != nil {
		return "", err
	}
	var data struct {
		ConversationID string `json:"conversationId"`
	}
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &data)
	}
	if data.ConversationID == "" {
		return "", errors.New("BIC-QA 未返回会话编号")
	}
	return data.ConversationID, nil
}

// ChatStream 发起流式问答，返回 SSE 响应体，由调用方解析并转发。
func (c *Client) ChatStream(ctx context.Context, question, conversationID string) (io.ReadCloser, error) {
	body := map[string]string{
		"question":       question,
		"conversationId": conversationID,
	}
	if c.model != "" {
		body["model"] = c.model
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/open-api/v1/chat", body)
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
		return nil, fmt.Errorf("BIC-QA 返回 HTTP %d：%s", resp.StatusCode, truncate(string(raw), 200))
	}
	contentType := resp.Header.Get("Content-Type")
	// 少数失败场景会以 JSON 返回，这里提前识别，避免前端一直空等流。
	if strings.Contains(contentType, "application/json") {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		var env envelope
		if json.Unmarshal(raw, &env) == nil && !env.ok() {
			msg := env.message()
			if msg == "" {
				msg = "BIC-QA 返回失败"
			}
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("BIC-QA 未返回流式响应：%s", truncate(string(raw), 200))
	}
	return resp.Body, nil
}

// Usage 查询当前 Key 的额度明细，原样返回 JSON。
func (c *Client) Usage(ctx context.Context) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	env, err := c.doJSON(ctx, http.MethodPost, "/open-api/v1/token-usage/details", map[string]any{})
	if err != nil {
		return nil, err
	}
	return env.Data, nil
}

func friendlyNetErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return errors.New("请求已取消")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("连接 BIC-QA 超时")
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "no such host"):
		return errors.New("无法解析 api.bic-qa.com，请检查网络或代理设置")
	case strings.Contains(msg, "connection refused"):
		return errors.New("连接被拒绝，请检查网络或代理设置")
	}
	return fmt.Errorf("连接 BIC-QA 失败：%w", err)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
