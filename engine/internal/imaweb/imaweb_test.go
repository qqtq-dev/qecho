package imaweb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"quizbox/engine/internal/aistream"
)

const testCookie = "IMA-UID=0019ff48; IMA-TOKEN=abc; UID-TYPE=2"

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, testCookie)
}

func TestNormalizeCookie(t *testing.T) {
	cases := map[string]string{
		"  IMA-TOKEN=abc  ":                          "IMA-TOKEN=abc",
		"x-ima-cookie: IMA-TOKEN=abc":                "IMA-TOKEN=abc",
		"Cookie: IMA-TOKEN=abc":                      "IMA-TOKEN=abc",
		"IMA-TOKEN=abc;\nIMA-UID=1":                  "IMA-TOKEN=abc; IMA-UID=1",
		"X-Ima-Cookie:  IMA-TOKEN=a; IMA-REFRESH=b ": "IMA-TOKEN=a; IMA-REFRESH=b",
		// 用户实际最常见的两种粘法：整条 cURL、DevTools 整行请求头。
		`curl 'https://ima.qq.com/cgi-bin/assistant/qa' -H 'x-ima-cookie: PLATFORM=H5; IMA-TOKEN=a=b; UID-TYPE=2' -H 'from_browser_ima: 1'`: "PLATFORM=H5; IMA-TOKEN=a=b; UID-TYPE=2",
		`x-ima-cookie: PLATFORM=H5; IMA-IUA=Mozilla/5.0 (Windows NT 10.0, Win64, x64); IMA-TOKEN=t`:                                         "PLATFORM=H5; IMA-IUA=Mozilla/5.0 (Windows NT 10.0, Win64, x64); IMA-TOKEN=t",
	}
	for in, want := range cases {
		if got := NormalizeCookie(in); got != want {
			t.Fatalf("NormalizeCookie(%q)\n got %q\nwant %q", in, got, want)
		}
	}
}

func TestCreateSessionOK(t *testing.T) {
	var gotPath, gotCookie, gotBrowser string
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCookie = r.Header.Get(headerCookie)
		gotBrowser = r.Header.Get(headerBrowser)
		_ = jsonDecode(r, &body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"msg":"ok","session_id":"sess-1"}`)
	})
	sid, err := c.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sid != "sess-1" {
		t.Fatalf("sessionId = %q", sid)
	}
	if gotPath != "/cgi-bin/session_logic/init_session" {
		t.Fatalf("path = %q", gotPath)
	}
	// 鉴权走自定义头，不是 Cookie 头。
	if gotCookie != testCookie {
		t.Fatalf("x-ima-cookie = %q", gotCookie)
	}
	if gotBrowser != "1" {
		t.Fatalf("from_browser_ima = %q", gotBrowser)
	}
	env, _ := body["envInfo"].(map[string]any)
	if env == nil {
		t.Fatalf("missing envInfo: %+v", body)
	}
	if env["robotType"] != float64(robotMain) || env["interactType"] != float64(interactChatPage) {
		t.Fatalf("envInfo = %+v", env)
	}
}

func TestCreateSessionCamelCaseSessionID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":0,"sessionId":"camel-9"}`)
	})
	sid, err := c.CreateSession(context.Background())
	if err != nil || sid != "camel-9" {
		t.Fatalf("sid=%q err=%v", sid, err)
	}
}

func TestCreateSessionLoginExpiredByBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// ima 的鉴权失败也是 HTTP 200，只能靠 code 判断。
		io.WriteString(w, `{"code":41,"msg":"登录失败，请重新登录"}`)
	})
	_, err := c.CreateSession(context.Background())
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("want ErrLoginRequired, got %v", err)
	}
}

func TestCreateSessionLoginExpiredByHeader(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerCode, "41")
		w.WriteHeader(http.StatusUnauthorized)
	})
	_, err := c.CreateSession(context.Background())
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("want ErrLoginRequired, got %v", err)
	}
}

func TestCreateSessionOtherCodeKeepsServerMessage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":51,"msg":"invalid InitSessionReq.EnvInfo: value is required"}`)
	})
	_, err := c.CreateSession(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid InitSessionReq") {
		t.Fatalf("err = %v", err)
	}
}

func TestEmptyCookieIsNotConfigured(t *testing.T) {
	c := NewClient("", "")
	if _, err := c.CreateSession(context.Background()); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
	if err := c.Ask(context.Background(), "q", "s", func(aistream.Event) error { return nil }); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestAskSendsExpectedBody(t *testing.T) {
	var body map[string]any
	var ctype, accept, cookie string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/assistant/qa" {
			t.Errorf("path = %q", r.URL.Path)
		}
		ctype = r.Header.Get("Content-Type")
		accept = r.Header.Get("Accept")
		cookie = r.Header.Get(headerCookie)
		_ = jsonDecode(r, &body)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: CLOSE\ndata: [DONE]\n\n")
	})
	err := c.Ask(context.Background(), "什么是索引", "sess-1", func(aistream.Event) error { return nil })
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if accept != "text/event-stream" {
		t.Fatalf("accept = %q", accept)
	}
	// 真实抓包确认：提问请求的 Content-Type 也是 text/event-stream。
	if ctype != "text/event-stream" {
		t.Fatalf("content-type = %q", ctype)
	}
	if cookie != testCookie {
		t.Fatalf("x-ima-cookie = %q", cookie)
	}
	if body["question"] != "什么是索引" || body["session_id"] != "sess-1" {
		t.Fatalf("body = %+v", body)
	}
	if body["robot_type"] != float64(robotMain) || body["question_type"] != float64(1) {
		t.Fatalf("body = %+v", body)
	}
	mi, _ := body["model_info"].(map[string]any)
	if mi == nil || mi["model_id"] != modelID {
		t.Fatalf("model_info = %+v", body["model_info"])
	}
}

func TestAskStreamsBlockMessages(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: QA_START\ndata: {\"title\":\"ima\"}\n\n")
		io.WriteString(w, "event: SESSION_START\ndata: {\"MsgSeqID\":\"x_1\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"loading\",\"Data\":{},\"Id\":\"loading\",\"UpdateType\":\"create\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"loading\",\"Data\":{},\"Id\":\"loading\",\"UpdateType\":\"delete\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"blockMessage\",\"Data\":{\"text_message\":{\"Text\":\"数据库索引\"}},\"Id\":\"b1\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"blockMessage\",\"Data\":{\"text_message\":{\"Text\":\"是一种辅助结构\"}},\"Id\":\"b1\"}\n\n")
		io.WriteString(w, "event: SUGGEST_QUESTION\ndata: [{\"Text\":\"追问\"}]\n\n")
		io.WriteString(w, "event: ATTACHED_BLOCK\ndata: {\"Type\":\"qaDownloadGuide\",\"Data\":{}}\n\n")
		io.WriteString(w, "event: COMPLETED\ndata: {\"Code\":0,\"Msg\":\"\"}\n\n")
		io.WriteString(w, "event: CLOSE\ndata: [DONE]\n\n")
	})
	var answer strings.Builder
	var sawDone bool
	err := c.Ask(context.Background(), "q", "s", func(ev aistream.Event) error {
		switch ev.Kind {
		case "delta":
			answer.WriteString(ev.Text)
		case "done":
			sawDone = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if got := answer.String(); got != "数据库索引是一种辅助结构" {
		t.Fatalf("answer = %q", got)
	}
	if !sawDone {
		t.Fatal("缺少 done 事件")
	}
}

func TestAskCompletedErrorCode(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: COMPLETED\ndata: {\"Code\":1001,\"Msg\":\"内容不合规\"}\n\n")
		io.WriteString(w, "event: CLOSE\ndata: [DONE]\n\n")
	})
	var msg string
	err := c.Ask(context.Background(), "q", "s", func(ev aistream.Event) error {
		if ev.Kind == "error" {
			msg = ev.Text
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if msg != "内容不合规" {
		t.Fatalf("error text = %q", msg)
	}
}

func TestAskLoginExpiredMidStream(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: COMPLETED\ndata: {\"Code\":41,\"Msg\":\"登录失败\"}\n\n")
	})
	err := c.Ask(context.Background(), "q", "s", func(aistream.Event) error { return nil })
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("want ErrLoginRequired, got %v", err)
	}
}

func TestAskJSONErrorBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":41,"msg":"登录失败，请重新登录"}`)
	})
	err := c.Ask(context.Background(), "q", "s", func(aistream.Event) error { return nil })
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("want ErrLoginRequired, got %v", err)
	}
}

func TestClientIDShape(t *testing.T) {
	id := clientID()
	parts := strings.Split(id, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[4]) != 12 {
		t.Fatalf("clientID = %q", id)
	}
	if id == clientID() {
		t.Fatal("clientID 不应重复")
	}
}

func jsonDecode(r *http.Request, v any) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

// TestAskKnowledgeBaseUsesRetrievalCommand 校验「按知识库问答」的请求形态：
// robot_type 切到 5，并带上 command_info.type=14 与 knowledge_ids。
func TestAskKnowledgeBaseUsesRetrievalCommand(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: CLOSE\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testCookie).WithKnowledgeBase("kb-1")
	if err := c.Ask(context.Background(), "题目", "s", func(aistream.Event) error { return nil }); err != nil {
		t.Fatalf("ask: %v", err)
	}
	if body["robot_type"] != float64(robotKnowledge) {
		t.Fatalf("robot_type = %v", body["robot_type"])
	}
	cmd, _ := body["command_info"].(map[string]any)
	if cmd == nil || cmd["type"] != float64(commandKQA) {
		t.Fatalf("command_info = %+v", body["command_info"])
	}
	info, _ := cmd["knowledge_qa_info"].(map[string]any)
	ids, _ := info["knowledge_ids"].([]any)
	if len(ids) != 1 || ids[0] != "kb-1" {
		t.Fatalf("knowledge_ids = %+v", info)
	}
}

// TestAskKnowledgeBaseNarrationGoesToThinking 校验工具旁白（英文短句、出现在
// 检索结果之前）被归到 thinking，答案正文才进 delta。
func TestAskKnowledgeBaseNarrationGoesToThinking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"loading\",\"Data\":{},\"Id\":\"loading\",\"UpdateType\":\"create\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"blockMessage\",\"Data\":{\"text_message\":{\"Text\":\"I'll search the knowledge base.\"}},\"Id\":\"p1\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"fileList\",\"Data\":{},\"Id\":\"call_1\",\"UpdateType\":\"update\"}\n\n")
		io.WriteString(w, "event: STRUCTURED_BLOCK\ndata: {\"Type\":\"blockMessage\",\"Data\":{\"text_message\":{\"Text\":\"OceanBase 的租户是隔离的\"}},\"Id\":\"a1\"}\n\n")
		io.WriteString(w, "event: CLOSE\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, testCookie).WithKnowledgeBase("kb-1")
	var answer, thinking strings.Builder
	err := c.Ask(context.Background(), "题目", "s", func(ev aistream.Event) error {
		switch ev.Kind {
		case "delta":
			answer.WriteString(ev.Text)
		case "thinking":
			thinking.WriteString(ev.Text)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if got := answer.String(); got != "OceanBase 的租户是隔离的" {
		t.Fatalf("answer = %q", got)
	}
	if !strings.Contains(thinking.String(), "I'll search") {
		t.Fatalf("thinking = %q", thinking.String())
	}
	if strings.Contains(answer.String(), "I'll") {
		t.Fatal("旁白混进了答案正文")
	}
}
