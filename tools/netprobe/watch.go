package main

// 长时间采样模式。
//
// 存在理由：08-02 的五轮对照测试每轮只跑约 10 秒，四种配置全部成功，
// 而一小时前的同一套代码是四种全败。这说明故障是**间歇性**的，
// 短窗口采样可能整个错过故障期——于是「测了没问题」并不等于「没问题」。
//
// 本模式每隔固定时间取一次样，只做两件必要的事：
//   1. 用与 store_client.go 相同的配置发一次真实请求
//   2. 失败时把当时的网络环境快照一起记下来
//
// 第 2 点是关键。故障发生时最想知道的不是「失败了」，而是「失败那一刻
// DNS 解析到哪、系统代理是什么状态」——事后再查已经变了。

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// fakeIPPrefixes 是 TUN 模式代理常用的假地址网段。
//
// 198.18.0.0/15 由 RFC 2544 保留给设备测试，现实中不会有服务器住在那儿；
// 240.0.0.0/4 是 Class E 保留段，部分工具也拿它做 fake-ip。
// 解析到这些网段说明流量本该被虚拟网卡接管——若同时请求失败，
// 就是「接住了但没转出去」，正是 08-02 00:46 那次故障的形态。
var fakeIPPrefixes = []string{"198.18.", "198.19.", "240.", "241.", "242."}

func runWatch(target string, interval time.Duration, logPath string) {
	if logPath == "" {
		logPath = filepath.Join("tools", "netwatch.log")
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("无法打开日志文件 %s：%v\n", logPath, err)
		return
	}
	defer func() { _ = f.Close() }()

	// 只测「当前实现」这一种配置。采样模式要回答的是「盒子现在通不通」，
	// 四种全测会把单次采样拉长到近一分钟，反而降低时间分辨率。
	client := &http.Client{Timeout: timeout}

	emit := func(format string, args ...any) {
		line := fmt.Sprintf(format, args...)
		fmt.Println(line)
		_, _ = fmt.Fprintln(f, line)
	}

	emit("")
	emit("=== netwatch 开始 %s，间隔 %s ===",
		time.Now().Format("2006-01-02 15:04:05"), interval)
	emit("配置与 store_client.go 一致：http.Client{Timeout: %s} + 默认 Transport", timeout)
	emit("Ctrl+C 结束并输出汇总")

	// 收 Ctrl+C 后走正常退出路径，才能打出汇总——挂了一晚的数据
	// 若因强杀而只剩逐行记录，还得自己数，那正是最容易数错的时候。
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	var (
		total, ok int
		failKinds = map[string]int{}
		durations []time.Duration
		firstFail time.Time
	)

	tick := time.NewTicker(interval)
	defer tick.Stop()

	sample := func() {
		total++
		ts := time.Now()

		// DNS 单独计时：它与 HTTP 失败的区分很重要——解析到假 IP
		// 而后超时，与解析正常但连不上，两者修法不同。
		ips, dnsErr := net.LookupHost("store.steampowered.com")
		ipStr := strings.Join(ips, ",")
		if dnsErr != nil {
			ipStr = "解析失败: " + dnsErr.Error()
		}

		start := time.Now()
		n, err := fetch(client, target)
		el := time.Since(start).Round(time.Millisecond)

		if err == nil {
			ok++
			durations = append(durations, el)
			// 成功时只记一行，避免一晚的日志被刷爆。
			emit("[%s] 成功 %8s %d 字节  dns=%s",
				ts.Format("15:04:05"), el, n, ipStr)
			return
		}

		kind := classify(err)
		failKinds[kind]++
		if firstFail.IsZero() {
			firstFail = ts
		}

		// 失败时才展开环境快照。此刻的状态是事后无法复原的关键证据。
		emit("[%s] 失败 %8s  %s", ts.Format("15:04:05"), el, kind)
		emit("          dns  = %s%s", ipStr, fakeIPNote(ips))
		emit("          代理 = %s", proxySnapshot())
	}

	sample() // 先立刻取一次，不必等第一个 tick

	for {
		select {
		case <-tick.C:
			sample()
		case <-stop:
			emit("")
			emit("=== netwatch 汇总 ===")
			emit("采样   : %d 次，成功 %d，失败 %d", total, ok, total-ok)
			if total > 0 {
				emit("成功率 : %.1f%%", float64(ok)/float64(total)*100)
			}
			if len(durations) > 0 {
				emit("成功耗时: 最快 %s，最慢 %s，中位 %s",
					minDur(durations), maxDur(durations), medianDur(durations))
			}
			if len(failKinds) > 0 {
				emit("首次失败: %s", firstFail.Format("15:04:05"))
				emit("失败分类:")
				for k, v := range failKinds {
					emit("    %3d 次  %s", v, k)
				}
			}
			emit("日志已写入 %s", logPath)
			return
		}
	}
}

// fakeIPNote 在解析结果落在假地址网段时附一句说明。
//
// 不自动判定为故障：fake-ip 本身是正常机制，只有「假 IP + 请求失败」
// 同时出现才指向「虚拟网卡接住了但没转发」。
func fakeIPNote(ips []string) string {
	for _, ip := range ips {
		for _, p := range fakeIPPrefixes {
			if strings.HasPrefix(ip, p) {
				return "  ← 假 IP 网段，流量本该被 TUN 接管"
			}
		}
	}
	return ""
}

// proxySnapshot 返回一行可读的代理状态，供失败时留证。
func proxySnapshot() string {
	var parts []string
	for _, k := range []string{"HTTPS_PROXY", "HTTP_PROXY"} {
		if v := os.Getenv(k); v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "环境变量未设")
	}
	parts = append(parts, "系统代理: "+winINETSummary())
	return strings.Join(parts, " | ")
}

func minDur(ds []time.Duration) time.Duration {
	m := ds[0]
	for _, d := range ds[1:] {
		if d < m {
			m = d
		}
	}
	return m
}

func maxDur(ds []time.Duration) time.Duration {
	m := ds[0]
	for _, d := range ds[1:] {
		if d > m {
			m = d
		}
	}
	return m
}

// medianDur 取中位数。
//
// 用中位数而非平均值：网络耗时的分布是长尾的，个别 2 秒的慢样本会把
// 平均值拉高到不像任何一次真实体验。
func medianDur(ds []time.Duration) time.Duration {
	s := make([]time.Duration, len(ds))
	copy(s, ds)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s[len(s)/2]
}
