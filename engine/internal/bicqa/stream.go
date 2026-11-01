package bicqa

import (
	"io"

	"quizbox/engine/internal/aistream"
)

// Event 是解析后的统一流式事件，Kind 取值 delta / thinking / meta / error / done。
// delta 是答案正文，thinking 是模型的思考过程（可选展示）。
type Event = aistream.Event

// ParseStream 解析 BIC-QA 的 SSE 响应，把 meta / delta / file / error / done
// 归一化成 Event 交给 emit。
//
// 帧切分与宽松字段提取是各厂商通用的部分，已抽到 internal/aistream，
// 这里只保留 BIC-QA 这一层的入口，避免上层契约随重构变化。
func ParseStream(body io.Reader, emit func(Event) error) error {
	return aistream.Parse(body, emit)
}
