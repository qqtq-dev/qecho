package bicqa

import (
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

func TestMaskedKey(t *testing.T) {
	cfg := Config{APIKey: "sk-1234567890abcdef"}
	got := cfg.MaskedKey()
	if strings.Contains(got, "234567890abc") {
		t.Fatalf("key leaked: %q", got)
	}
	if !strings.HasPrefix(got, "sk-") || !strings.HasSuffix(got, "cdef") {
		t.Fatalf("mask shape = %q", got)
	}
	if (Config{APIKey: ""}).MaskedKey() != "" {
		t.Fatal("empty key should mask to empty string")
	}
}

func TestReadyRequiresKeyAndSwitch(t *testing.T) {
	if (Config{Enabled: true, APIKey: "sk-x"}).Ready() != true {
		t.Fatal("should be ready")
	}
	if (Config{Enabled: false, APIKey: "sk-x"}).Ready() != false {
		t.Fatal("disabled should not be ready")
	}
	if (Config{Enabled: true}).Ready() != false {
		t.Fatal("missing key should not be ready")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Enabled: true, BaseURL: "https://api.bic-qa.com/", APIKey: "sk-test"}
	if err := Save(dir, 7, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := Load(dir, 7)
	if !got.Enabled || got.APIKey != "sk-test" {
		t.Fatalf("round trip = %+v", got)
	}
	if got.BaseURL != "https://api.bic-qa.com" {
		t.Fatalf("baseUrl = %q", got.BaseURL)
	}
	if got.TimeoutSec != 300 {
		t.Fatalf("timeout default = %d", got.TimeoutSec)
	}
}

func TestConfigIsolatedPerUser(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, 1, Config{Enabled: true, APIKey: "sk-user1"}); err != nil {
		t.Fatalf("save user1: %v", err)
	}
	if err := Save(dir, 2, Config{Enabled: true, APIKey: "sk-user2"}); err != nil {
		t.Fatalf("save user2: %v", err)
	}

	if got := Load(dir, 1).APIKey; got != "sk-user1" {
		t.Fatalf("user1 key = %q", got)
	}
	if got := Load(dir, 2).APIKey; got != "sk-user2" {
		t.Fatalf("user2 key = %q", got)
	}
	// 未配置的用户拿不到别人的 Key
	other := Load(dir, 3)
	if other.APIKey != "" || other.Ready() {
		t.Fatalf("user3 should be empty, got %+v", other)
	}
	// 清除自己的 Key 不影响他人
	if err := Save(dir, 1, Config{Enabled: false}); err != nil {
		t.Fatalf("clear user1: %v", err)
	}
	if got := Load(dir, 2).APIKey; got != "sk-user2" {
		t.Fatalf("user2 key lost after clearing user1: %q", got)
	}
	if got := Load(dir, 1).APIKey; got != "" {
		t.Fatalf("user1 key should be cleared, got %q", got)
	}
	// 非法用户编号不应落盘
	if err := Save(dir, 0, Config{APIKey: "sk-x"}); err == nil {
		t.Fatal("expected error for invalid user id")
	}
}

func TestParseStreamSeparatesThinking(t *testing.T) {
	raw := strings.Join([]string{
		`data:{"type":"meta","chatSessionId":"abc","done":false}`,
		``,
		`data:{"type":"delta","delta":{"role":"assistant"},"done":false}`,
		``,
		`data:{"type":"delta","delta":{"thinking":{"text":"先看","type":"text"}},"done":false}`,
		``,
		`data:{"type":"delta","delta":{"content":"## 结论"}}`,
		``,
		`data:{"type":"done","done":true,"totalTokens":100}`,
		``,
	}, "\n")

	events := collect(t, raw)
	var thinking, answer strings.Builder
	var kinds []string
	for _, ev := range events {
		kinds = append(kinds, ev.Kind)
		switch ev.Kind {
		case "thinking":
			thinking.WriteString(ev.Text)
		case "delta":
			answer.WriteString(ev.Text)
		}
	}
	if thinking.String() != "先看" {
		t.Fatalf("thinking = %q", thinking.String())
	}
	if answer.String() != "## 结论" {
		t.Fatalf("answer = %q (kinds=%v)", answer.String(), kinds)
	}
	if len(events) == 0 || events[len(events)-1].Kind != "done" {
		t.Fatalf("last event should be done, got %v", kinds)
	}
	// role 帧没有正文，不应产生事件
	for _, ev := range events {
		if ev.Kind == "delta" && ev.Text == "" {
			t.Fatal("empty delta should be dropped")
		}
	}
}
