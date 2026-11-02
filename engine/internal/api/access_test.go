package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"quizbox/engine/internal/store"
)

// 本文件锁住题库权限矩阵：6 种身份 × 3 种操作要求（读 / 改内容 / 管理）。
//
// 这套判定直接决定谁能看到、谁能改、谁能删题库，但一旦写错不会 panic、
// 不会报错，只会「悄悄放行」或「悄悄拒绝」。以前只在改完时用临时脚本手工验，
// 脚本删掉后就没有任何回归保护了。改动 access.go / sharing.go 请先跑这里。

func int64Ptr(v int64) *int64 { return &v }

// TestApplyBankAccessLevels 校验身份判定与两个能力布尔值。
func TestApplyBankAccessLevels(t *testing.T) {
	const (
		ownerID int64 = 10
		adminID int64 = 1
		otherID int64 = 20
	)
	cases := []struct {
		name       string
		user       *store.User
		ownerID    *int64
		member     string
		wantLevel  string
		wantEdit   bool
		wantManage bool
	}{
		{"所有者", &store.User{ID: ownerID, Role: "user"}, int64Ptr(ownerID), "", levelOwner, true, true},
		{"管理员看他人题库", &store.User{ID: adminID, Role: "admin"}, int64Ptr(ownerID), "", levelAdmin, true, true},
		// 管理员看自己建的题库必须判成 owner，否则列表「我的」分组会整组漏掉。
		{"管理员看自己的题库", &store.User{ID: adminID, Role: "admin"}, int64Ptr(adminID), "", levelOwner, true, true},
		{"共享可编辑", &store.User{ID: otherID, Role: "user"}, int64Ptr(ownerID), store.LevelEditor, levelEditor, true, false},
		{"共享只读", &store.User{ID: otherID, Role: "user"}, int64Ptr(ownerID), store.LevelViewer, levelViewer, false, false},
		{"陌生人", &store.User{ID: otherID, Role: "user"}, int64Ptr(ownerID), "", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bank := &store.Bank{OwnerID: c.ownerID}
			applyBankAccess(c.user, bank, c.member)
			if bank.MemberLevel != c.wantLevel {
				t.Errorf("身份判定 = %q，期望 %q", bank.MemberLevel, c.wantLevel)
			}
			if bank.CanEdit != c.wantEdit {
				t.Errorf("canEdit = %v，期望 %v", bank.CanEdit, c.wantEdit)
			}
			if bank.CanManage != c.wantManage {
				t.Errorf("canManage = %v，期望 %v", bank.CanManage, c.wantManage)
			}
		})
	}
}

// TestCanReadBank 校验「能不能看见」：公共题库与对全站开放的题库任何人可读。
func TestCanReadBank(t *testing.T) {
	cases := []struct {
		name string
		bank *store.Bank
		want bool
	}{
		{"公共题库（无所有者）", &store.Bank{}, true},
		{"个人库未共享", &store.Bank{OwnerID: int64Ptr(10)}, false},
		{"个人库共享只读", &store.Bank{OwnerID: int64Ptr(10), MemberLevel: levelViewer}, true},
		{"个人库共享可编辑", &store.Bank{OwnerID: int64Ptr(10), MemberLevel: levelEditor}, true},
		{"个人库对全站开放", &store.Bank{OwnerID: int64Ptr(10), Visibility: store.VisibilityPublic}, true},
		{"公共题库带共享标记", &store.Bank{MemberLevel: levelViewer}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := canReadBank(c.bank); got != c.want {
				t.Errorf("canReadBank = %v，期望 %v", got, c.want)
			}
		})
	}
}

// accessFixture 是一套真实数据库：五种身份的账号 + 四本不同可见性的题库。
type accessFixture struct {
	srv *Server

	admin    *store.User // 管理员，另有一本自己的库
	owner    *store.User // 私有库所有者
	editor   *store.User // 被共享为可编辑
	viewer   *store.User // 被共享为只读
	stranger *store.User // 与本库无关的普通用户

	adminBank  *store.Bank // admin 自己的私有库
	ownBank    *store.Bank // owner 的私有库（共享给 editor / viewer）
	openBank   *store.Bank // owner 的库，对全站开放只读
	commonBank *store.Bank // 公共题库（owner_id 为空）
}

func newAccessFixture(t *testing.T) *accessFixture {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开测试库失败：%v", err)
	}
	t.Cleanup(func() { st.Close() })

	mustUser := func(name, role string) *store.User {
		t.Helper()
		u, err := st.CreateUser(name, "pw", role)
		if err != nil {
			t.Fatalf("创建用户 %s 失败：%v", name, err)
		}
		return u
	}
	f := &accessFixture{
		srv:      &Server{store: st},
		admin:    mustUser("admin-a", "admin"),
		owner:    mustUser("owner-a", "user"),
		editor:   mustUser("editor-a", "user"),
		viewer:   mustUser("viewer-a", "user"),
		stranger: mustUser("stranger-a", "user"),
	}

	mustBank := func(name string, ownerID *int64) *store.Bank {
		t.Helper()
		b, err := st.CreateBank(name, "", ownerID)
		if err != nil {
			t.Fatalf("创建题库 %s 失败：%v", name, err)
		}
		return b
	}
	f.adminBank = mustBank("管理员私有库", &f.admin.ID)
	f.ownBank = mustBank("私有库", &f.owner.ID)
	f.openBank = mustBank("公开只读库", &f.owner.ID)
	f.commonBank = mustBank("公共库", nil)

	if err := st.SetBankVisibility(f.openBank.ID, store.VisibilityPublic); err != nil {
		t.Fatalf("设置公开可见失败：%v", err)
	}
	if err := st.UpsertBankMember(f.ownBank.ID, f.editor.ID, store.LevelEditor); err != nil {
		t.Fatalf("共享给可编辑失败：%v", err)
	}
	if err := st.UpsertBankMember(f.ownBank.ID, f.viewer.ID, store.LevelViewer); err != nil {
		t.Fatalf("共享给只读失败：%v", err)
	}
	return f
}

// probe 走真实的 bankFor 判定，返回 HTTP 状态码（200 表示放行）。
func (f *accessFixture) probe(t *testing.T, u *store.User, bankID int64, need bankAccess) (int, *store.Bank) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), userKey, u))
	rec := httptest.NewRecorder()
	bank, ok := f.srv.bankFor(rec, req, bankID, need)
	if !ok {
		return rec.Code, nil
	}
	return http.StatusOK, bank
}

// TestBankForPermissionMatrix 是那张 18 格表的端到端版本。
func TestBankForPermissionMatrix(t *testing.T) {
	f := newAccessFixture(t)
	cases := []struct {
		name   string
		user   *store.User
		bankID int64
		need   bankAccess
		want   int
	}{
		// 所有者：三级全通
		{"所有者·读", f.owner, f.ownBank.ID, accessRead, http.StatusOK},
		{"所有者·改内容", f.owner, f.ownBank.ID, accessEdit, http.StatusOK},
		{"所有者·管理", f.owner, f.ownBank.ID, accessManage, http.StatusOK},

		// 管理员：对他人题库三级全通
		{"管理员·读他人库", f.admin, f.ownBank.ID, accessRead, http.StatusOK},
		{"管理员·改他人库", f.admin, f.ownBank.ID, accessEdit, http.StatusOK},
		{"管理员·管理他人库", f.admin, f.ownBank.ID, accessManage, http.StatusOK},

		// 共享可编辑：能读能改，不能管理
		{"可编辑·读", f.editor, f.ownBank.ID, accessRead, http.StatusOK},
		{"可编辑·改内容", f.editor, f.ownBank.ID, accessEdit, http.StatusOK},
		{"可编辑·管理", f.editor, f.ownBank.ID, accessManage, http.StatusForbidden},

		// 共享只读：只能读
		{"只读·读", f.viewer, f.ownBank.ID, accessRead, http.StatusOK},
		{"只读·改内容", f.viewer, f.ownBank.ID, accessEdit, http.StatusForbidden},
		{"只读·管理", f.viewer, f.ownBank.ID, accessManage, http.StatusForbidden},

		// 陌生人看私有库：一律拒绝
		{"陌生人·读私有库", f.stranger, f.ownBank.ID, accessRead, http.StatusForbidden},
		{"陌生人·改私有库", f.stranger, f.ownBank.ID, accessEdit, http.StatusForbidden},
		{"陌生人·管理私有库", f.stranger, f.ownBank.ID, accessManage, http.StatusForbidden},

		// 对全站开放：能读，不能改
		{"陌生人·读公开库", f.stranger, f.openBank.ID, accessRead, http.StatusOK},
		{"陌生人·改公开库", f.stranger, f.openBank.ID, accessEdit, http.StatusForbidden},
		{"陌生人·管理公开库", f.stranger, f.openBank.ID, accessManage, http.StatusForbidden},

		// 公共题库：人人可读，只有管理员能改
		{"普通用户·读公共库", f.stranger, f.commonBank.ID, accessRead, http.StatusOK},
		{"普通用户·改公共库", f.stranger, f.commonBank.ID, accessEdit, http.StatusForbidden},
		{"普通用户·管理公共库", f.stranger, f.commonBank.ID, accessManage, http.StatusForbidden},
		{"管理员·管理公共库", f.admin, f.commonBank.ID, accessManage, http.StatusOK},

		// 题库不存在给 404，不是 403
		{"不存在的题库", f.owner, 999999, accessRead, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, bank := f.probe(t, c.user, c.bankID, c.need)
			if code != c.want {
				t.Fatalf("状态码 = %d，期望 %d", code, c.want)
			}
			if code == http.StatusOK && bank == nil {
				t.Fatal("放行时必须返回题库对象")
			}
		})
	}
}

// TestBankForFillsPermissionFields 校验返回给前端的 memberLevel / canEdit / canManage，
// 前端的按钮显隐完全依赖这三个字段。
func TestBankForFillsPermissionFields(t *testing.T) {
	f := newAccessFixture(t)
	cases := []struct {
		name       string
		user       *store.User
		bankID     int64
		wantLevel  string
		wantEdit   bool
		wantManage bool
	}{
		{"所有者", f.owner, f.ownBank.ID, levelOwner, true, true},
		{"管理员看他人库", f.admin, f.ownBank.ID, levelAdmin, true, true},
		// 上一轮的回归点：管理员看自己的库必须还是 owner。
		{"管理员看自己的库", f.admin, f.adminBank.ID, levelOwner, true, true},
		{"共享可编辑", f.editor, f.ownBank.ID, levelEditor, true, false},
		{"共享只读", f.viewer, f.ownBank.ID, levelViewer, false, false},
		{"对全站开放的陌生人", f.stranger, f.openBank.ID, "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, bank := f.probe(t, c.user, c.bankID, accessRead)
			if code != http.StatusOK {
				t.Fatalf("读权限被拒，状态码 %d", code)
			}
			if bank.MemberLevel != c.wantLevel {
				t.Errorf("memberLevel = %q，期望 %q", bank.MemberLevel, c.wantLevel)
			}
			if bank.CanEdit != c.wantEdit {
				t.Errorf("canEdit = %v，期望 %v", bank.CanEdit, c.wantEdit)
			}
			if bank.CanManage != c.wantManage {
				t.Errorf("canManage = %v，期望 %v", bank.CanManage, c.wantManage)
			}
		})
	}
}

// TestDecorateBanks 校验列表批量填充（不传 id，靠 BankMemberLevels 批量查）。
func TestDecorateBanks(t *testing.T) {
	f := newAccessFixture(t)
	banks := []store.Bank{*f.ownBank, *f.openBank, *f.commonBank}
	if err := f.srv.decorateBanks(f.viewer, banks); err != nil {
		t.Fatalf("批量填充失败：%v", err)
	}
	if banks[0].MemberLevel != levelViewer {
		t.Errorf("共享只读库的 memberLevel = %q，期望 %q", banks[0].MemberLevel, levelViewer)
	}
	if banks[0].CanEdit || banks[0].CanManage {
		t.Error("共享只读库不应有编辑或管理权限")
	}
	if banks[1].MemberLevel != "" {
		t.Errorf("未共享的对全站开放库 memberLevel = %q，期望空", banks[1].MemberLevel)
	}
	if banks[2].CanManage {
		t.Error("公共题库对普通用户不应有管理权限")
	}
}

// TestRequireExportImport 校验「导出/导入题库」能力位：管理员恒通过。
func TestRequireExportImport(t *testing.T) {
	f := newAccessFixture(t)
	authorized := *f.editor
	authorized.CanExportImport = true

	cases := []struct {
		name string
		user *store.User
		want bool
	}{
		{"管理员即使未勾选也通过", f.admin, true},
		{"普通用户未授权", f.owner, false},
		{"普通用户已授权", &authorized, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), userKey, c.user))
			rec := httptest.NewRecorder()
			if got := requireExportImport(rec, req); got != c.want {
				t.Fatalf("requireExportImport = %v，期望 %v（状态码 %d）", got, c.want, rec.Code)
			}
			if !c.want && rec.Code != http.StatusForbidden {
				t.Errorf("拒绝时应返回 403，实际 %d", rec.Code)
			}
		})
	}
}

// TestCanManageExam 校验考试归属：创建者或管理员。
func TestCanManageExam(t *testing.T) {
	cases := []struct {
		name    string
		user    *store.User
		ownerID int64
		want    bool
	}{
		{"管理员", &store.User{ID: 1, Role: "admin"}, 99, true},
		{"创建者", &store.User{ID: 5, Role: "user"}, 5, true},
		{"其他人", &store.User{ID: 5, Role: "user"}, 6, false},
		{"归属未知", &store.User{ID: 5, Role: "user"}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := canManageExam(c.user, c.ownerID); got != c.want {
				t.Errorf("canManageExam = %v，期望 %v", got, c.want)
			}
		})
	}
}

// TestSearchUsersKeywordRules 校验共享时挑人的搜索规则：
// 少于两个字符不返回结果（防止一次列出大半账号），一次最多 10 条。
func TestSearchUsersKeywordRules(t *testing.T) {
	f := newAccessFixture(t)
	for i := 1; i <= 12; i++ {
		name := fmt.Sprintf("user%02d", i)
		if _, err := f.srv.store.CreateUser(name, "pw", "user"); err != nil {
			t.Fatalf("创建用户 %s 失败：%v", name, err)
		}
	}

	search := func(keyword string) []string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/search?keyword="+url.QueryEscape(keyword), nil)
		rec := httptest.NewRecorder()
		f.srv.searchUsers(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("搜索 %q 返回状态码 %d", keyword, rec.Code)
		}
		var body struct {
			Data struct {
				Items []struct {
					Username string `json:"username"`
				} `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("解析响应失败：%v", err)
		}
		out := make([]string, 0, len(body.Data.Items))
		for _, item := range body.Data.Items {
			out = append(out, item.Username)
		}
		return out
	}

	if got := search("user"); len(got) != userSearchLimit {
		t.Errorf("一次返回 %d 条，期望上限 %d 条", len(got), userSearchLimit)
	}
	if got := search("user01"); len(got) != 1 || got[0] != "user01" {
		t.Errorf("精确关键字应命中 1 条，实际 %v", got)
	}
	if got := search("u"); len(got) != 0 {
		t.Errorf("一个字符应返回空，实际 %d 条", len(got))
	}
	if got := search(""); len(got) != 0 {
		t.Errorf("空关键字应返回空，实际 %d 条", len(got))
	}
}
