//go:build dev

// buildmode_dev.go
//
// 本文件仅在带 dev 构建标签时参与编译，用于标识开发构建。
// wails dev 会自动携带该标签（见 Wails 的 internal/app/app_dev.go），
// 故无需人工配置任何环境变量。

package main

// isDevBuild 标识当前是否为开发构建。
//
// 影响数据目录选址：开发构建下 exe 位于构建输出目录，数据若写在其同级
// 会随下次构建清理而消失，故改用用户主目录以跨构建保留配置与历史。
// 详见 config.go 的 appDataDir。
const isDevBuild = true
