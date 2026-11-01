package bicqa

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// Event 是解析后的统一流式事件，Kind 取值 delta / thinking / meta / error / done。
// delta 是答案正文，thinking 是模型的思考过程（可选展示）。
type Event struct {
	Kind string
	Text string
}

const maxStreamRunes = 200_000

// ParseStream 解析 BIC-QA 的 SSE 响应，把 meta / delta / file / error / done
// 归一化成 Event 交给 emit。服务端事件字段并不完全稳定，这里做了宽松提取：
// 优先识别显式的 type / event 字段，其次从 delta.content 等常见位置取正文，
// 完全解析不出来时退回“把原始载荷当正文”，避免前端一片空白。
func ParseStream(body io.Reader, emit func(Event) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var eventName string
	var dataLines []string
	total := 0

	flush := func() error {
		if len(dataLines) == 0 {
			eventName = ""
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		name := eventName
		dataLines = dataLines[:0]
		eventName = ""

		if strings.TrimSpace(payload) == "[DONE]" {
			return emit(Event{Kind: "done"})
		}
		kind, text := extractEvent(payload)
		if name != "" {
			switch strings.ToLower(name) {
			case "delta", "meta", "error", "done", "file":
				if kind == "delta" && strings.ToLower(name) != "delta" {
					kind = strings.ToLower(name)
				}
			}
		}
		if kind == "delta" || kind == "thinking" {
			if text == "" {
				return nil
			}
		}
		if kind == "delta" {
			total += len([]rune(text))
			if total > maxStreamRunes {
				return emit(Event{Kind: "done"})
			}
		}
		return emit(Event{Kind: kind, Text: text})
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return err
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
		// 兼容没有 SSE 帧头、直接逐行输出 JSON 的实现。
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			dataLines = append(dataLines, strings.TrimSpace(line))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

func extractEvent(payload string) (kind, text string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return "delta", payload
	}

	if status, ok := m["status"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "error", "failed", "fail", "unauthorized", "forbidden":
			msg := messageOf(m)
			if msg == "" {
				msg = "BIC-QA 返回错误"
			}
			return "error", msg
		}
	}
	if errVal, ok := m["error"]; ok && errVal != nil {
		msg := firstText(errVal, 0)
		if msg == "" {
			msg = messageOf(m)
		}
		if msg == "" {
			msg = "BIC-QA 返回错误"
		}
		return "error", msg
	}

	if s, ok := m["type"].(string); ok && strings.TrimSpace(s) != "" {
		kind = strings.ToLower(strings.TrimSpace(s))
	}
	if s, ok := m["event"].(string); ok && kind == "" && strings.TrimSpace(s) != "" {
		kind = strings.ToLower(strings.TrimSpace(s))
	}
	if b, ok := m["done"].(bool); ok && b {
		kind = "done"
	}

	// BIC-QA 的正文与思考过程都放在 delta 里：
	//   {"type":"delta","delta":{"content":"##"}}
	//   {"type":"delta","delta":{"thinking":{"text":"The","type":"text"}}}
	// 只发正文，思考过程单独归类，供前端提示“正在思考”。
	if d, ok := m["delta"].(map[string]any); ok {
		if th, ok := d["thinking"]; ok {
			if s := firstText(th, 0); s != "" {
				return "thinking", s
			}
		}
		if s := firstText(d, 0); s != "" {
			return "delta", s
		}
		if kind == "" {
			return "delta", ""
		}
	}

	text = firstText(m, 0)
	if kind == "" {
		kind = "delta"
	}
	switch kind {
	case "error", "failed", "fail":
		kind = "error"
		if text == "" {
			text = messageOf(m)
		}
		if text == "" {
			text = "BIC-QA 返回错误"
		}
	case "done", "end", "finish", "complete", "completed":
		kind = "done"
		text = ""
	case "meta", "file", "attachment":
		kind = "meta"
		text = ""
	}
	if text == "" && kind == "delta" {
		text = messageOf(m)
	}
	return kind, text
}

func messageOf(m map[string]any) string {
	for _, k := range []string{"message", "msg", "detail", "reason"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func firstText(v any, depth int) string {
	if depth > 4 {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, it := range t {
			b.WriteString(firstText(it, depth+1))
		}
		return b.String()
	case map[string]any:
		for _, k := range []string{"content", "text", "delta", "answer"} {
			if sub, ok := t[k]; ok {
				if s := firstText(sub, depth+1); s != "" {
					return s
				}
			}
		}
		if sub, ok := t["data"]; ok {
			return firstText(sub, depth+1)
		}
	}
	return ""
}
