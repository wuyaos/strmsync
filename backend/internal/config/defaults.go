// Package config 提供默认常量定义
package config

// 默认环境变量值
const (
	DefaultServerPort              = 6786
	DefaultServerHost              = "0.0.0.0"
	DefaultDBPath                  = "data.db"
	DefaultLogLevel                = "info"
	DefaultLogPath                 = "logs"
	DefaultLogToDB                 = true
	DefaultLogSQL                  = false
	DefaultLogSQLSlowMs            = 0
	DefaultLogDebugModules         = "engine,worker,filesystem"
	DefaultLogDebugRPS             = 10
	DefaultLogRotateMaxSizeMB      = 10
	DefaultLogRotateMaxBackups     = 7
	DefaultLogRotateMaxAgeDays     = 30
	DefaultLogRotateCompress       = true
	DefaultEncryptionKey           = ""
	DefaultScannerConcurrency      = 20
	DefaultScannerBatchSize        = 500
	DefaultNotifierEnabled         = false
	DefaultNotifierProvider        = ""
	DefaultNotifierBaseURL         = ""
	DefaultNotifierToken           = ""
	DefaultNotifierTimeoutSeconds  = 10
	DefaultNotifierRetryMax        = 3
	DefaultNotifierRetryBaseMs     = 1000
	DefaultNotifierDebounceSeconds = 5
	DefaultNotifierScope           = "global"
)

var defaultMediaExtensions = []string{
	".mkv",
	".iso",
	".ts",
	".mp4",
	".avi",
	".rmvb",
	".wmv",
	".m2ts",
	".mpg",
	".flv",
	".rm",
	".mov",
}
var defaultMetaExtensions = []string{
	".nfo",
	".jpg",
	".jpeg",
	".png",
	".svg",
	".ass",
	".srt",
	".sup",
	".ssa",
	".json",
}
var knownVideoExtensionSet = map[string]struct{}{
	".mkv":  {},
	".iso":  {},
	".ts":   {},
	".mp4":  {},
	".avi":  {},
	".rmvb": {},
	".wmv":  {},
	".m2ts": {},
	".mpg":  {},
	".flv":  {},
	".rm":   {},
	".mov":  {},
}

// DefaultMediaExtensions 返回默认媒体文件扩展名列表（拷贝）
func DefaultMediaExtensions() []string {
	return append([]string{}, defaultMediaExtensions...)
}

// DefaultMetaExtensions 返回默认元数据扩展名列表（拷贝）
func DefaultMetaExtensions() []string {
	return append([]string{}, defaultMetaExtensions...)
}

// IsKnownVideoExtension 判断是否为已知视频扩展名（需要传入小写ext）
func IsKnownVideoExtension(ext string) bool {
	_, ok := knownVideoExtensionSet[ext]
	return ok
}
