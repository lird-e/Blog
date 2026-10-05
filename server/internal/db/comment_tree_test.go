package db

import "testing"

// TestDeleteCommentTree 删除父评论必须连整栋回复一起删掉，
// 否则留下的孤儿回复（parent_id 指向已删行）会被前端渲染成没有上下文的根评论。
func TestDeleteCommentTree(t *testing.T) {
	d := newTestDB(t)
	postID, err := d.UpsertPost(&Post{
		Slug: "p", Title: "T", ContentMD: "c", ContentHTML: "<p>c</p>",
		Published: true, CreatedAt: "2026-01-01T00:00:00+08:00", UpdatedAt: "2026-01-01T00:00:00+08:00",
	})
	if err != nil {
		t.Fatalf("插入文章: %v", err)
	}
	add := func(parent int64) int64 {
		t.Helper()
		id, err := d.CreateComment(&Comment{
			PostID: postID, ParentID: parent, Nickname: "n", Content: "c", Status: "approved",
		})
		if err != nil {
			t.Fatalf("插入评论: %v", err)
		}
		return id
	}
	root := add(0)
	child := add(root)
	grand := add(child)
	sibling := add(0)

	// 游客评论落库时 parent_id / commenter_id 都是 NULL（见 CreateComment 的 nullableID），
	// 早期按 int64 扫描这两列会让整条列表查询报错，两个读取入口都要覆盖。
	if list, _, err := d.AdminListComments("", 20, 0); err != nil || len(list) != 4 {
		t.Fatalf("AdminListComments: %d 条 err=%v，期望 4 条", len(list), err)
	}
	if list, err := d.ListCommentsByPost(postID); err != nil || len(list) != 4 {
		t.Fatalf("ListCommentsByPost: %d 条 err=%v，期望 4 条", len(list), err)
	}

	mustExist := func(id int64, want bool) {
		t.Helper()
		c, err := d.GetCommentForPost(id, postID)
		if err != nil {
			t.Fatalf("查询评论 %d: %v", id, err)
		}
		if (c != nil) != want {
			t.Fatalf("评论 %d 存在=%v，期望 %v", id, c != nil, want)
		}
	}

	n, err := d.DeleteCommentTree(child)
	if err != nil || n != 2 {
		t.Fatalf("删除 child 子树: n=%d err=%v，期望删掉 2 条", n, err)
	}
	mustExist(child, false)
	mustExist(grand, false)
	mustExist(root, true)
	mustExist(sibling, true)

	if n, err := d.DeleteCommentTree(root); err != nil || n != 1 {
		t.Fatalf("删除 root: n=%d err=%v，期望 1", n, err)
	}
	// 删不存在的 id 不应报错，也不应影响其他行
	if n, err := d.DeleteCommentTree(9999); err != nil || n != 0 {
		t.Fatalf("删除不存在评论: n=%d err=%v，期望 0", n, err)
	}
	mustExist(sibling, true)
}
