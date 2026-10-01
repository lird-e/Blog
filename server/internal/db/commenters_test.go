package db

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

// 令牌版本生命周期：新身份为 1 → 吊销递增 → 重新登录重置回 1。
func TestCommenterTokenVersion(t *testing.T) {
	d := newTestDB(t)

	id, err := d.UpsertCommenter("github", "42", "octocat", "http://e/a.png")
	if err != nil {
		t.Fatalf("UpsertCommenter 失败: %v", err)
	}
	cm, err := d.GetCommenter(id)
	if err != nil || cm == nil {
		t.Fatalf("GetCommenter 失败: %v", err)
	}
	if cm.TokenVersion != 1 {
		t.Fatalf("新身份令牌版本应为 1，got %d", cm.TokenVersion)
	}

	// 吊销两次 → 版本递增到 3，且必须持久化
	for i, want := range []int{2, 3} {
		if err := d.RevokeCommenterTokens(id); err != nil {
			t.Fatalf("第 %d 次吊销失败: %v", i+1, err)
		}
		got, err := d.GetCommenter(id)
		if err != nil || got == nil {
			t.Fatalf("吊销后 GetCommenter 失败: %v", err)
		}
		if got.TokenVersion != want {
			t.Fatalf("第 %d 次吊销后版本应为 %d，got %d", i+1, want, got.TokenVersion)
		}
	}

	// 重新登录（同 external_id）后版本重置为 1，新签发的令牌即可用
	if _, err := d.UpsertCommenter("github", "42", "octocat2", "http://e/b.png"); err != nil {
		t.Fatalf("重新登录失败: %v", err)
	}
	again, err := d.GetCommenter(id)
	if err != nil || again == nil {
		t.Fatalf("重新登录后 GetCommenter 失败: %v", err)
	}
	if again.TokenVersion != 1 {
		t.Errorf("重新登录后令牌版本应重置为 1，got %d", again.TokenVersion)
	}
	if again.Nickname != "octocat2" {
		t.Errorf("昵称未随重新登录更新，got %q", again.Nickname)
	}
}

// 老库升级场景：ensureColumn 用 ADD COLUMN ... NOT NULL DEFAULT 1，
// SQLite 会把已有行回填为 1。这里验证回填结果与「首次吊销从 1 起算」，
// 防止将来把默认值改掉导致老身份的版本从 0 起算、被误判为已吊销。
func TestCommenterTokenVersionLegacyBackfill(t *testing.T) {
	d := newTestDB(t)
	id, err := d.UpsertCommenter("github", "7", "legacy", "")
	if err != nil {
		t.Fatal(err)
	}
	cm, err := d.GetCommenter(id)
	if err != nil || cm == nil {
		t.Fatalf("GetCommenter 失败: %v", err)
	}
	if cm.TokenVersion != 1 {
		t.Fatalf("老身份版本应为 1，got %d", cm.TokenVersion)
	}
	// 首次吊销必须得到 2（而不是 1 或 0），否则客户端拿到的旧令牌不会被判失效
	after, err := d.BumpCommenterTokenVersion(id)
	if err != nil {
		t.Fatalf("吊销失败: %v", err)
	}
	if after != 2 {
		t.Errorf("首次吊销后版本应为 2，got %d", after)
	}
}
