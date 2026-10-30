package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// spaHandler 托管前端构建产物，未命中的 GET 路径回退到 index.html（hash 路由）。
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." || clean == "" {
			http.ServeFile(w, r, index)
			return
		}
		full := filepath.Join(dir, clean)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if strings.Contains(clean, ".") && !strings.HasSuffix(clean, ".html") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}
