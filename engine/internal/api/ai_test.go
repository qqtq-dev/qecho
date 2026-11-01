package api

import (
	"strings"
	"testing"
)

func TestWithAnswerMode(t *testing.T) {
	const q = "题干：OceanBase 的副本类型有哪几种？"

	cases := []struct {
		name     string
		mode     string
		wantHint string
	}{
		{"默认简答", "", answerHintBrief},
		{"显式简答", "brief", answerHintBrief},
		{"详答", "full", answerHintFull},
		{"未知值按简答处理", "whatever", answerHintBrief},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := withAnswerMode(q, c.mode)
			if !strings.HasPrefix(got, q) {
				t.Fatalf("原问题被改动：%q", got)
			}
			if !strings.HasSuffix(got, c.wantHint) {
				t.Fatalf("期望追加 %q，实际 %q", c.wantHint, got[len(q):])
			}
		})
	}

	// 详答不应被简答的 200 字限制污染
	if strings.Contains(answerHintFull, "200 字") {
		t.Fatal("详答提示词不应包含字数限制")
	}
	if !strings.Contains(answerHintBrief, "200 字") {
		t.Fatal("简答提示词应包含字数限制")
	}
}
