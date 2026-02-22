package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger 适配 gorm/logger.Interface
type GormLogger struct {
	log                  *zap.Logger
	level                gormlogger.LogLevel
	slowThreshold        time.Duration
	ignoreRecordNotFound bool
}

// NewGormLogger 创建 Gorm Logger 适配器
func NewGormLogger(base *zap.Logger) *GormLogger {
	if base == nil {
		base = L()
	}
	return &GormLogger{
		log:                  base,
		level:                gormlogger.Warn,
		slowThreshold:        200 * time.Millisecond,
		ignoreRecordNotFound: true,
	}
}

// WithSlowThreshold 设置慢查询阈值
func (l *GormLogger) WithSlowThreshold(threshold time.Duration) *GormLogger {
	if l == nil {
		return l
	}
	cp := *l
	cp.slowThreshold = threshold
	return &cp
}

// LogMode 设置日志级别
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	if l == nil {
		return l
	}
	cp := *l
	cp.level = level
	return &cp
}

// Info 输出 Info 日志
func (l *GormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l == nil || l.level < gormlogger.Info {
		return
	}
	WithContext(ctx).Info(fmt.Sprintf(msg, data...))
}

// Warn 输出 Warn 日志
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l == nil || l.level < gormlogger.Warn {
		return
	}
	WithContext(ctx).Warn(fmt.Sprintf(msg, data...))
}

// Error 输出 Error 日志
func (l *GormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l == nil || l.level < gormlogger.Error {
		return
	}
	WithContext(ctx).Error(fmt.Sprintf(msg, data...))
}

// Trace 输出 SQL Trace
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l == nil || l.level == gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()
	log := WithContext(ctx)

	switch {
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.ignoreRecordNotFound):
		log.Error("gorm query error",
			zap.Duration("elapsed", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
			zap.Error(err),
		)
	case elapsed > l.slowThreshold && l.slowThreshold > 0:
		log.Warn("gorm slow query",
			zap.Duration("elapsed", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
	case l.level >= gormlogger.Info:
		log.Debug("gorm query",
			zap.Duration("elapsed", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
	}
}
