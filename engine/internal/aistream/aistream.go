// Package aistream 把各家 AI 问答服务的 SSE 流统一成同一组事件。
//
// 不同厂商的流式协议字段五花八门，但都逃不出三类载荷：增量正文、思考过程、
// 终止与错误。本包只做与厂商无关的那部分——按 SSE 规范切帧，再对每帧做宽松的
// JSON 提取；厂商特有的语义（例如某家把正文塞在 blockList[0].content 里）
// 由各自的客户端在 emit 回调里再加工。
package aistream

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// Event 是统一后的流式事件，Kind 取值 delta / thinking / meta / error / done。
// delta 是答案正文，thinking 是模型的思考过程（可选展示）。
type Event struct {
	Kind string
	Text string
}

// MaxRunes 限制单次回答累计正文字数，防止异常流把内存打满。
const MaxRunes = 200_000

// Parse 逐帧解析 SSE 响应并归一化成 Event 交给 emit。
//
// 服务端事件字段并不完全稳定，这里做了宽松处理：优先识别显式的 type / event
// 字段，其次从 delta.content 等常见位置取正文，完全解析不出来时退回
// “把原始载荷当正文”，避免前端一片空白。
func Parse(body io.Reader, emit func(Event) error) error {
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
		kind, text := Extract(payload)
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
			if total > MaxRunes {
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

// Extract 从单帧载荷中提取事件类型与正文。
func Extract(payload string) (kind, text string) {
	var m map[string]any
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return "delta", payload
	}
	return ExtractMap(m, payload)
}

// ExtractMap 与 Extract 相同，但接受已经解析好的对象，便于厂商客户端先做语义加工。
// raw 仅用于完全无法识别时兜底。
func ExtractMap(m map[string]any, raw string) (kind, text string) {
	if status, ok := m["status"].(string); ok {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "error", "failed", "fail", "unauthorized", "forbidden":
			msg := Message(m)
			if msg == "" {
				msg = "AI 服务返回错误"
			}
			return "error", msg
		}
	}
	if errVal, ok := m["error"]; ok && errVal != nil {
		msg := FirstText(errVal, 0)
		if msg == "" {
			// OpenAI 兼容协议把原因放在 error.message 里，FirstText 的候选键不含
			// message，取不到就只剩一句笼统的「AI 服务返回错误」，用户无从下手。
			if em, ok := errVal.(map[string]any); ok {
				msg = Message(em)
			}
		}
		if msg == "" {
			msg = Message(m)
		}
		if msg == "" {
			msg = "AI 服务返回错误"
		}
		return "error", msg
	}

	// OpenAI 兼容协议（DeepSeek、通义千问、智谱、Kimi 等均遵循这一套）把正文
	// 放在 choices[0] 里，外面套了一层数组，FirstText 的候选键够不到，必须单独处理。
	//   {"choices":[{"delta":{"content":"你"},"finish_reason":null}]}
	//   {"choices":[{"delta":{"reasoning_content":"思考中"},"finish_reason":null}]}
	//   {"choices":[{"delta":{},"finish_reason":"stop"}]}
	if choices, ok := m["choices"].([]any); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]any); ok {
			if d, ok := first["delta"].(map[string]any); ok {
				// reasoning_content 是 DeepSeek-R1 / QwQ 这类推理模型的思考过程，
				// 部分网关会简写成 reasoning。
				for _, k := range []string{"reasoning_content", "reasoning"} {
					if s, ok := d[k].(string); ok && strings.TrimSpace(s) != "" {
						return "thinking", s
					}
				}
				if s, ok := d["content"].(string); ok && s != "" {
					return "delta", s
				}
			}
			// 少数实现不按流式回，直接把完整结果放在 message 里。
			if msg, ok := first["message"].(map[string]any); ok {
				if s, ok := msg["content"].(string); ok && s != "" {
					return "delta", s
				}
			}
			// finish_reason 只在最后一帧出现，此时 delta 通常是空对象。
			if fr, ok := first["finish_reason"].(string); ok && strings.TrimSpace(fr) != "" {
				return "done", ""
			}
		}
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

	// 常见形态：正文与思考过程都放在 delta 里
	//   {"type":"delta","delta":{"content":"##"}}
	//   {"type":"delta","delta":{"thinking":{"text":"The","type":"text"}}}
	if d, ok := m["delta"].(map[string]any); ok {
		if th, ok := d["thinking"]; ok {
			if s := FirstText(th, 0); s != "" {
				return "thinking", s
			}
		}
		if s := FirstText(d, 0); s != "" {
			return "delta", s
		}
		if kind == "" {
			return "delta", ""
		}
	}

	text = FirstText(m, 0)
	if kind == "" {
		kind = "delta"
	}
	switch kind {
	case "error", "failed", "fail":
		kind = "error"
		if text == "" {
			text = Message(m)
		}
		if text == "" {
			text = "AI 服务返回错误"
		}
	case "done", "end", "finish", "complete", "completed":
		kind = "done"
		text = ""
	case "meta", "file", "attachment":
		kind = "meta"
		text = ""
	}
	if text == "" && kind == "delta" {
		text = Message(m)
	}
	if text == "" && kind == "delta" && raw != "" && !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		text = raw
	}
	return kind, text
}

// Message 从对象里挑一个像错误信息的字符串。
func Message(m map[string]any) string {
	for _, k := range []string{"message", "msg", "detail", "reason", "errorMsg"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// FirstText 递归找出对象里第一段可当正文的文本。
func FirstText(v any, depth int) string {
	if depth > 4 {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var b strings.Builder
		for _, it := range t {
			b.WriteString(FirstText(it, depth+1))
		}
		return b.String()
	case map[string]any:
		// 前四个是通用正文键；后面几个是 ima 这类「分段返回」的容器名，
		// 递归进去逐段取 content，避免正文整段丢失。
		for _, k := range []string{
			"content", "text", "delta", "answer",
			"blocks", "blockList", "paragraphs", "paragraphList",
		} {
			if sub, ok := t[k]; ok {
				if s := FirstText(sub, depth+1); s != "" {
					return s
				}
			}
		}
		if sub, ok := t["data"]; ok {
			return FirstText(sub, depth+1)
		}
	}
	return ""
}
