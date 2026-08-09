//go:build !windows

package main

import (
	"fmt"
	"net/http"
	"net/url"
)

// proxyFromWinINET 在非 Windows 平台上无对应概念，返回 nil 即直连。
//
// 保留这个桩而非用别的方式规避，是为了让 main.go 不必写平台判断——
// 那种散在业务逻辑里的 if runtime.GOOS 是后续维护的负担。
func proxyFromWinINET() func(*http.Request) (*url.URL, error) {
	fmt.Println("    [WinINET] 非 Windows 平台，本项等同直连")
	return nil
}

// winINETSummary 在非 Windows 平台上无对应概念。
func winINETSummary() string { return "不适用（非 Windows）" }
