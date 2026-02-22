package logger

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type debugPolicy struct {
	Enabled bool
	Modules map[string]struct{}
	RPS     int
}

var (
	debugPolicyValue atomic.Value
	debugLimiterInst = newDebugLimiter()
)

func init() {
	debugPolicyValue.Store(debugPolicy{
		Enabled: false,
		Modules: map[string]struct{}{},
		RPS:     0,
	})
}

// ApplyDebugPolicy 设置 debug 输出策略
// enabled: 是否启用debug输出
// modules: 模块白名单（空则禁用debug）
// rps: 每秒最大debug条数（<=0 表示不限流）
func ApplyDebugPolicy(enabled bool, modules []string, rps int) {
	policy := debugPolicy{
		Enabled: enabled,
		Modules: map[string]struct{}{},
		RPS:     rps,
	}
	for _, module := range modules {
		value := normalizeModule(module)
		if value == "" {
			continue
		}
		policy.Modules[value] = struct{}{}
	}
	debugPolicyValue.Store(policy)
}

func shouldAllowDebug(module string) bool {
	policy := getDebugPolicy()
	if !policy.Enabled {
		return false
	}
	module = normalizeModule(module)
	if module == "" {
		return false
	}
	if len(policy.Modules) == 0 {
		return false
	}
	if _, ok := policy.Modules[module]; !ok {
		return false
	}
	if policy.RPS <= 0 {
		return true
	}
	return debugLimiterInst.Allow(module, policy.RPS)
}

func getDebugPolicy() debugPolicy {
	if value := debugPolicyValue.Load(); value != nil {
		if policy, ok := value.(debugPolicy); ok {
			return policy
		}
	}
	return debugPolicy{Enabled: false, Modules: map[string]struct{}{}, RPS: 0}
}

func normalizeModule(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

type debugLimiter struct {
	mu      sync.Mutex
	buckets map[string]*debugBucket
}

type debugBucket struct {
	second int64
	count  int
}

func newDebugLimiter() *debugLimiter {
	return &debugLimiter{
		buckets: map[string]*debugBucket{},
	}
}

func (l *debugLimiter) Allow(module string, rps int) bool {
	if rps <= 0 {
		return true
	}
	now := time.Now().Unix()
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[module]
	if !ok {
		l.buckets[module] = &debugBucket{second: now, count: 1}
		return true
	}
	if bucket.second != now {
		bucket.second = now
		bucket.count = 1
		return true
	}
	if bucket.count >= rps {
		return false
	}
	bucket.count++
	return true
}
