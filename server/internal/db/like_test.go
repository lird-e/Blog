package db

import "testing"

// TestEscapeLike 用户输入里的 LIKE 通配符必须按字面匹配：
// 搜「100%」不能命中所有标题，搜「a_b」不能命中「aXb」。
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"100%":   `100\%`,
		"a_b":    `a\_b`,
		`50\off`: `50\\off`,
		"普通词":    "普通词",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestListPostsSearchLiteralWildcards 端到端验证：含 % 的关键词只命中字面包含 % 的文章。
func TestListPostsSearchLiteralWildcards(t *testing.T) {
	d := newTestDB(t)
	mk := func(slug, title string) {
		t.Helper()
		if _, err := d.UpsertPost(&Post{
			Slug: slug, Title: title, ContentMD: "c", ContentHTML: "<p>c</p>",
			Published: true, CreatedAt: "2026-01-01T00:00:00+08:00", UpdatedAt: "2026-01-01T00:00:00+08:00",
		}); err != nil {
			t.Fatalf("插入 %s: %v", slug, err)
		}
	}
	mk("pct", "折扣 100% 返现")
	mk("plain", "折扣返现指南")

	items, total, err := d.ListPosts(ListOpts{Query: "100%", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Slug != "pct" {
		t.Fatalf("搜「100%%」应只命中 pct，实际 total=%d items=%v", total, items)
	}

	items, total, err = d.ListPosts(ListOpts{Query: "返现", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if total != 2 {
		t.Fatalf("不含通配符的普通关键词仍应命中两篇（都含「返现」），实际 total=%d", total)
	}
}
