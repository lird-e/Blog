package handlers

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello, World!": "hello-world",
		"Go 入门指南":      "go-入门指南",
		"AI Agent 深入":   "ai-agent-深入",
		"  --  ":        "post",
		"":              "post",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHashWithSalt(t *testing.T) {
	if got := hashWithSalt("s", ""); got != "" {
		t.Fatalf("空值应返回空串，got %q", got)
	}
	a := hashWithSalt("s1", "1.2.3.4")
	b := hashWithSalt("s1", "1.2.3.4")
	c := hashWithSalt("s2", "1.2.3.4")
	if a != b {
		t.Fatal("同盐同值哈希应一致")
	}
	if a == c {
		t.Fatal("不同盐哈希应不同")
	}
	if len(a) != 64 {
		t.Fatalf("SHA-256 hex 长度应为 64，got %d", len(a))
	}
}
