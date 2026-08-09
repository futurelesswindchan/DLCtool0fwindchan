// netprobe 用 Go 自己的 HTTP 栈实测四种传输配置，定位搜索超时的真凶。
//
// 为什么必须用 Go 而不能只用 curl 对照：curl 读环境变量、不读 WinINET，
// 行为「类似」盒子但不等于盒子。加速器若走 WFP 驱动做进程白名单，
// curl.exe 与 kazeusa.exe 拿到的待遇可能完全不同——那种情况下
// curl 通了也不能证明盒子会通。
//
// 本程序独立 main 包，不进主构建（tools/ 下不被 ./... 之外的构建引用）。
//
// 用法（在仓库根目录）：
//
//	go run ./tools/netprobe
//	go run ./tools/netprobe -term deep+rock     指定关键词
//	go run ./tools/netprobe -n 5                每种配置重复 5 次
//
// 每种网络状态各跑一次，输出贴回来对照。
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// 与 store_client.go 保持一致，否则测的不是同一件事。
const (
	endpoint = "https://store.steampowered.com/api/storesearch/"
	timeout  = 15 * time.Second
)

func main() {
	term := flag.String("term", "深岩银河", "搜索关键词")
	rounds := flag.Int("n", 3, "每种配置重复次数")
	watch := flag.Duration("watch", 0, "长时间采样的间隔，如 30s。给了则持续跑至 Ctrl+C")
	logPath := flag.String("log", "", "采样模式下追加写入的文件，默认 tools/netwatch.log")
	flag.Parse()

	target := endpoint + "?" + url.Values{
		"cc":   {"CN"},
		"l":    {"schinese"},
		"term": {*term},
	}.Encode()

	if *watch > 0 {
		runWatch(target, *watch, *logPath)
		return
	}

	fmt.Println("=== netprobe ===")
	fmt.Printf("时间   : %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("关键词 : %s\n", *term)
	fmt.Printf("超时   : %s（与 storeHTTPTimeout 一致）\n", timeout)
	fmt.Println()

	reportEnv()
	reportDNS()

	for _, c := range configs() {
		run(c, target, *rounds)
	}

	fmt.Println()
	fmt.Println("判读要点：")
	fmt.Println("  · 只有「当前实现」失败而「强制直连」成功 → 环境变量代理配错了")
	fmt.Println("  · 「强制直连」与「当前实现」都失败，「系统代理」成功 → 需支持 WinINET")
	fmt.Println("  · 四种全失败 → 加速器没覆盖本进程，或 DNS 被污染")
	fmt.Println("  · 四种全成功但耗时接近 15s → 是超时上限太紧，不是连不通")
}

// probeConfig 是一种待测的传输配置。
type probeConfig struct {
	name string
	// desc 说明这一项在回答什么问题，避免看输出时忘了测它的目的。
	desc   string
	client *http.Client
}

func configs() []probeConfig {
	return []probeConfig{
		{
			name:   "当前实现",
			desc:   "http.Client{Timeout} + 默认 Transport。只读 HTTP(S)_PROXY 环境变量",
			client: &http.Client{Timeout: timeout},
		},
		{
			name: "强制直连",
			desc: "显式关掉代理。用于判断「代理是否反而是障碍」",
			client: &http.Client{
				Timeout:   timeout,
				Transport: &http.Transport{Proxy: nil},
			},
		},
		{
			name: "系统代理",
			desc: "读 WinINET 注册表设置，即浏览器走的那条路",
			client: &http.Client{
				Timeout:   timeout,
				Transport: &http.Transport{Proxy: proxyFromWinINET()},
			},
		},
		{
			name: "调优直连",
			desc: "直连 + 放宽握手参数 + 强制 HTTP/1.1。测「是不是 HTTP/2 或握手超时」",
			client: &http.Client{
				Timeout: timeout,
				Transport: &http.Transport{
					Proxy: http.ProxyFromEnvironment,
					DialContext: (&net.Dialer{
						Timeout:   10 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext,
					TLSHandshakeTimeout:   10 * time.Second,
					ResponseHeaderTimeout: 12 * time.Second,
					ExpectContinueTimeout: 1 * time.Second,
					ForceAttemptHTTP2:     false,
					TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
					MaxIdleConns:          10,
					IdleConnTimeout:       90 * time.Second,
				},
			},
		},
	}
}

func run(c probeConfig, target string, rounds int) {
	fmt.Printf("--- %s ---\n", c.name)
	fmt.Printf("    %s\n", c.desc)

	var ok, fail int
	for i := 1; i <= rounds; i++ {
		start := time.Now()
		n, err := fetch(c.client, target)
		el := time.Since(start).Round(time.Millisecond)

		if err != nil {
			fail++
			fmt.Printf("    #%d  失败  %8s  %s\n", i, el, classify(err))
			continue
		}
		ok++
		fmt.Printf("    #%d  成功  %8s  %d 字节\n", i, el, n)
	}
	fmt.Printf("    小计：成功 %d / 失败 %d\n\n", ok, fail)
}

func fetch(cl *http.Client, target string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return 0, err
	}
	// 与 getJSON 一致：接口的语言判定同时参考查询参数与请求头。
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := cl.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return len(body), fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	return len(body), nil
}

// classify 把 Go 的网络错误归到几类可行动的原因上。
//
// 原始错误串很长且嵌套多层，直接打出来看不出区别；而「等响应头超时」
// 与「TCP 连不上」指向的修法完全不同，必须分开。
func classify(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "awaiting headers"):
		return "等响应头超时（TCP 可能已连上，但对端不回）"
	case strings.Contains(s, "TLS handshake"):
		return "TLS 握手超时（连上了但证书交换被打断）"
	case strings.Contains(s, "no such host"):
		return "DNS 解析失败"
	case strings.Contains(s, "connection was forcibly closed"),
		strings.Contains(s, "wsarecv"):
		return "连接被强制关闭（典型的中途打断）"
	case strings.Contains(s, "connectex"), strings.Contains(s, "i/o timeout"):
		return "TCP 建连超时（路由不通或被丢包）"
	case strings.Contains(s, "EOF"):
		return "对端提前关闭（EOF）"
	default:
		if len(s) > 120 {
			return s[:120] + "…"
		}
		return s
	}
}

func reportEnv() {
	fmt.Println("--- 环境变量（Go 默认 Transport 唯一认的东西）---")
	any := false
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy"} {
		if v := os.Getenv(k); v != "" {
			fmt.Printf("    %-12s = %s\n", k, v)
			any = true
		}
	}
	if !any {
		fmt.Println("    全部未设置 → 默认 Transport 会直连")
	}
	fmt.Println()
}

func reportDNS() {
	fmt.Println("--- DNS 解析 ---")
	start := time.Now()
	ips, err := net.LookupHost("store.steampowered.com")
	el := time.Since(start).Round(time.Millisecond)
	if err != nil {
		fmt.Printf("    失败（%s）：%v\n\n", el, err)
		return
	}
	fmt.Printf("    耗时 %s，得到 %d 个地址：\n", el, len(ips))
	for _, ip := range ips {
		fmt.Printf("      %s\n", ip)
	}
	fmt.Println()
}
