// detector.go
//
// 本文件定义注入器环境检测的抽象接口。
//
// 检测的唯一目的是「告知用户当前环境能否正常工作」，
// 属于纯只读的诊断行为。
//
// 职责边界（架构铁律）：
//   - 只检测，不安装、不更新、不修复注入器
//   - 不读取注入器自身的配置文件
//   - 不校验注入器版本
//
// 检测结果供前端展示状态横幅，若环境不就绪则引导用户查阅
// 博客教程自行安装——本工具不代劳，避免与注入器的安装器抢活。

package main

// DetectStatus 表示环境检测的结论。
type DetectStatus string

const (
	// StatusReady 表示注入器已就绪，可以正常部署清单。
	StatusReady DetectStatus = "ready"

	// StatusMissing 表示注入器未安装或文件不完整。
	StatusMissing DetectStatus = "missing"

	// StatusUnknown 表示无法完成检测，通常因为 Steam 路径未设置
	// 或路径无效。此状态下不应断言注入器是否存在。
	StatusUnknown DetectStatus = "unknown"
)

// DetectorResult 表示一次环境检测的完整结果。
//
// 该结构体会序列化给前端，前端据此渲染状态提示。
//
// 字段说明：
//   - Name:         被检测的注入器名称
//   - Status:       检测结论
//   - Available:    是否可用，等价于 Status == StatusReady。
//     冗余提供是为了让前端能直接用于 v-if 条件判断
//   - Message:      面向用户的状态描述，可直接展示
//   - MissingFiles: 缺失的文件名列表，供前端展开显示细节
//   - CheckedPath:  实际检查的目录，便于用户核对是否为预期的 Steam 目录
type DetectorResult struct {
	Name         string       `json:"name"`
	Status       DetectStatus `json:"status"`
	Available    bool         `json:"available"`
	Message      string       `json:"message"`
	MissingFiles []string     `json:"missingFiles"`
	CheckedPath  string       `json:"checkedPath"`
}

// Detector 定义注入器环境检测接口。
type Detector interface {
	// Detect 检查注入器是否已安装且环境就绪。
	//
	// 实现必须保证任何输入下都返回非 nil 结果——检测失败本身
	// 也是一种有效结论（StatusUnknown），不应通过 error 表达，
	// 否则前端要为「检测不出来」单独写一套错误分支。
	Detect(steamPath string) *DetectorResult

	// Name 返回被检测的注入器名称。
	Name() string
}
