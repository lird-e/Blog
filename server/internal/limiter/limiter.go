// Package limiter 提供进程内滑动窗口限流。
// 覆盖三类场景：评论频率（同 IP 每分钟 1 条 / 每日 20 条）、
// 登录失败锁定（5 次锁 15 分钟）。单实例部署，内存计数即可；
// Nginx limit_req 是外层第一道保险，两者互补。
package limiter

import (
	"sync"
	"time"
)

type Limiter struct {
	mu        sync.Mutex
	hits      map[string][]time.Time
	lastSweep time.Time
}

func New() *Limiter {
	return &Limiter{hits: make(map[string][]time.Time)}
}

// Allow 记录一次命中并判断是否放行：window 窗口内最多 limit 次。
func (l *Limiter) Allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.sweepLocked(now)
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}

// Fail 记录一次失败。
func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.hits[key] = append(l.hits[key], time.Now())
}

// Blocked 判断 key 在 window 内的失败次数是否达到 threshold（达到即锁定）。
func (l *Limiter) Blocked(key string, threshold int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	l.sweepLocked(now)
	n := 0
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			n++
		}
	}
	return n >= threshold
}

// sweepLocked 机会式清扫：距上次清扫超过 10 分钟时，清掉所有条目已过期
// （超过项目内最长窗口 24h）的 key，防止不再出现的 IP 永久驻留内存。
// 调用方必须已持有 mu。
func (l *Limiter) sweepLocked(now time.Time) {
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < 10*time.Minute {
		return
	}
	l.lastSweep = now
	for key, ts := range l.hits {
		kept := ts[:0]
		for _, t := range ts {
			if now.Sub(t) < 24*time.Hour {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.hits, key)
		} else {
			l.hits[key] = kept
		}
	}
}

// Reset 清除 key 的失败记录（登录成功后调用）。
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}
