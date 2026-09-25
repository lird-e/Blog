package db

import (
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"网络, 随笔,,AI", "网络,随笔,AI"},
		{[]any{"网络", " 随笔 ", 42}, "网络,随笔,42"},
		{"  ", ""},
	}
	for _, c := range cases {
		if got := NormalizeTags(c.in); got != c.want {
			t.Errorf("NormalizeTags(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSplitTags(t *testing.T) {
	got := SplitTags("")
	if got == nil || len(got) != 0 {
		t.Fatalf("空串应返回非 nil 空切片，got %#v", got)
	}
	if got := SplitTags("网络, 随笔"); !reflect.DeepEqual(got, []string{"网络", "随笔"}) {
		t.Fatalf("got %#v", got)
	}
}
