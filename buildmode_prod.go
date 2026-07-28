//go:build !dev

// buildmode_prod.go
//
// 本文件在不带 dev 构建标签时参与编译，即所有正式构建与普通 go build。
// 与 buildmode_dev.go 互斥，二者共同保证 isDevBuild 恒有定义。

package main

// isDevBuild 标识当前是否为开发构建。
//
// 正式构建下为 false，数据目录跟随 exe 同级，以实现「拷走一个文件夹
// 即带走全部数据」的绿色软件语义。详见 config.go 的 appDataDir。
const isDevBuild = false
