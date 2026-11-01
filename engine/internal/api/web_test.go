package api

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStaticFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>index</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("console.log('qecho');", 400)
	if err := os.WriteFile(filepath.Join(assets, "app-abc123.js"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func do(t *testing.T, h http.Handler, method, target string, hdr map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestSpaHandlerGzipAsset(t *testing.T) {
	h := spaHandler(newStaticFixture(t))

	res := do(t, h, http.MethodGet, "/assets/app-abc123.js", map[string]string{"Accept-Encoding": "gzip, deflate"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if got := res.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := res.Header.Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("Cache-Control = %q, 期望 immutable", got)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	if want := strings.Repeat("console.log('qecho');", 400); string(raw) != want {
		t.Fatalf("解压后内容不一致，len=%d", len(raw))
	}
}

func TestSpaHandlerPlainWhenNoGzipAccept(t *testing.T) {
	h := spaHandler(newStaticFixture(t))
	res := do(t, h, http.MethodGet, "/assets/app-abc123.js", nil)
	if got := res.Header.Get("Content-Encoding"); got != "" {
		t.Fatalf("Content-Encoding = %q, 未声明 gzip 时不应压缩", got)
	}
	if res.ContentLength <= 0 {
		t.Fatalf("未压缩响应应带 Content-Length，实际 %d", res.ContentLength)
	}
}

func TestSpaHandlerIndexNoCacheAndRevalidate(t *testing.T) {
	h := spaHandler(newStaticFixture(t))
	res := do(t, h, http.MethodGet, "/", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	if got := res.Header.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("index Cache-Control = %q, want no-cache", got)
	}
	etag := res.Header.Get("ETag")
	if etag == "" {
		t.Fatal("缺少 ETag")
	}
	again := do(t, h, http.MethodGet, "/", map[string]string{"If-None-Match": etag})
	if again.StatusCode != http.StatusNotModified {
		t.Fatalf("重新校验 status = %d, want 304", again.StatusCode)
	}
}

func TestSpaHandlerFallbackAndMissingAsset(t *testing.T) {
	h := spaHandler(newStaticFixture(t))

	if res := do(t, h, http.MethodGet, "/practice", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("SPA 回退 status = %d, want 200", res.StatusCode)
	}
	if res := do(t, h, http.MethodGet, "/assets/missing.js", nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("缺失资源 status = %d, want 404", res.StatusCode)
	}
	if res := do(t, h, http.MethodPost, "/", nil); res.StatusCode != http.StatusNotFound {
		t.Fatalf("POST status = %d, want 404", res.StatusCode)
	}
}

func TestSpaHandlerHeadGzipNoBody(t *testing.T) {
	h := spaHandler(newStaticFixture(t))
	res := do(t, h, http.MethodHead, "/assets/app-abc123.js", map[string]string{"Accept-Encoding": "gzip"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if len(body) != 0 {
		t.Fatalf("HEAD 不应返回响应体，实际 %d 字节", len(body))
	}
}

func TestSpaHandlerRangeStillSupported(t *testing.T) {
	h := spaHandler(newStaticFixture(t))

	res := do(t, h, http.MethodGet, "/assets/app-abc123.js", map[string]string{"Range": "bytes=0-9"})
	if res.StatusCode != http.StatusPartialContent {
		t.Fatalf("Range status = %d, want 206", res.StatusCode)
	}
	if got := res.Header.Get("Content-Range"); !strings.HasPrefix(got, "bytes 0-9/") {
		t.Fatalf("Content-Range = %q", got)
	}
	body, _ := io.ReadAll(res.Body)
	if len(body) != 10 {
		t.Fatalf("分片长度 = %d, want 10", len(body))
	}

	// 压缩响应无法按字节续传，应显式声明
	gz := do(t, h, http.MethodGet, "/assets/app-abc123.js", map[string]string{"Accept-Encoding": "gzip"})
	if got := gz.Header.Get("Accept-Ranges"); got != "none" {
		t.Fatalf("gzip 响应 Accept-Ranges = %q, want none", got)
	}
}
