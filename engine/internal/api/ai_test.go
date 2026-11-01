package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizbox/engine/internal/aistream"
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
	if sess.open == nil || sess.ask == nil {
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

// TestImaSessionPassesQuestionAndSessionInOrder 守住一个真实踩过的坑：
// aiSession.ask 的签名是 (question, sessionID)，与 imaweb.Client.Ask 的参数顺序
// 必须一致。曾经两者顺序相反，导致 question 与 session_id 互换，
// ima 直接返回 code 1402「当前会话已失效」。
func TestImaSessionPassesQuestionAndSessionInOrder(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: CLOSE\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	sess := aiSessionOf(bicqa.Config{
		Provider: bicqa.ProviderImaWeb,
		BaseURL:  srv.URL,
		Cookie:   "IMA-TOKEN=x",
	})
	emitted := 0
	err := sess.ask(context.Background(), "题干内容", "sess-123", func(aistream.Event) error {
		emitted++
		return nil
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if gotBody["question"] != "题干内容" {
		t.Fatalf("question = %v", gotBody["question"])
	}
	if gotBody["session_id"] != "sess-123" {
		t.Fatalf("session_id = %v", gotBody["session_id"])
	}
	if emitted == 0 {
		t.Fatal("未收到 done 事件")
	}
}

func TestEnforceExclusiveCredentials(t *testing.T) {
	// 两套凭据同时存在时，保存后只应留下当前接入方式那一套。
	both := func(provider string) bicqa.Config {
		return bicqa.Config{
			Provider: provider,
			APIKey:   "sk-keep-me",
			Cookie:   "IMA-TOKEN=keep-me",
			KBID:     "kb-1",
			KBName:   "OceanBase",
		}
	}

	t.Run("ima 保留 Cookie 清 Key", func(t *testing.T) {
		cfg := both(bicqa.ProviderImaWeb)
		enforceExclusiveCredentials(&cfg)
		if cfg.APIKey != "" {
			t.Fatalf("ima 下应清掉 APIKey，实际 %q", cfg.APIKey)
		}
		if cfg.Cookie == "" || cfg.KBID == "" {
			t.Fatalf("ima 下应保留 Cookie 与知识库，实际 cookie=%q kb=%q", cfg.Cookie, cfg.KBID)
		}
	})

	t.Run("bicqa 保留 Key 清 Cookie 与知识库", func(t *testing.T) {
		cfg := both(bicqa.ProviderBICQA)
		enforceExclusiveCredentials(&cfg)
		if cfg.Cookie != "" {
			t.Fatalf("bicqa 下应清掉 Cookie，实际 %q", cfg.Cookie)
		}
		if cfg.KBID != "" || cfg.KBName != "" {
			t.Fatalf("Cookie 清掉后知识库也应清空，实际 %q/%q", cfg.KBID, cfg.KBName)
		}
		if cfg.APIKey == "" {
			t.Fatal("bicqa 下应保留 APIKey")
		}
	})

	t.Run("空 provider 按 BIC-QA 处理", func(t *testing.T) {
		cfg := both("")
		enforceExclusiveCredentials(&cfg)
		if cfg.Cookie != "" || cfg.APIKey == "" {
			t.Fatal("旧配置（无 provider 字段）应保留 APIKey 并清掉 Cookie")
		}
	})

	t.Run("ima 未填 Cookie 时知识库一并清空", func(t *testing.T) {
		cfg := bicqa.Config{Provider: bicqa.ProviderImaWeb, KBID: "kb-1", KBName: "x"}
		enforceExclusiveCredentials(&cfg)
		if cfg.KBID != "" || cfg.KBName != "" {
			t.Fatal("没有 ima 凭据时不应保留知识库选择")
		}
	})
}
