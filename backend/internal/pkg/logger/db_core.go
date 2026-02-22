// Package logger 提供日志数据库写入的核心实现
package logger

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

var (
	dbCoreMu       sync.Mutex
	dbCoreAttached bool
)

type dbCore struct {
	db     *gorm.DB
	level  zapcore.LevelEnabler
	fields []zapcore.Field
}

func newDBCore(db *gorm.DB, level zapcore.LevelEnabler) *dbCore {
	return &dbCore{
		db:    db,
		level: level,
	}
}

func (c *dbCore) Enabled(level zapcore.Level) bool {
	return c.level.Enabled(level)
}

func (c *dbCore) With(fields []zapcore.Field) zapcore.Core {
	if len(fields) == 0 {
		return c
	}
	merged := make([]zapcore.Field, 0, len(c.fields)+len(fields))
	merged = append(merged, c.fields...)
	merged = append(merged, fields...)
	return &dbCore{
		db:     c.db,
		level:  c.level,
		fields: merged,
	}
}

func (c *dbCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *dbCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if c.db == nil {
		return nil
	}

	allFields := fields
	if len(c.fields) > 0 {
		allFields = make([]zapcore.Field, 0, len(c.fields)+len(fields))
		allFields = append(allFields, c.fields...)
		allFields = append(allFields, fields...)
	}

	module, requestID, userAction, jobID := extractLogMeta(entry, allFields)
	message := strings.TrimSpace(entry.Message)
	if message == "" {
		message = entry.Level.String()
	}
	message = formatMessageForDB(message, allFields)

	WriteLogToDB(c.db, &model.LogEntry{
		Level:      entry.Level.String(),
		Module:     module,
		Message:    message,
		RequestID:  requestID,
		UserAction: userAction,
		JobID:      jobID,
		CreatedAt:  entry.Time,
	})

	return nil
}

func (c *dbCore) Sync() error {
	return nil
}

// AttachDBWriter 将日志写入数据库（需要先初始化日志器）
func AttachDBWriter(db *gorm.DB, level string) error {
	if db == nil {
		return errors.New("数据库连接为空")
	}

	parsed, err := parseLevel(level)
	if err != nil {
		return err
	}

	dbCoreMu.Lock()
	defer dbCoreMu.Unlock()
	if dbCoreAttached {
		return nil
	}

	if !initialized.Load() {
		return errors.New("日志未初始化")
	}

	current := L()
	next := current.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewTee(core, wrapDebugFilterCore(newDBCore(db, parsed)))
	}))
	globalLogger.Store(next)
	globalSugar.Store(next.Sugar())
	zap.ReplaceGlobals(next)
	dbCoreAttached = true
	return nil
}

func extractLogMeta(entry zapcore.Entry, fields []zapcore.Field) (*string, *string, *string, *uint) {
	var (
		module     *string
		requestID  *string
		userAction *string
		jobID      *uint
	)

	for _, field := range fields {
		switch field.Key {
		case "component", "module":
			if module == nil {
				if value := readStringField(field); value != "" {
					v := value
					module = &v
				}
			}
		case "request_id", "requestId":
			if requestID == nil {
				if value := readStringField(field); value != "" {
					v := value
					requestID = &v
				}
			}
		case "user_action":
			if userAction == nil {
				if value := readStringField(field); value != "" {
					v := value
					userAction = &v
				}
			}
		case "job_id":
			if jobID == nil {
				if value, ok := readUintField(field); ok {
					v := value
					jobID = &v
				}
			}
		}
	}

	if module == nil {
		if name := strings.TrimSpace(entry.LoggerName); name != "" {
			module = &name
		}
	}

	return module, requestID, userAction, jobID
}

func readStringField(field zapcore.Field) string {
	if field.Type == zapcore.StringType {
		return strings.TrimSpace(field.String)
	}
	if field.Type == zapcore.ByteStringType {
		if value, ok := field.Interface.([]byte); ok {
			return strings.TrimSpace(string(value))
		}
	}
	return ""
}

func readUintField(field zapcore.Field) (uint, bool) {
	switch field.Type {
	case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		return uint(field.Integer), true
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
		if field.Integer >= 0 {
			return uint(field.Integer), true
		}
	case zapcore.StringType:
		if value := strings.TrimSpace(field.String); value != "" {
			if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
				return uint(parsed), true
			}
		}
	}
	return 0, false
}

func formatMessageForDB(message string, fields []zapcore.Field) string {
	pairs := collectFieldPairs(fields)
	if len(pairs) == 0 {
		return message
	}
	return message + " " + strings.Join(pairs, " ")
}

func collectFieldPairs(fields []zapcore.Field) []string {
	skipped := map[string]struct{}{
		"module":     {},
		"component":  {},
		"request_id": {},
		"requestId":  {},
		"user_action": {},
		"job_id":     {},
		"trace_id":   {},
	}
	seen := map[string]struct{}{}
	pairs := make([]string, 0, len(fields))
	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		if _, skip := skipped[key]; skip {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		value, ok := formatFieldValue(field)
		if !ok || value == "" {
			continue
		}
		seen[key] = struct{}{}
		pairs = append(pairs, key+"="+value)
	}
	return pairs
}

func formatFieldValue(field zapcore.Field) (string, bool) {
	switch field.Type {
	case zapcore.StringType:
		return strings.TrimSpace(field.String), true
	case zapcore.ByteStringType:
		if value, ok := field.Interface.([]byte); ok {
			return strings.TrimSpace(string(value)), true
		}
	case zapcore.BoolType:
		if field.Integer == 1 {
			return "true", true
		}
		return "false", true
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
		return strconv.FormatInt(field.Integer, 10), true
	case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		return strconv.FormatUint(uint64(field.Integer), 10), true
	case zapcore.DurationType:
		return time.Duration(field.Integer).String(), true
	}
	return "", false
}
