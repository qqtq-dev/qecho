package api

import (
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// 可压缩的静态资源后缀：文本类才值得 gzip，图片/字体本身已压缩
var compressibleExt = map[string]bool{
	".js":    true,
	".mjs":   true,
	".css":   true,
	".html":  true,
	".json":  true,
	".svg":   true,
	".map":   true,
	".txt":   true,
	".xml":   true,
	".ico":   false,
	".png":   false,
	".jpg":   false,
	".jpeg":  false,
	".webp":  false,
	".gif":   false,
	".woff":  false,
	".woff2": false,
	".ttf":   false,
}

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return w
	},
}

// spaHandler 托管前端构建产物，未命中的 GET 路径回退到 index.html（hash 路由）。
// 文本资源按 Accept-Encoding 现场 gzip；assets 下是带哈希的产物，可长期缓存。
func spaHandler(dir string) http.Handler {
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." || clean == "" {
			serveStatic(w, r, index, false)
			return
		}
		full := filepath.Join(dir, clean)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			// dist/assets 下的文件名带内容哈希，可以放心长缓存
			immutable := strings.HasPrefix(filepath.ToSlash(clean), "assets/")
			serveStatic(w, r, full, immutable)
			return
		}
		if strings.Contains(clean, ".") && !strings.HasSuffix(clean, ".html") {
			http.NotFound(w, r)
			return
		}
		serveStatic(w, r, index, false)
	})
}

func serveStatic(w http.ResponseWriter, r *http.Request, path string, immutable bool) {
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}

	etag := `W/"` + strconv.FormatInt(info.Size(), 16) + "-" + strconv.FormatInt(info.ModTime().UnixNano(), 16) + `"`
	h := w.Header()
	h.Set("ETag", etag)
	h.Set("Vary", "Accept-Encoding")
	if immutable {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "no-cache")
	}

	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	h.Set("Content-Type", ctype)

	useGzip := compressibleExt[strings.ToLower(filepath.Ext(path))] &&
		strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")

	if !useGzip {
		h.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		io.Copy(w, f)
		return
	}

	h.Set("Content-Encoding", "gzip")
	h.Del("Content-Length")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}

	gz := gzipPool.Get().(*gzip.Writer)
	gz.Reset(w)
	io.Copy(gz, f)
	gz.Close()
	gzipPool.Put(gz)
}
