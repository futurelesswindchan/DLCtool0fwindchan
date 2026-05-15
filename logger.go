// logger.go
//
// 本文件提供统一的日志记录能力，用于替代散落在各处的 fmt.Errorf 和静默忽略。
// 所有关键操作（文件读写、解析、检测、安装、卸载）都应通过此模块记录日志，
// 便于后续排障时快速定位问题，而不依赖手工复现。
//
// 日志策略：
//   - 使用 Go 标准库 log 包，输出到文件和控制台
//   - 日志文件位于用户临时目录下：<TempDir>/dlctool.log
//   - 提供 Info / Warn / Error 三个级别
//   - 应用启动时自动初始化

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
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
// 通过 NewLogger() 创建，支持同时输出到文件和标准错误流。
// 日志文件路径：<os.TempDir()>/dlctool.log
type Logger struct {
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	logFile     *os.File
}

// NewLogger 创建并初始化日志记录器。
//
// 日志文件创建在系统临时目录下，使用追加模式写入。
// 若日志文件创建失败，仅输出到标准错误流（不影响程序运行）。
//
// 返回值：
//   - *Logger: 初始化完成的日志记录器实例
func NewLogger() *Logger {
	l := &Logger{}

	// 尝试创建日志文件
	logPath := filepath.Join(os.TempDir(), "dlctool.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

	var writer io.Writer
	if err != nil {
		// 日志文件创建失败，仅输出到标准错误
		writer = os.Stderr
	} else {
		l.logFile = logFile
		writer = io.MultiWriter(os.Stderr, logFile)
	}

	l.infoLogger = log.New(writer, "[INFO]  ", 0)
	l.warnLogger = log.New(writer, "[WARN]  ", 0)
	l.errorLogger = log.New(writer, "[ERROR] ", 0)

	return l
}

// Close 关闭日志文件句柄。
//
// 应在应用退出时调用，确保日志内容完整写入磁盘。
func (l *Logger) Close() {
	if l.logFile != nil {
		l.logFile.Close()
	}
}

// timestamp 生成当前时间戳字符串，格式为 2006-01-02 15:04:05。
func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Info 记录信息级别日志。
//
// 用于记录正常操作流程，如"开始解压"、"安装完成"等。
func (l *Logger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.infoLogger.Printf("%s %s", timestamp(), msg)
}

// Warn 记录警告级别日志。
//
// 用于记录非致命异常，如"manifest 文件删除失败但不影响整体流程"。
func (l *Logger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.warnLogger.Printf("%s %s", timestamp(), msg)
}

// Error 记录错误级别日志。
//
// 用于记录导致操作失败的异常，如"config.vdf 写入失败"。
func (l *Logger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.errorLogger.Printf("%s %s", timestamp(), msg)
}
