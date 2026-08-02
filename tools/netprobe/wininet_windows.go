//go:build windows

package main

// WinINET 系统代理的读取。
//
// 这是本次诊断的核心对照项：Go 的默认 Transport 只认 HTTP(S)_PROXY
// 环境变量，而浏览器与绝大多数 Windows 程序走的是 WinINET 那一套
// （注册表 Internet Settings）。国内用户装的商用代理软件多半只写后者，
// 于是「浏览器能开、盒子超时」——正是 08-02 实测到的现象。
//
// 注意本文件只读注册表、不做 PAC 求值。AutoConfigURL（PAC 脚本）情形
// 无法只靠读键值解决，需要下载并执行 JS，代价远超收益；此处只如实报告
// 检测到了 PAC，把结论交给人判断。

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// proxyFromWinINET 返回一个按系统代理设置决定去向的 Proxy 函数。
//
// 读不到设置或未启用代理时返回 nil，此时 Transport 直连——
// 与「系统本来就没配代理」的实际行为一致。
func proxyFromWinINET() func(*http.Request) (*url.URL, error) {
	cfg, err := readWinINET()
	if err != nil {
		fmt.Printf("    [WinINET] 读取失败：%v\n", err)
		return nil
	}

	if cfg.autoConfigURL != "" {
		// PAC 无法只靠读键值处理，如实报告而不假装支持。
		fmt.Printf("    [WinINET] 检测到 PAC 脚本：%s\n", cfg.autoConfigURL)
		fmt.Println("    [WinINET] 本探针不求值 PAC，本项结果仅代表「按静态设置」的行为")
	}

	if !cfg.enabled || cfg.server == "" {
		fmt.Println("    [WinINET] 系统代理未启用 → 本项等同直连")
		return nil
	}

	proxyURL, err := normalizeProxy(cfg.server)
	if err != nil {
		fmt.Printf("    [WinINET] 代理地址无法解析（%s）：%v\n", cfg.server, err)
		return nil
	}
	fmt.Printf("    [WinINET] 使用系统代理：%s\n", proxyURL)

	return func(*http.Request) (*url.URL, error) { return proxyURL, nil }
}

// winINETSummary 返回一行系统代理状态，供采样模式在失败时留证。
//
// 与 proxyFromWinINET 分开：那个要构造 Proxy 函数并会打印说明，
// 采样模式每次失败都调用的话日志会被刷爆。
func winINETSummary() string {
	cfg, err := readWinINET()
	if err != nil {
		return "读取失败: " + err.Error()
	}
	s := "未启用"
	if cfg.enabled {
		s = "启用 " + cfg.server
	}
	if cfg.autoConfigURL != "" {
		s += "（PAC: " + cfg.autoConfigURL + "）"
	}
	return s
}

// winINETConfig 是注册表里与代理相关的三个值。
type winINETConfig struct {
	enabled       bool
	server        string
	autoConfigURL string
}

func readWinINET() (winINETConfig, error) {
	var cfg winINETConfig

	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return cfg, err
	}
	defer func() { _ = k.Close() }()

	// ProxyEnable 缺失等同未启用，不视为错误——干净系统上本就没有这个值。
	if v, _, err := k.GetIntegerValue("ProxyEnable"); err == nil {
		cfg.enabled = v != 0
	}
	if v, _, err := k.GetStringValue("ProxyServer"); err == nil {
		cfg.server = v
	}
	if v, _, err := k.GetStringValue("AutoConfigURL"); err == nil {
		cfg.autoConfigURL = v
	}
	return cfg, nil
}

// normalizeProxy 把注册表里的代理串规整成 URL。
//
// ProxyServer 有两种写法，必须分别处理：
//
//	127.0.0.1:7890                        所有协议共用
//	http=127.0.0.1:7890;https=127.0.0.1:7890   按协议分列
//
// 后者是分号分隔的键值对，直接丢给 url.Parse 会得到一个语义错误
// 但不报错的结果——那种「不报错的错」最难查，故显式拆解。
func normalizeProxy(server string) (*url.URL, error) {
	raw := strings.TrimSpace(server)

	if strings.Contains(raw, "=") {
		// 优先取 https，其次 http。本工具访问的全是 https 端点。
		var httpAddr string
		for _, part := range strings.Split(raw, ";") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(kv[0])) {
			case "https":
				raw = strings.TrimSpace(kv[1])
				httpAddr = ""
			case "http":
				if httpAddr == "" {
					httpAddr = strings.TrimSpace(kv[1])
				}
			}
		}
		if httpAddr != "" {
			raw = httpAddr
		}
	}

	if raw == "" {
		return nil, fmt.Errorf("代理地址为空")
	}
	// 注册表里通常不带协议前缀，补 http:// 才能被 url.Parse 正确识别
	// 为「主机 + 端口」而非「路径」。
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	return url.Parse(raw)
}
