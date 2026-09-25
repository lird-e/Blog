// cmd/genpass 生成管理员密码的 bcrypt 哈希，用于 .env 的 ADMIN_PASS_HASH。
// 用法: go run ./cmd/genpass 你的强密码
package main

import (
	"fmt"
	"os"

	"github.com/lird-e/Blog/server/internal/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: go run ./cmd/genpass <密码>")
		os.Exit(1)
	}
	hash, err := auth.HashPassword(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "生成失败:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}
