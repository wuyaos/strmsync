package logger

import "go.uber.org/zap/zapcore"

type debugFilterCore struct {
	core   zapcore.Core
	module string
}

func wrapDebugFilterCore(core zapcore.Core) zapcore.Core {
	if core == nil {
		return core
	}
	return &debugFilterCore{core: core}
}

func (c *debugFilterCore) Enabled(level zapcore.Level) bool {
	return c.core.Enabled(level)
}

func (c *debugFilterCore) With(fields []zapcore.Field) zapcore.Core {
	if len(fields) == 0 {
		return c
	}
	module := c.module
	if value := extractModuleFromFields(fields); value != "" {
		module = value
	}
	return &debugFilterCore{
		core:   c.core.With(fields),
		module: module,
	}
}

func (c *debugFilterCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *debugFilterCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	if entry.Level == zapcore.DebugLevel {
		module := c.module
		if module == "" {
			module = extractModuleFromFields(fields)
		}
		if !shouldAllowDebug(module) {
			return nil
		}
	}
	return c.core.Write(entry, fields)
}

func (c *debugFilterCore) Sync() error {
	return c.core.Sync()
}

func extractModuleFromFields(fields []zapcore.Field) string {
	for _, field := range fields {
		switch field.Key {
		case "module", "component":
			if field.Type == zapcore.StringType {
				return normalizeModule(field.String)
			}
			if field.Type == zapcore.ByteStringType {
				if value, ok := field.Interface.([]byte); ok {
					return normalizeModule(string(value))
				}
			}
		}
	}
	return ""
}
