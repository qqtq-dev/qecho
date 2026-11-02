package bicqa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func collect(t *testing.T, raw string) []Event {
	t.Helper()
	var out []Event
	err := ParseStream(strings.NewReader(raw), func(ev Event) error {
		out = append(out, ev)
		return nil
	})
	if err != nil {
		t.Fatalf("parse stream: %v", err)
	}
	return out
}

func TestParseStreamDeltaVariants(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "event+delta.content",
			raw:  "event: delta\ndata: {\"delta\":{\"content\":\"索引用不上\"}}\n\n",
			want: "索引用不上",
		},
		{
			name: "type+content",
			raw:  "data: {\"type\":\"delta\",\"content\":\"MVCC\"}\n\n",
			want: "MVCC",
		},
		{
			name: "nested data envelope",
			raw:  "data: {\"status\":\"success\",\"data\":{\"delta\":{\"content\":\"回表\"}}}\n\n",
			want: "回表",
		},
		{
			name: "bare json line",
			raw:  "{\"delta\":\"执行计划\"}\n",
			want: "执行计划",
		},
		{
			name: "multi line data",
			raw:  "data: {\"content\":\"第一行\"}\ndata: \n\n",
			want: "第一行",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			events := collect(t, c.raw)
			var sb strings.Builder
			for _, ev := range events {
				if ev.Kind == "delta" {
					sb.WriteString(ev.Text)
				}
			}
			if sb.String() != c.want {
				t.Fatalf("text = %q, want %q (events=%+v)", sb.String(), c.want, events)
			}
		})
	}
}

func TestParseStreamErrorAndDone(t *testing.T) {
	events := collect(t, "data: {\"status\":\"error\",\"message\":\"API Key 无效\"}\n\n")
	if len(events) == 0 || events[0].Kind != "error" {
		t.Fatalf("expected error event, got %+v", events)
	}
	if !strings.Contains(events[0].Text, "API Key 无效") {
		t.Fatalf("error text = %q", events[0].Text)
	}

	events = collect(t, "data: {\"type\":\"error\",\"message\":\"额度不足\"}\n\n")
	if len(events) == 0 || events[0].Kind != "error" || !strings.Contains(events[0].Text, "额度不足") {
		t.Fatalf("expected error event, got %+v", events)
	}

	events = collect(t, "event: done\ndata: {}\n\n")
	if len(events) == 0 || events[0].Kind != "done" {
		t.Fatalf("expected done event, got %+v", events)
	}

	events = collect(t, "data: [DONE]\n\n")
	if len(events) == 0 || events[0].Kind != "done" {
		t.Fatalf("expected done for [DONE], got %+v", events)
	}
}

func TestParseStreamFallsBackToRawText(t *testing.T) {
	events := collect(t, "data: 纯文本回答\n\n")
	if len(events) != 1 || events[0].Kind != "delta" || events[0].Text != "纯文本回答" {
		t.Fatalf("fallback failed: %+v", events)
	}
}

// 两条通道各自独立的构造辅助，让测试读起来更清楚。
func bicQAConfig(ch Channel) Config {
	return Config{Active: ProviderBicQA, BicQA: ch}
}

func openAIConfig(ch Channel) Config {
	return Config{Active: ProviderOpenAI, OpenAI: ch}
}

func TestMaskedKey(t *testing.T) {
	got := Channel{APIKey: "sk-1234567890abcdef"}.MaskedKey()
	if strings.Contains(got, "234567890abc") {
		t.Fatalf("key leaked: %q", got)
	}
	if !strings.HasPrefix(got, "sk-") || !strings.HasSuffix(got, "cdef") {
		t.Fatalf("mask shape = %q", got)
	}
	if (Channel{APIKey: ""}).MaskedKey() != "" {
		t.Fatal("empty key should mask to empty string")
	}
}

func TestReadyRequiresKeyAndSwitch(t *testing.T) {
	if !bicQAConfig(Channel{Enabled: true, APIKey: "sk-x"}).Ready() {
		t.Fatal("should be ready")
	}
	if bicQAConfig(Channel{Enabled: false, APIKey: "sk-x"}).Ready() {
		t.Fatal("disabled should not be ready")
	}
	if bicQAConfig(Channel{Enabled: true}).Ready() {
		t.Fatal("missing key should not be ready")
	}
}

// TestChannelsAreIndependent 守住核心约束：两条通道各自保存配置，
// 切换生效的那条不影响另一条，且同一时刻只有一条参与请求。
func TestChannelsAreIndependent(t *testing.T) {
	dir := t.TempDir()

	cfg := bicQAConfig(Channel{Enabled: true, APIKey: "sk-bicqa", BaseURL: "https://api.bic-qa.com"})
	if err := Save(dir, 5, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	// 切到通用模型，写它自己那份配置
	cfg.Active = ProviderOpenAI
	cfg.OpenAI = Channel{
		Enabled: true, APIKey: "sk-aliyun",
		BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Model: "qwen-plus",
	}
	if err := Save(dir, 5, cfg); err != nil {
		t.Fatalf("save openai: %v", err)
	}

	got := Load(dir, 5)
	if got.Active != ProviderOpenAI || !got.IsOpenAI() {
		t.Fatalf("生效通道应为 openai，实际 %q", got.Active)
	}
	if got.Current().APIKey != "sk-aliyun" {
		t.Fatalf("当前通道 Key = %q", got.Current().APIKey)
	}
	// BIC-QA 那条必须原样保留
	if got.BicQA.APIKey != "sk-bicqa" || !got.BicQA.Enabled {
		t.Fatalf("另一条通道被改动：%+v", got.BicQA)
	}

	// 切回 BIC-QA 不需要重填任何东西
	got.Active = ProviderBicQA
	if err := Save(dir, 5, got); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	back := Load(dir, 5)
	if back.Current().APIKey != "sk-bicqa" {
		t.Fatalf("切回后 Key 丢失：%q", back.Current().APIKey)
	}
	if back.OpenAI.Model != "qwen-plus" {
		t.Fatalf("通用模型配置丢失：%+v", back.OpenAI)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := bicQAConfig(Channel{Enabled: true, BaseURL: "https://api.bic-qa.com/", APIKey: "sk-test"})
	if err := Save(dir, 7, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load(dir, 7)
	ch := got.Current()
	if !ch.Enabled || ch.APIKey != "sk-test" {
		t.Fatalf("round trip = %+v", ch)
	}
	if ch.BaseURL != "https://api.bic-qa.com" {
		t.Fatalf("baseUrl = %q", ch.BaseURL)
	}
	if ch.TimeoutSec != 300 {
		t.Fatalf("timeout default = %d", ch.TimeoutSec)
	}
}

func TestConfigIsolatedPerUser(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, 1, bicQAConfig(Channel{Enabled: true, APIKey: "sk-user1"})); err != nil {
		t.Fatalf("save user1: %v", err)
	}
	if err := Save(dir, 2, bicQAConfig(Channel{Enabled: true, APIKey: "sk-user2"})); err != nil {
		t.Fatalf("save user2: %v", err)
	}

	if got := Load(dir, 1).Current().APIKey; got != "sk-user1" {
		t.Fatalf("user1 key = %q", got)
	}
	if got := Load(dir, 2).Current().APIKey; got != "sk-user2" {
		t.Fatalf("user2 key = %q", got)
	}
	// 未配置的用户拿不到别人的 Key
	other := Load(dir, 3)
	if other.Current().APIKey != "" || other.Ready() {
		t.Fatalf("user3 should be empty, got %+v", other)
	}
	// 清除自己的 Key 不影响他人
	if err := Save(dir, 1, bicQAConfig(Channel{Enabled: false})); err != nil {
		t.Fatalf("clear user1: %v", err)
	}
	if got := Load(dir, 2).Current().APIKey; got != "sk-user2" {
		t.Fatalf("user2 key lost after clearing user1: %q", got)
	}
	if got := Load(dir, 1).Current().APIKey; got != "" {
		t.Fatalf("user1 key should be cleared, got %q", got)
	}
	// 非法用户编号不应落盘
	if err := Save(dir, 0, bicQAConfig(Channel{APIKey: "sk-x"})); err == nil {
		t.Fatal("expected error for invalid user id")
	}
}

// TestLegacyImaBaseURLIsReset 守住升级兼容：早期配置里可能存着已移除的
// ima 网页版地址，那条地址在 BIC-QA 下请求不到，读取时必须纠正为默认值。
func TestLegacyImaBaseURLIsReset(t *testing.T) {
	for _, in := range []string{"https://ima.qq.com", "https://ima.qq.com/", "HTTPS://IMA.QQ.COM"} {
		cfg := bicQAConfig(Channel{BaseURL: in, APIKey: "sk-x"}).Normalized()
		if cfg.Current().BaseURL != DefaultBaseURL {
			t.Fatalf("baseUrl=%q 应纠正为 %q，实际 %q", in, DefaultBaseURL, cfg.Current().BaseURL)
		}
	}
	// 用户自填的其他地址（例如代理）必须保留。
	custom := bicQAConfig(Channel{BaseURL: "https://proxy.example.com/api", APIKey: "sk-x"}).Normalized()
	if custom.Current().BaseURL != "https://proxy.example.com/api" {
		t.Fatalf("自定义地址不应被改动，实际 %q", custom.Current().BaseURL)
	}
}

// TestLegacyFlatConfigMigrates 守住升级兼容：最早的配置文件是扁平结构，
// 连 provider 字段都没有，读取时必须迁移成「BIC-QA 通道生效」。
func TestLegacyFlatConfigMigrates(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"enabled":true,"baseUrl":"","apiKey":"sk-old","timeoutSec":300}`)
	path := configPath(dir, 9)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	got := Load(dir, 9)
	if got.Active != ProviderBicQA {
		t.Fatalf("老配置应归为 bicqa，实际 %q", got.Active)
	}
	if got.Current().BaseURL != DefaultBaseURL {
		t.Fatalf("老配置应补默认地址，实际 %q", got.Current().BaseURL)
	}
	if !got.Ready() {
		t.Fatal("老配置升级后应仍然可用")
	}
	// 通用模型通道应保持空白，不参与请求
	if got.OpenAI.Enabled || got.OpenAI.APIKey != "" {
		t.Fatalf("通用模型通道应保持空白：%+v", got.OpenAI)
	}
}

func TestOpenAIConfigNeedsAddressAndModel(t *testing.T) {
	// 通用模型没有官方默认地址，绝不能回落到 BIC-QA 的域名
	cfg := openAIConfig(Channel{Enabled: true, APIKey: "sk-x"}).Normalized()
	if cfg.Current().BaseURL != "" {
		t.Fatalf("openai 模式不应补默认地址，实际 %q", cfg.Current().BaseURL)
	}
	if cfg.Ready() {
		t.Fatal("缺服务地址时不应 Ready")
	}

	cfg = openAIConfig(Channel{
		Enabled: true, APIKey: "sk-x",
		BaseURL: "https://api.deepseek.com/v1",
	}).Normalized()
	if cfg.Ready() {
		t.Fatal("缺模型名时不应 Ready")
	}

	cfg = openAIConfig(Channel{
		Enabled: true, APIKey: "sk-x",
		BaseURL: "https://api.deepseek.com/v1/", Model: "deepseek-chat",
	}).Normalized()
	if !cfg.Ready() {
		t.Fatal("配置齐全时应 Ready")
	}
	if cfg.Current().BaseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("末尾斜杠应被清掉，实际 %q", cfg.Current().BaseURL)
	}

	// 未知通道一律归到 bicqa，避免手工改配置文件写错导致整条功能失效
	if p := (Config{Active: "gpt-like"}).Normalized().Active; p != ProviderBicQA {
		t.Fatalf("未知通道应归为 bicqa，实际 %q", p)
	}
}

// TestLegacyProviderFieldMigrates 中途出现过带 provider 的扁平结构。
// 迁移时必须落到对应通道，而不是一律塞进 BIC-QA。
func TestLegacyProviderFieldMigrates(t *testing.T) {
	dir := t.TempDir()
	legacy := []byte(`{"provider":"openai","enabled":true,"baseUrl":"https://api.deepseek.com/v1","apiKey":"sk-ds","model":"deepseek-chat"}`)
	path := configPath(dir, 11)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := Load(dir, 11)
	if got.Active != ProviderOpenAI {
		t.Fatalf("应迁到 openai 通道，实际 %q", got.Active)
	}
	if got.OpenAI.Model != "deepseek-chat" || got.OpenAI.APIKey != "sk-ds" {
		t.Fatalf("通用模型配置未迁移：%+v", got.OpenAI)
	}
	if got.BicQA.APIKey != "" {
		t.Fatalf("BIC-QA 通道应为空：%+v", got.BicQA)
	}
}
