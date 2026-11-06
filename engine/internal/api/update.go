package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"quizbox/engine/internal/response"
)

// 版本清单只从 GitHub 主仓库读取。
// 想换地址（例如改放自有服务器）时设置环境变量 QUIZBOX_UPDATE_URL 即可，不用重新编译。
const (
	githubUpdateManifestURL = "https://raw.githubusercontent.com/qqtq-dev/qecho/main/release/version.json"
	defaultReleasePageURL   = "https://github.com/qqtq-dev/qecho/releases"
	updateSuccessTTL        = 30 * time.Minute
	updateFailureTTL        = 5 * time.Minute
	updateTimeout           = 8 * time.Second
	updateMaxBytes          = 64 << 10
)

// defaultUpdateManifestURLs 版本清单地址。
var defaultUpdateManifestURLs = []string{githubUpdateManifestURL}

// updateManifest 对应公开仓库里的 version.json。
type updateManifest struct {
	Version     string `json:"version"`
	Notes       string `json:"notes"`
	PublishedAt string `json:"publishedAt"`
	URL         string `json:"url"`
	DownloadURL string `json:"downloadUrl"`
	Force       bool   `json:"force"`
}

type updateSnapshot struct {
	at       time.Time
	manifest updateManifest
	err      error
}

// updateManifestURLs 返回实际使用的清单地址列表；设置 QUIZBOX_UPDATE_URL 时只走该地址。
func updateManifestURLs() []string {
	if v := strings.TrimSpace(os.Getenv("QUIZBOX_UPDATE_URL")); v != "" {
		return []string{v}
	}
	return defaultUpdateManifestURLs
}

// checkUpdate 返回当前版本与远端最新版本，供顶栏铃铛展示。
// 外网请求放在引擎里做：桌面端没有跨域问题，也能统一缓存，避免每次开窗口都打 Gitee。
func (s *Server) checkUpdate(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force")
	manifest, err := s.fetchUpdateManifest(force == "1" || force == "true")

	checkedAt := time.Now().Format(time.RFC3339)
	if err != nil {
		// 拉不到版本信息不算故障：正常用，只是不提示更新。
		response.OK(w, map[string]any{
			"ok":        false,
			"current":   s.version,
			"hasUpdate": false,
			"message":   "暂时无法获取版本信息",
			"checkedAt": checkedAt,
		})
		return
	}

	current := strings.TrimSpace(s.version)
	latest := strings.TrimSpace(manifest.Version)
	has := false
	if cur, okCur := parseVersion(current); okCur {
		if lat, okLat := parseVersion(latest); okLat {
			has = compareVersion(lat, cur) > 0
		}
	}

	page := strings.TrimSpace(manifest.URL)
	if page == "" {
		page = defaultReleasePageURL
	}
	download := strings.TrimSpace(manifest.DownloadURL)
	if download == "" {
		download = page
	}

	response.OK(w, map[string]any{
		"ok":          true,
		"current":     current,
		"latest":      latest,
		"hasUpdate":   has,
		"force":       manifest.Force,
		"notes":       manifest.Notes,
		"publishedAt": manifest.PublishedAt,
		"url":         page,
		"downloadUrl": download,
		"checkedAt":   checkedAt,
	})
}

// fetchUpdateManifest 命中缓存就直接返回；失败也缓存一小段时间，避免断网时反复超时。
func (s *Server) fetchUpdateManifest(force bool) (updateManifest, error) {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()

	if !force && s.updateSnap != nil {
		ttl := updateSuccessTTL
		if s.updateSnap.err != nil {
			ttl = updateFailureTTL
		}
		if time.Since(s.updateSnap.at) < ttl {
			return s.updateSnap.manifest, s.updateSnap.err
		}
	}

	// 逐个源尝试，第一个成功即返回；全部失败才把错误往上抛。
	// 正常情况下只有一个源（GitHub），列表形式仅为 QUIZBOX_UPDATE_URL 之外的扩展留口子。
	var manifest updateManifest
	var err error
	for _, endpoint := range updateManifestURLs() {
		manifest, err = requestUpdateManifest(endpoint)
		if err == nil {
			break
		}
	}

	if err != nil {
		s.updateSnap = &updateSnapshot{at: time.Now(), err: err}
		return updateManifest{}, err
	}
	s.updateSnap = &updateSnapshot{at: time.Now(), manifest: manifest}
	return manifest, nil
}

func requestUpdateManifest(endpoint string) (updateManifest, error) {
	var manifest updateManifest

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return manifest, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Qecho-Updater/1.0 (+https://github.com/qqtq-dev/qecho)")

	client := &http.Client{
		Timeout: updateTimeout,
		// raw 地址可能跳一次，跟随即可。
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return manifest, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return manifest, &updateHTTPError{status: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, updateMaxBytes))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

type updateHTTPError struct{ status int }

func (e *updateHTTPError) Error() string {
	return "版本文件请求失败：HTTP " + strconv.Itoa(e.status)
}

// parseVersion 把 "v0.3.0"、"0.3.0-beta.1" 归一成数字段；"dev" 这类无法比较的返回 false。
func parseVersion(raw string) ([]int, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return nil, false
	}
	parts := strings.Split(s, ".")
	nums := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || n < 0 {
			return nil, false
		}
		nums = append(nums, n)
	}
	return nums, true
}

// compareVersion 逐位比较，缺位按 0 处理：a>b 返回 1，a<b 返回 -1。
func compareVersion(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}
