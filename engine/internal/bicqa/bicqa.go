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
	configDirName  = "bicqa"

	// ProviderBicQA 走 BIC-QA 私有协议（服务端存会话、有额度接口）。
	ProviderBicQA = "bicqa"
	// ProviderOpenAI 走 OpenAI 兼容协议，覆盖百炼、DeepSeek、智谱、Kimi 等。
	ProviderOpenAI = "openai"

	// MaxQuestionRunes 限制单次提问长度，避免把整份题图或超长文本误发出去。
	MaxQuestionRunes = 4000

	// ImaLegacyBaseURL 是已移除的 ima 网页版地址，仅用于识别并纠正历史配置。
	ImaLegacyBaseURL = "https://ima.qq.com"
)

// ErrNotConfigured 表示尚未启用或缺少 API Key。
var ErrNotConfigured = errors.New("未配置 BIC-QA API Key")

// Channel 是一条 AI 通道的完整配置。两条通道各自独立保存，
// 来回切换服务商时不需要重新填 Key、地址和模型。
type Channel struct {
	Enabled    bool   `json:"enabled"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	Model      string `json:"model"`
	TimeoutSec int    `json:"timeoutSec"`
}

// Config 是一份完整配置。Active 决定哪条通道生效——两条只能同时生效一条，
// 另一条只是存着，不参与任何请求。
type Config struct {
	Active string  `json:"active"`
	BicQA  Channel `json:"bicqa"`
	OpenAI Channel `json:"openai"`
}

// UnmarshalJSON 兼容旧版扁平配置。provider 字段是演进过程中出现过的形态，
// 早期文件连它都没有，直接是 enabled / baseUrl / apiKey 平铺在顶层。
// 这类文件一律迁移成「BIC-QA 通道生效」，保证升级后行为不变。
func (c *Config) UnmarshalJSON(raw []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return err
	}
	if _, ok := probe["bicqa"]; ok {
		type plain Config // 避免递归调用本方法
		var p plain
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		*c = Config(p)
		return nil
	}

	var legacy struct {
		Channel
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return err
	}
	// 中途出现过带 provider 的扁平结构，如实迁移到对应通道，别把用户
	// 选过的通用模型硬拽回 BIC-QA。
	if strings.EqualFold(strings.TrimSpace(legacy.Provider), ProviderOpenAI) {
		*c = Config{Active: ProviderOpenAI, OpenAI: legacy.Channel}
		return nil
	}
	*c = Config{Active: ProviderBicQA, BicQA: legacy.Channel}
	return nil
}

// Current 返回当前生效的通道。
func (c Config) Current() Channel {
	if c.Active == ProviderOpenAI {
		return c.OpenAI
	}
	return c.BicQA
}

// WithCurrent 把改动写回当前通道，另一条原样保留。
func (c Config) WithCurrent(ch Channel) Config {
	if c.Active == ProviderOpenAI {
		c.OpenAI = ch
	} else {
		c.BicQA = ch
	}
	return c
}

// IsOpenAI 表示当前生效的是通用大模型。
func (c Config) IsOpenAI() bool {
	return c.Active == ProviderOpenAI
}

// Normalized 返回补齐默认值后的副本。读取配置和回显配置前都要过一遍，
// 否则界面上会出现空的服务地址（实际保存时用的是补全后的值）。
func (c Config) Normalized() Config {
	return c.normalized()
}

func (c Config) normalized() Config {
	c.Active = normalizeProvider(c.Active)
	c.BicQA = c.BicQA.normalized(ProviderBicQA)
	c.OpenAI = c.OpenAI.normalized(ProviderOpenAI)
	return c
}

// normalized 按通道补齐默认值。BIC-QA 有官方地址可以回落；通用模型各家域名
// 和版本段都不一样，猜不出来，只能留空交给 Ready 拦下并由前端提示去选预设，
// 绝不能回落到 BIC-QA 的地址——那会让人以为配好了却一直失败。
func (ch Channel) normalized(provider string) Channel {
	ch.BaseURL = strings.TrimRight(strings.TrimSpace(ch.BaseURL), "/")
	if provider == ProviderBicQA {
		// 早期版本支持过 ima 网页版，那条接入方式用的是另一套私有协议与域名。
		// 它的地址留到 BIC-QA 下必然请求不到，读取时直接纠正为官方默认地址。
		if ch.BaseURL == "" || strings.EqualFold(ch.BaseURL, ImaLegacyBaseURL) {
			ch.BaseURL = DefaultBaseURL
		}
	} else if strings.EqualFold(ch.BaseURL, ImaLegacyBaseURL) {
		ch.BaseURL = ""
	}
	ch.APIKey = strings.TrimSpace(ch.APIKey)
	ch.Model = strings.TrimSpace(ch.Model)
	if ch.TimeoutSec < 30 || ch.TimeoutSec > 900 {
		ch.TimeoutSec = 300
	}
	return ch
}

// normalizeProvider 把未知值一律归到 bicqa，避免手工改配置文件写错时整条功能失效。
func normalizeProvider(p string) string {
	if strings.EqualFold(strings.TrimSpace(p), ProviderOpenAI) {
		return ProviderOpenAI
	}
	return ProviderBicQA
}

// Ready 表示当前生效的那条通道已启用且配置足以发起请求。
// 通用模型必须同时有服务地址和模型名，少一个请求就会被服务商打回。
func (c Config) Ready() bool {
	ch := c.Current()
	if !ch.Enabled || ch.APIKey == "" {
		return false
	}
	if c.IsOpenAI() {
		return ch.BaseURL != "" && ch.Model != ""
	}
	return true
}

// MaskedKey 返回脱敏后的 Key，仅用于前端展示，绝不回传明文。
func (ch Channel) MaskedKey() string {
	key := strings.TrimSpace(ch.APIKey)
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
	// 通道的默认值统一交给 normalized 补，这里不预置，否则旧结构迁移后
	// 会被预置值覆盖成 BIC-QA 的地址。
	cfg := Config{}
	if userID <= 0 {
		return cfg.normalized()
	}
	raw, err := os.ReadFile(configPath(dir, userID))
	if err != nil {
		return cfg.normalized()
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}.normalized()
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

func NewClient(ch Channel) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(ch.BaseURL), "/"),
		apiKey:  strings.TrimSpace(ch.APIKey),
		model:   strings.TrimSpace(ch.Model),
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
