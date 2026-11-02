package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpoint(t *testing.T) {
	cases := []struct {
		base string
		want string
	}{
		{"https://api.deepseek.com/v1", "https://api.deepseek.com/v1/chat/completions"},
		{"https://open.bigmodel.cn/api/paas/v4/", "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		// 用户按别家文档把完整路径贴进来时不要重复拼接
		{"https://x.test/v1/chat/completions", "https://x.test/v1/chat/completions"},
	}
	for _, c := range cases {
		if got := NewClient(c.base, "k", "m").Endpoint(); got != c.want {
			t.Errorf("base=%q got=%q want=%q", c.base, got, c.want)
		}
	}
}

func TestPingSuccess(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"p"}}]}`)
	}))
	defer srv.Close()

	if err := NewClient(srv.URL+"/v1", "sk-test", "qwen-plus").Ping(context.Background()); err != nil {
		t.Fatalf("Ping 应成功：%v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("鉴权头错误：%q", gotAuth)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("请求路径错误：%q", gotPath)
	}
}

func TestPingSurfacesVendorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error"}}`)
	}))
	defer srv.Close()

	err := NewClient(srv.URL+"/v1", "bad", "m").Ping(context.Background())
	if err == nil {
		t.Fatal("鉴权失败时应返回错误")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Incorrect API key") {
		t.Fatalf("错误信息未透出服务商原文：%q", msg)
	}
	// 401 应给出可操作的提示，而不是只报状态码
	if !strings.Contains(msg, "API Key") {
		t.Fatalf("401 未提示检查 Key：%q", msg)
	}
}

func TestChatStreamReturnsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n")
	}))
	defer srv.Close()

	body, err := NewClient(srv.URL+"/v1", "k", "m").ChatStream(context.Background(), []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("ChatStream 失败：%v", err)
	}
	defer body.Close()
	raw, _ := io.ReadAll(body)
	if !strings.Contains(string(raw), "\"content\":\"好\"") {
		t.Fatalf("响应体被篡改：%q", raw)
	}
}
