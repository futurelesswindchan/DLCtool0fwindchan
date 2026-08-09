// logger.go
// logger.go
//
// 本文件提供统一的日志记录能力，所有关键操作（配置读写、清单解析、
// 环境检测、部署、卸载、仓库拉取）都应通过此模块记录，便于用户
// 报障时直接提供日志而不必手工复现。
//
// 日志策略：
//   - 位置：~/.kazeusa/logs/kazeusa.log
//     v1.4 曾写在 %TEMP% 下，会被系统清理临时文件时连带抹掉，
//     导致用户报 bug 时拿不到任何线索，故迁移至持久化目录。
//   - 轮转：单文件超过 5MB 时改名为 .1，最多保留 3 份，
//     避免长期使用后日志无限膨胀。
//   - 级别：Info / Warn / Error 三级，同时输出到文件与标准错误流。
//   - 降级：日志目录不可用时退化为仅输出标准错误，绝不阻断应用启动。
//   - 敏感数据：depot 解密密钥与 API 凭据一律不得完整写入日志。
//     需要标识某个密钥时用 maskSecret 截断（见文件末尾）。

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// maxLogSize 是单个日志文件的大小上限（字节），超过则触发轮转。
	maxLogSize = 5 * 1024 * 1024

	// maxLogBackups 是保留的历史日志份数，不含当前正在写入的文件。
	// 即磁盘上最多存在 kazeusa.log + .1 + .2 + .3 共 4 个文件。
	maxLogBackups = 3

	// logFileName 是当前日志文件名。
	logFileName = "kazeusa.log"
)

// LogLevel 定义日志级别。
type LogLevel int

const (
	// LogLevelInfo 信息级别，记录正常操作流程。
	LogLevelInfo LogLevel = iota
	// LogLevelWarn 警告级别，记录非致命但需关注的异常。
	LogLevelWarn
	// LogLevelError 错误级别，记录导致操作失败的异常。
	LogLevelError
)

// Logger 是应用的统一日志记录器。
//
// 通过 NewLogger() 创建，并发安全：Wails 为每个前端调用启用独立
// goroutine，多个操作可能同时写日志。
//
// 所有写入方法在轮转发生时会短暂持锁，正常情况下开销可忽略。
type Logger struct {
	mu      sync.Mutex
	writer  io.Writer
	logFile *os.File
	logPath string
	size    int64
}

// NewLogger 创建并初始化日志记录器。
//
// 日志文件位于 ~/.kazeusa/logs/ 下，使用追加模式写入。
// 若目录创建或文件打开失败，退化为仅输出到标准错误流——
// 日志系统本身的故障不应导致应用无法启动。
//
// 返回值：
//   - *Logger: 初始化完成的记录器，任何情况下均非 nil 且可安全调用
func NewLogger() *Logger {
	l := &Logger{writer: os.Stderr}

	dir, err := appDataDir()
	if err != nil {
		return l
	}

	logDir := filepath.Join(dir, LogDirName)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return l
	}

	l.logPath = filepath.Join(logDir, logFileName)
	l.open()
	return l
}

// open 打开日志文件并记录其当前大小。
//
// 调用方需自行保证并发安全。失败时保留 os.Stderr 作为输出目标。
func (l *Logger) open() {
	f, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		l.writer = os.Stderr
		return
	}

	l.logFile = f
	l.writer = io.MultiWriter(os.Stderr, f)

	// 追加模式下需知道已有大小，否则重启后轮转判断会从 0 开始，
	// 导致文件实际远超上限才触发轮转。
	if info, err := f.Stat(); err == nil {
		l.size = info.Size()
	}
}

// rotate 将当前日志文件归档，并开启一个新的空文件。
//
// 归档策略为逐级后移：.2 → .3，.1 → .2，当前文件 → .1。
// 最旧的一份（超出 maxLogBackups）被自然覆盖丢弃。
//
// 调用方必须已持有锁。任一环节失败都会尝试重新打开日志文件，
// 保证记录器在轮转出错后依然可用。
func (l *Logger) rotate() {
	if l.logPath == "" {
		return
	}

	if l.logFile != nil {
		_ = l.logFile.Close()
		l.logFile = nil
	}

	// 从最旧的开始后移，避免覆盖尚未挪走的文件。
	for i := maxLogBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", l.logPath, i)
		dst := fmt.Sprintf("%s.%d", l.logPath, i+1)
		if fileExists(src) {
			_ = os.Remove(dst)
			_ = os.Rename(src, dst)
		}
	}

	first := l.logPath + ".1"
	_ = os.Remove(first)
	_ = os.Rename(l.logPath, first)

	l.size = 0
	l.open()
}

// Close 关闭日志文件句柄。
//
// 必须在应用退出时调用（经由 Wails 的 OnShutdown 回调），
// 否则缓冲区中的最后几条日志可能不会落盘。
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		_ = l.logFile.Sync()
		_ = l.logFile.Close()
		l.logFile = nil
	}
	l.writer = os.Stderr
}

// Path 返回当前日志文件的完整路径。
//
// 供前端的「打开日志目录」功能使用。日志系统降级运行时返回空字符串。
func (l *Logger) Path() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logPath
}

// write 是所有日志级别的统一出口，负责格式化、轮转检查与落盘。
func (l *Logger) write(tag, format string, args ...any) {
	line := fmt.Sprintf("[%s] %s %s\n",
		tag,
		time.Now().Format("2006-01-02 15:04:05"),
		fmt.Sprintf(format, args...),
	)

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil && l.size+int64(len(line)) > maxLogSize {
		l.rotate()
	}

	n, err := io.WriteString(l.writer, line)
	if err == nil {
		l.size += int64(n)
	}
}

// Info 记录信息级别日志。
//
// 用于记录正常操作流程，如「配置加载成功」「清单已部署」。
func (l *Logger) Info(format string, args ...any) {
	l.write("INFO ", format, args...)
}

// Warn 记录警告级别日志。
//
// 用于记录不影响主流程的异常，如「主仓库源不可达，已回退镜像」。
func (l *Logger) Warn(format string, args ...any) {
	l.write("WARN ", format, args...)
}

// Error 记录错误级别日志。
//
// 用于记录导致操作失败的异常，如「清单文件写入失败」。
func (l *Logger) Error(format string, args ...any) {
	l.write("ERROR", format, args...)
}

// maskSecret 将密钥类字符串截断为前 8 位，供日志安全引用。
//
// depot 解密密钥为 64 位十六进制、API 凭据长度不定，二者完整写入日志后
// 会随用户报障的日志文件一同外流。前 8 位足以区分「是哪一个密钥」与
// 「密钥是否为空」这两类排障需求，而不足以还原原值。
//
// 用法：
//
//	logger.Info("depot %s 采用密钥 %s", depotID, maskSecret(key))
//	// 输出：depot 2399831 采用密钥 320e0bcc…（共 64 位）
//
// 空字符串返回「(空)」而非空白，否则日志中「密钥为空」与「未记录密钥」
// 两种情形无法区分——前者是数据缺陷，后者只是没写这条日志。
func maskSecret(s string) string {
	if s == "" {
		return "(空)"
	}
	if len(s) <= 8 {
		// 短于 8 位的不是合法密钥，整体暴露也无泄露风险，
		// 且此时更需要看到原值以判断数据从哪一步开始出错。
		return fmt.Sprintf("%s（仅 %d 位，疑似异常）", s, len(s))
	}
	return fmt.Sprintf("%s…（共 %d 位）", s[:8], len(s))
}
