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
