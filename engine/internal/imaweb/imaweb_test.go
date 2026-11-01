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

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "IMA-TOKEN=abc; other=1")
}

func TestCreateSessionOK(t *testing.T) {
	var gotPath, gotCookie, gotAccept string
	var body map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotCookie = r.Header.Get("Cookie")
		gotAccept = r.Header.Get("Origin")
		_ = jsonDecode(r, &body)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":0,"sessionId":"sess-1"}`)
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
	if gotCookie != "IMA-TOKEN=abc; other=1" {
		t.Fatalf("cookie = %q", gotCookie)
	}
	if !strings.HasPrefix(gotAccept, "http://") {
		t.Fatalf("origin = %q", gotAccept)
	}
	env, _ := body["envInfo"].(map[string]any)
	if env == nil {
		t.Fatalf("missing envInfo: %+v", body)
	}
	if env["robotType"] != float64(robotMain) || env["interactType"] != float64(interactChatPage) {
		t.Fatalf("envInfo = %+v", env)
	}
}

func TestCreateSessionNestedSessionInfo(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"code":0,"data":{"sessionInfo":{"id":"nested-9"}}}`)
	})
	sid, err := c.CreateSession(context.Background())
	if err != nil || sid != "nested-9" {
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
		w.Header().Set("Trpc-Func-Ret", "41")
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
	if _, err := c.ChatStream(context.Background(), "q", "s"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}

func TestChatStreamSendsExpectedBody(t *testing.T) {
	var body map[string]any
	var accept string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/assistant/qa" {
			t.Errorf("path = %q", r.URL.Path)
		}
		accept = r.Header.Get("Accept")
		_ = jsonDecode(r, &body)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "event: add\ndata: {\"content\":\"索引\"}\n\n")
		io.WriteString(w, "event: done\ndata: {\"done\":true}\n\n")
	})
	rc, err := c.ChatStream(context.Background(), "什么是索引", "sess-1")
	if err != nil {
		t.Fatalf("chat stream: %v", err)
	}
	defer rc.Close()
	if accept != "text/event-stream" {
		t.Fatalf("accept = %q", accept)
	}
	if body["question"] != "什么是索引" || body["sessionId"] != "sess-1" {
		t.Fatalf("body = %+v", body)
	}
	if body["robotType"] != float64(robotMain) {
		t.Fatalf("robotType = %v", body["robotType"])
	}

	var answer strings.Builder
	var sawDone bool
	if err := ParseStream(rc, func(ev aistream.Event) error {
		switch ev.Kind {
		case "delta":
			answer.WriteString(ev.Text)
		case "done":
			sawDone = true
		}
		return nil
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if answer.String() != "索引" {
		t.Fatalf("answer = %q", answer.String())
	}
	if !sawDone {
		t.Fatal("missing done event")
	}
}

func TestChatStreamJSONErrorBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"code":41,"msg":"登录失败，请重新登录"}`)
	})
	_, err := c.ChatStream(context.Background(), "q", "s")
	if !errors.Is(err, ErrLoginRequired) {
		t.Fatalf("want ErrLoginRequired, got %v", err)
	}
}

func jsonDecode(r *http.Request, v any) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
