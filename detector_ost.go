// detector_ost.go
//
// 本文件是 Detector 接口针对 OpenSteamTool 的实现。
//
// 检测依据来自 OST 源码分析确认的 DLL 劫持加载链：
// Steam 启动时会加载系统目录下的 dwmapi.dll 与 xinput1_4.dll，
// OST 在 Steam 根目录放置同名代理 DLL 抢先被加载，二者任一
// 被载入后都会转而加载核心的 OpenSteamTool.dll。
//
// 因此「三个文件都存在」即等价于「OST 已正确安装」。
// 不检测版本、不检测 pattern 缓存、不读取 OST 的 toml 配置——
// 那些属于注入器的内部事务。

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OSTDetector 是面向 OpenSteamTool 的环境检测器实现。
type OSTDetector struct {
	logger *Logger
}

// 编译期断言：确保 OSTDetector 完整实现了 Detector 接口。
var _ Detector = (*OSTDetector)(nil)

// NewOSTDetector 创建一个 OpenSteamTool 环境检测器。
//
// 参数：
//   - logger: 日志记录器，可为 nil
func NewOSTDetector(logger *Logger) *OSTDetector {
	return &OSTDetector{logger: logger}
}

// Name 返回注入器名称。
func (d *OSTDetector) Name() string {
	return "OpenSteamTool"
}

// Detect 检查 OpenSteamTool 是否已安装于指定的 Steam 目录。
//
// 参数：
//   - steamPath: Steam 安装根目录
//
// 返回值：
//   - *DetectorResult: 永不为 nil。Steam 路径无效时返回
//     StatusUnknown 而非 StatusMissing——前者表示「查不了」，
//     后者表示「查过了没装」，对用户而言是两种完全不同的处境
//
// 检测过程不修改任何文件，可安全地重复调用。
func (d *OSTDetector) Detect(steamPath string) *DetectorResult {
	result := &DetectorResult{
		Name:         d.Name(),
		CheckedPath:  steamPath,
		MissingFiles: []string{},
	}

	if steamPath == "" {
		result.Status = StatusUnknown
		result.Message = "尚未确定 Steam 安装路径，无法检测注入器状态"
		return result
	}

	if info, err := os.Stat(steamPath); err != nil || !info.IsDir() {
		result.Status = StatusUnknown
		result.Message = fmt.Sprintf("Steam 路径不存在或不是目录：%s", steamPath)
		d.warnf("环境检测跳过，Steam 路径无效: %s", steamPath)
		return result
	}

	for _, dll := range ostRequiredDLLs {
		if !fileExists(filepath.Join(steamPath, dll)) {
			result.MissingFiles = append(result.MissingFiles, dll)
		}
	}

	if len(result.MissingFiles) == 0 {
		result.Status = StatusReady
		result.Available = true
		result.Message = "OpenSteamTool 已就绪，可以开始入库"
		d.logf("环境检测通过: %s", steamPath)
		return result
	}

	result.Status = StatusMissing
	result.Message = fmt.Sprintf(
		"未检测到 OpenSteamTool（缺少 %s），请先按教程完成安装",
		strings.Join(result.MissingFiles, "、"),
	)
	d.logf("环境检测未通过，缺少文件: %s", strings.Join(result.MissingFiles, ", "))
	return result
}

// logf 在 logger 可用时记录信息级日志。
func (d *OSTDetector) logf(format string, args ...any) {
	if d.logger != nil {
		d.logger.Info(format, args...)
	}
}

// warnf 在 logger 可用时记录警告级日志。
func (d *OSTDetector) warnf(format string, args ...any) {
	if d.logger != nil {
		d.logger.Warn(format, args...)
	}
}
