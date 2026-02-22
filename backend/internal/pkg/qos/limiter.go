package qos

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/strmsync/strmsync/internal/pkg/errutil"
)

// Config 描述限流器配置。
type Config struct {
	Rate        int
	Concurrency int
	Timeout     time.Duration
}

// Stats 记录限流等待与超时情况。
type Stats struct {
	WaitCount    uint64
	TimeoutCount uint64
	WaitNanos    uint64
}

// Limiter 提供速率与并发控制。
type Limiter struct {
	rate         *tokenBucket
	sem          *semaphore
	timeout      time.Duration
	waitNanos    uint64
	waitCount    uint64
	timeoutCount uint64
}

var ErrAcquireTimeout = errors.New("qos: acquire timeout")

// NewLimiter 创建限流器（rate/concurrency <=0 表示禁用）。
func NewLimiter(cfg Config) *Limiter {
	rate := newTokenBucket(cfg.Rate)
	sem := newSemaphore(cfg.Concurrency)
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if rate == nil && sem == nil {
		return nil
	}
	return &Limiter{
		rate:    rate,
		sem:     sem,
		timeout: timeout,
	}
}

// Acquire 获取速率与并发许可，返回释放函数。
func (l *Limiter) Acquire(ctx context.Context) (func(), error) {
	if l == nil {
		return func() {}, nil
	}
	start := time.Now()
	release := func() {}

	if l.sem != nil {
		if err := l.sem.acquire(ctx, l.timeout); err != nil {
			l.recordTimeout(start)
			return nil, errutil.Retryable(err)
		}
		release = l.sem.release
	}

	if l.rate != nil {
		if err := l.rate.acquire(ctx, l.timeout); err != nil {
			release()
			l.recordTimeout(start)
			return nil, errutil.Retryable(err)
		}
	}

	l.recordWait(start)
	return release, nil
}

// Stats 返回当前统计快照。
func (l *Limiter) Stats() Stats {
	if l == nil {
		return Stats{}
	}
	return Stats{
		WaitCount:    atomic.LoadUint64(&l.waitCount),
		TimeoutCount: atomic.LoadUint64(&l.timeoutCount),
		WaitNanos:    atomic.LoadUint64(&l.waitNanos),
	}
}

func (l *Limiter) recordWait(start time.Time) {
	if l == nil {
		return
	}
	atomic.AddUint64(&l.waitCount, 1)
	atomic.AddUint64(&l.waitNanos, uint64(time.Since(start).Nanoseconds()))
}

func (l *Limiter) recordTimeout(start time.Time) {
	if l == nil {
		return
	}
	atomic.AddUint64(&l.timeoutCount, 1)
	atomic.AddUint64(&l.waitNanos, uint64(time.Since(start).Nanoseconds()))
}

type tokenBucket struct {
	rate   int
	ch     chan struct{}
	ticker *time.Ticker
	stop   chan struct{}
}

func newTokenBucket(rate int) *tokenBucket {
	if rate <= 0 {
		return nil
	}
	interval := time.Second / time.Duration(rate)
	if interval <= 0 {
		interval = time.Millisecond
	}
	tb := &tokenBucket{
		rate:   rate,
		ch:     make(chan struct{}, rate),
		ticker: time.NewTicker(interval),
		stop:   make(chan struct{}),
	}
	for i := 0; i < rate; i++ {
		tb.ch <- struct{}{}
	}
	go tb.run()
	return tb
}

func (t *tokenBucket) run() {
	for {
		select {
		case <-t.ticker.C:
			select {
			case t.ch <- struct{}{}:
			default:
			}
		case <-t.stop:
			t.ticker.Stop()
			return
		}
	}
}

func (t *tokenBucket) acquire(ctx context.Context, timeout time.Duration) error {
	if t == nil {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return ErrAcquireTimeout
	case <-t.ch:
		return nil
	}
}

type semaphore struct {
	ch chan struct{}
}

func newSemaphore(limit int) *semaphore {
	if limit <= 0 {
		return nil
	}
	return &semaphore{ch: make(chan struct{}, limit)}
}

func (s *semaphore) acquire(ctx context.Context, timeout time.Duration) error {
	if s == nil {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return ErrAcquireTimeout
	case s.ch <- struct{}{}:
		return nil
	}
}

func (s *semaphore) release() {
	if s == nil {
		return
	}
	select {
	case <-s.ch:
	default:
	}
}

// Manager 复用相同 server 的限流器，避免重复创建。
type Manager struct {
	mu      sync.Mutex
	items   map[string]*Limiter
	configs map[string]Config
}

// NewManager 创建限流器管理器。
func NewManager() *Manager {
	return &Manager{
		items:   make(map[string]*Limiter),
		configs: make(map[string]Config),
	}
}

// Get 获取或创建限流器，配置变化则重新创建。
func (m *Manager) Get(key string, cfg Config) *Limiter {
	if cfg.Rate <= 0 && cfg.Concurrency <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.configs[key]; ok {
		if prev == cfg {
			return m.items[key]
		}
	}
	limiter := NewLimiter(cfg)
	m.items[key] = limiter
	m.configs[key] = cfg
	return limiter
}
