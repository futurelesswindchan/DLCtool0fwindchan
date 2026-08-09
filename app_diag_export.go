// app_diag_export.go
//
// 诊断包导出。从 app_diag.go 分出是因为脱敏逻辑与打包流程有相当篇幅，
// 且脱敏是本项目唯一「写错了会直接害到用户」的辅助功能，值得独立成文
// 便于审阅与测试。
//
// 设计前提：诊断包会被用户发到 QQ 群这类半公开场合。因此判断标准不是
// 「排障者需要什么」，而是「哪些内容即便流传出去也不损害用户」。二者
// 冲突时以后者为准——排障不便可以追问，凭据泄露无法挽回。

package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DiagnosticsResult 描述一次诊断包导出的结果。
//
// 跨 Wails 边界的 DTO，字段全为基础类型（Wails 不认 time.Time）。
type DiagnosticsResult struct {
	// Path 是生成的 zip 文件完整路径。
	Path string `json:"path"`

	// FileName 是 zip 的文件名，供前端在提示中直接展示。
	FileName string `json:"fileName"`

	// LogCount 是打包进去的日志文件数量。
	LogCount int `json:"logCount"`

	// Masked 标识配置副本中是否确实存在被脱敏的凭据。
	//
	// 前端据此决定是否强调「凭据已脱敏」——用户没填过 Token 时
	// 强调这句反而制造困惑。
	Masked bool `json:"masked"`

	// SizeKB 是 zip 的大小（KB），便于用户判断能否直接发群。
	SizeKB int64 `json:"sizeKB"`
}

// ExportDiagnostics 导出脱敏后的诊断包到数据目录。
//
// 返回值：
//   - *DiagnosticsResult: 成功时返回，含路径与统计信息
//   - error:              数据目录不可用或写入失败时返回
//
// 包内含三项：
//
//	config.masked.json  配置副本，所有 Token 字段替换为占位值
//	logs/*.log          最近 diagLogCount 个日志文件
//	环境信息.txt         版本号、系统、路径等排障必需的上下文
//
// **不含 history.json 与 packages/**。前者记录用户装过哪些游戏，后者
// 是清单内容本体——都与「工具为何出错」无关，而清单文件流入群聊会
// 使性质从工具测试变成内容分发。这条边界是刻意划下的。
//
// 导出后自动打开所在目录：用户拿到「已导出」提示却找不到文件，是这类
// 功能最常见的失败方式。
func (a *App) ExportDiagnostics() (*DiagnosticsResult, error) {
	dir, err := appDataDir()
	if err != nil {
		return nil, fmt.Errorf("数据目录不可用：%w", err)
	}

	name := fmt.Sprintf("kazeusa-诊断-%s.zip", time.Now().Format("20060102-150405"))
	zipPath := filepath.Join(dir, name)

	f, err := os.Create(zipPath)
	if err != nil {
		a.logger.Warn("创建诊断包失败: %v", err)
		return nil, fmt.Errorf("无法创建诊断包：%w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	masked, err := a.writeMaskedConfig(zw)
	if err != nil {
		_ = zw.Close()
		return nil, err
	}

	logCount := a.writeRecentLogs(zw, filepath.Join(dir, LogDirName))

	if err := writeZipEntry(zw, "环境信息.txt", a.environmentReport(dir, logCount)); err != nil {
		_ = zw.Close()
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("诊断包收尾失败：%w", err)
	}

	size := int64(0)
	if st, err := os.Stat(zipPath); err == nil {
		size = (st.Size() + 1023) / 1024
	}

	a.logger.Info("诊断包已导出 %s（%d 个日志，脱敏=%v）", name, logCount, masked)

	// 导出即打开目录，省去用户再点一次「打开数据目录」。
	// 失败不影响导出本身，故忽略返回值。
	_ = a.openInExplorer(dir)

	return &DiagnosticsResult{
		Path:     zipPath,
		FileName: name,
		LogCount: logCount,
		Masked:   masked,
		SizeKB:   size,
	}, nil
}
