package imaweb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// KnowledgeBase 是个人知识库的列表项，字段精简到界面需要的部分。
type KnowledgeBase struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	DocCount string `json:"docCount"`
	Updated  string `json:"updated"`
}

// 知识库首页接口的固定参数：四个 type 分别对应「我的 / 收藏 / 共享 / 订阅」等分组，
// 与网页端 knowledge_tab_reader.get_home_page_data 的请求体一致。
var knowledgeGroups = []int{1001, 1002, 1004, 1005}

// ListKnowledgeBases 拉取当前账号可见的个人知识库。
//
// 接口是网页端「知识库」标签页的首屏数据接口，一次返回全部分组，
// 每个分组的 knowledge_base_list 即该组下的库。同一个库可能在不同分组重复
// 出现（例如既在「我的」又在「最近」），按 id 去重并保留首次出现的顺序。
func (c *Client) ListKnowledgeBases(ctx context.Context) ([]KnowledgeBase, error) {
	if err := c.ready(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	params := make([]map[string]any, 0, len(knowledgeGroups))
	for _, t := range knowledgeGroups {
		params = append(params, map[string]any{"type": t, "cursor": "", "limit": 50})
	}
	body := map[string]any{
		"need_folder_number": true,
		"knowledge_list_req": map[string]any{
			"need_default_cover": false,
			"sort_type":          9,
			"limit":              50,
			"cursor":             "",
		},
		"knowledge_base_list_req": map[string]any{"params": params},
	}
	req, err := c.newRequest(ctx, "/cgi-bin/knowledge_tab_reader/get_home_page_data", "application/json", body)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, friendlyNetErr(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取知识库列表失败：%w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || retCode(resp) == codeLoginFailed {
		return nil, ErrLoginRequired
	}

	var out struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Message string `json:"message"`
		Results []struct {
			KnowledgeBaseList []kbItem `json:"knowledge_base_list"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("知识库列表返回了无法解析的内容：%s", truncate(string(raw), 200))
	}
	if out.Code == codeLoginFailed {
		return nil, ErrLoginRequired
	}
	if out.Code != codeSuccess {
		msg := strings.TrimSpace(out.Msg)
		if msg == "" {
			msg = strings.TrimSpace(out.Message)
		}
		if msg == "" {
			msg = fmt.Sprintf("ima 返回失败（code %d）", out.Code)
		}
		return nil, fmt.Errorf("%s", msg)
	}

	seen := make(map[string]bool, 32)
	list := make([]KnowledgeBase, 0, 32)
	for _, g := range out.Results {
		for _, it := range g.KnowledgeBaseList {
			id := strings.TrimSpace(it.ID)
			name := strings.TrimSpace(it.BasicInfo.Name)
			if id == "" || name == "" || seen[id] {
				continue
			}
			seen[id] = true
			list = append(list, KnowledgeBase{
				ID:       id,
				Name:     name,
				DocCount: strings.TrimSpace(it.BasicInfo.KnowledgeTotalSize),
				Updated:  strings.TrimSpace(it.KnowledgeBaseStatusToast),
			})
		}
	}
	// 界面上按名称排一遍，同名的保持接口返回顺序（用 id 兜底稳定排序）。
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Name == list[j].Name {
			return list[i].ID < list[j].ID
		}
		return list[i].Name < list[j].Name
	})
	return list, nil
}

type kbItem struct {
	ID        string `json:"id"`
	BasicInfo struct {
		Name               string `json:"name"`
		KnowledgeTotalSize string `json:"knowledge_total_size"`
	} `json:"basic_info"`
	KnowledgeBaseStatusToast string `json:"knowledge_base_status_toast"`
}
