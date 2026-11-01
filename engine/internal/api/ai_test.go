package api

import (
	"strings"
	"testing"

	"quizbox/engine/internal/bicqa"
)

func TestNormalizeCookie(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"空值", "   ", ""},
		{"纯 Cookie 串", "IMA-TOKEN=abc; uid=1", "IMA-TOKEN=abc; uid=1"},
		{"带 Cookie 前缀", "Cookie: IMA-TOKEN=abc", "IMA-TOKEN=abc"},
		{"前缀大小写不敏感", "cookie:IMA-TOKEN=abc", "IMA-TOKEN=abc"},
		{"含换行", "IMA-TOKEN=abc;\r\nuid=1", "IMA-TOKEN=abc; uid=1"},
		{"多余空格", "IMA-TOKEN=abc;   uid=1", "IMA-TOKEN=abc; uid=1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeCookie(c.in); got != c.want {
				t.Fatalf("normalizeCookie(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestProviderSwitchKeepsCustomBaseURL(t *testing.T) {
	// 默认地址在切换接入方式时应跟着换，自定义地址（例如走代理）必须保留。
	if providerDefaultBaseURL(bicqa.ProviderImaWeb) != bicqa.ImaWebDefaultBaseURL {
		t.Fatal("ima 默认地址不一致")
	}
	if providerDefaultBaseURL(bicqa.ProviderBICQA) != bicqa.DefaultBaseURL {
		t.Fatal("BIC-QA 默认地址不一致")
	}
}

func TestAISessionDispatch(t *testing.T) {
	imaCfg := bicqa.Config{Provider: bicqa.ProviderImaWeb, Cookie: "IMA-TOKEN=x", TimeoutSec: 300}
	sess := aiSessionOf(imaCfg)
	if sess.provider != bicqa.ProviderImaWeb || sess.label != "ima 网页版" {
		t.Fatalf("ima dispatch = %+v", sess)
	}
	if sess.open == nil || sess.stream == nil || sess.parse == nil {
		t.Fatal("ima session 缺少回调")
	}

	// 旧配置文件没有 provider 字段，必须继续走 BIC-QA。
	legacy := bicqa.Config{APIKey: "sk-x", TimeoutSec: 300}
	sess = aiSessionOf(legacy)
	if sess.provider != bicqa.ProviderBICQA {
		t.Fatalf("legacy dispatch = %+v", sess)
	}
}

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
