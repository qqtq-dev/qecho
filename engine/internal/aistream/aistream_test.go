package aistream

import (
	"strings"
	"testing"
)

func collect(t *testing.T, raw string) []Event {
	t.Helper()
	var out []Event
	if err := Parse(strings.NewReader(raw), func(ev Event) error {
		out = append(out, ev)
		return nil
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return out
}

func TestExtractVendorShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		kind string
		text string
	}{
		{"BIC-QA delta.content", `{"type":"delta","delta":{"content":"索引"}}`, "delta", "索引"},
		{"嵌套 data 信封", `{"status":"success","data":{"delta":{"content":"回表"}}}`, "delta", "回表"},
		{"thinking 单独归类", `{"type":"delta","delta":{"thinking":{"text":"先看"}}}`, "thinking", "先看"},
		{"done 标记", `{"type":"done","done":true}`, "done", ""},
		{"错误信息", `{"type":"error","message":"额度不足"}`, "error", "额度不足"},
		// ima 可能把正文放在数组里（blockList / paragraphs），FirstText 要能串起来
		{"数组正文", `{"blocks":[{"content":"第一段"},{"content":"第二段"}]}`, "delta", "第一段第二段"},
		// OpenAI 兼容协议：正文被 choices 数组包了一层
		{"OpenAI content", `{"choices":[{"index":0,"delta":{"content":"索引"},"finish_reason":null}]}`, "delta", "索引"},
		{"OpenAI 思考过程", `{"choices":[{"delta":{"reasoning_content":"先看题干"}}]}`, "thinking", "先看题干"},
		{"OpenAI 结束帧", `{"choices":[{"delta":{},"finish_reason":"stop"}]}`, "done", ""},
		{"OpenAI 非流式", `{"choices":[{"message":{"role":"assistant","content":"完整回答"}}]}`, "delta", "完整回答"},
		{"OpenAI 错误体", `{"error":{"message":"invalid api key","type":"auth"}}`, "error", "invalid api key"},
		{"非 JSON 走原文", `纯文本回答`, "delta", "纯文本回答"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kind, text := Extract(c.raw)
			if kind != c.kind || text != c.text {
				t.Fatalf("got (%q,%q), want (%q,%q)", kind, text, c.kind, c.text)
			}
		})
	}
}

func TestParseDropsEmptyDelta(t *testing.T) {
	events := collect(t, "data:{\"type\":\"delta\",\"delta\":{\"role\":\"assistant\"}}\n\n")
	for _, ev := range events {
		if ev.Kind == "delta" && ev.Text == "" {
			t.Fatalf("空 delta 应被丢弃: %+v", events)
		}
	}
}

func TestParseOpenAIStream(t *testing.T) {
	// 真实 OpenAI 兼容流的典型形态：首帧只带 role，中间是正文或思考过程，
	// 倒数第二帧 delta 为空但带 finish_reason，最后一行是 [DONE]。
	raw := strings.Join([]string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"index":0,"delta":{"reasoning_content":"先判断题干"},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"index":0,"delta":{"content":"选 A。"},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"index":0,"delta":{"content":"因为 B 项描述错误。"},"finish_reason":null}]}`,
		``,
		`data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	var answer, thinking string
	var done int
	for _, ev := range collect(t, raw) {
		switch ev.Kind {
		case "delta":
			answer += ev.Text
		case "thinking":
			thinking += ev.Text
		case "done":
			done++
		}
	}
	if answer != "选 A。因为 B 项描述错误。" {
		t.Fatalf("正文拼接错误：%q", answer)
	}
	if thinking != "先判断题干" {
		t.Fatalf("思考过程未归入 thinking：%q", thinking)
	}
	if done == 0 {
		t.Fatal("未收到 done 事件")
	}
}

func TestParseTruncatesRunawayStream(t *testing.T) {
	var b strings.Builder
	chunk := strings.Repeat("字", 10_000)
	for i := 0; i < 30; i++ {
		b.WriteString(`data:{"delta":{"content":"` + chunk + `"}}`)
		b.WriteString("\n\n")
	}
	var total int
	var sawDone bool
	if err := Parse(strings.NewReader(b.String()), func(ev Event) error {
		switch ev.Kind {
		case "delta":
			total += len([]rune(ev.Text))
		case "done":
			sawDone = true
		}
		return nil
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !sawDone {
		t.Fatal("超长流应以 done 收尾")
	}
	if total > MaxRunes+10_000 {
		t.Fatalf("累计正文 %d 字，未被截断", total)
	}
}
