// app_meta.go
//
// 本文件承载与业务无关的「应用自身元信息」类前端方法：版本号、外链打开、
// 检查更新。它们与 DLC 清单管理没有关系，故不放进已逾千行的 app.go。
//
// 三者的共同点是都属于「关于」区的诉求，且都不触碰 Steam 与注入器，
// 因而不受本项目三条铁律的约束。

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// appVersion 是当前构建的版本号，由编译期注入：
//
//	go build -ldflags "-X main.appVersion=2.0.0"
//
// 不硬编码字面量的理由：版本号一旦写进源码，就必然出现「改了 tag 忘了改
// 代码」或反之的偏差，而这种偏差在用户报障时会把排查引向错误的版本。
//
// 默认值 dev 表示未经注入的本地构建。此时 CheckUpdate 会跳过比对——
// 开发构建与任何 Release 比较都没有意义。
var appVersion = "dev"

const (
	// releaseAPIURL 是查询最新 Release 的 GitHub 接口。
	//
	// 用 /releases/latest 而非 /releases：前者只返回一条且自动跳过
	// prerelease 与 draft，正是「稳定版更新提示」需要的语义。
	releaseAPIURL = "https://api.github.com/repos/futurelesswindchan/DLCtool0fwindchan/releases/latest"

	// releasePageURL 是 Release 页面地址，作为接口失败时的兜底跳转目标。
	releasePageURL = "https://github.com/futurelesswindchan/DLCtool0fwindchan/releases"

	// updateCheckTimeout 是检查更新的超时上限。
	//
	// 取 12 秒：国内访问 api.github.com 极不稳定（见 DECISIONS-2 关于
	// probe 三态化的记录），但检查更新是用户主动点击的前台操作，
	// 等待过久不如早报失败让其自行前往发布页。
	updateCheckTimeout = 12 * time.Second

	// devVersionMarker 是未经 ldflags 注入时的版本占位值。
	devVersionMarker = "dev"
)

// UpdateInfo 描述一次检查更新的结果。
//
// 跨 Wails 边界的 DTO，故所有字段均为基础类型——Wails 的类型生成器
// 不认识 time.Time，会在前端得到一个无法使用的对象。发布时间因此以
// 已格式化的字符串传递。
type UpdateInfo struct {
	// HasUpdate 标识远端版本是否高于当前版本。
	//
	// NOTE: 开发构建（appVersion 为 dev）下恒为 false，此时
	// LatestVersion 仍会填充，便于开发者确认接口连通。
	HasUpdate bool `json:"hasUpdate"`

	// CurrentVersion 是当前运行的版本号。
	CurrentVersion string `json:"currentVersion"`

	// LatestVersion 是远端最新的版本号，已去除 v 前缀。
	LatestVersion string `json:"latestVersion"`

	// ReleaseName 是 Release 的标题，可能为空。
	ReleaseName string `json:"releaseName"`

	// ReleaseNotes 是 Release 的正文（Markdown 原文）。
	//
	// 前端应按纯文本渲染。此处内容来自外部，注入 innerHTML 等于
	// 把仓库写权限变成 XSS 入口。
	ReleaseNotes string `json:"releaseNotes"`

	// ReleaseURL 是 Release 页面地址，供「前往下载」按钮使用。
	ReleaseURL string `json:"releaseURL"`

	// PublishedAt 是发布日期，格式 2006-01-02；解析失败时为空。
	PublishedAt string `json:"publishedAt"`
}

// GetAppVersion 返回当前构建的版本号。
//
// 返回值：
//   - string: 版本号；未经 ldflags 注入的本地构建返回 "dev"
func (a *App) GetAppVersion() string {
	return appVersion
}

// OpenURL 在系统默认浏览器中打开指定链接。
//
// 参数：
//   - rawURL: 目标链接，必须是 http 或 https
//
// 返回值：
//   - *OperationResult: 校验失败时 Success 为 false 并说明原因
//
// 为何要校验 scheme：BrowserOpenURL 在 Windows 上最终交给 ShellExecute，
// 它会执行 file: 指向的程序、按注册表处理任意自定义协议。前端目前只传
// 常量链接，但这个方法一旦存在便是通用出口，日后若有人把用户输入或
// 清单源返回的字段接进来，未加限制的实现就成了任意程序启动器。
//
// NOTE: 打开本地目录的诉求已由 OpenDataDir 单独承担，故此处无需放行
// 文件路径。
func (a *App) OpenURL(rawURL string) *OperationResult {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return failure("链接为空")
	}

	u, err := url.Parse(trimmed)
	if err != nil {
		return failure("链接格式无法识别")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		a.logger.Warn("拒绝打开非 http(s) 链接: %q", trimmed)
		return failure("仅支持打开 http 或 https 链接")
	}

	if u.Host == "" {
		return failure("链接缺少主机名")
	}

	runtime.BrowserOpenURL(a.ctx, u.String())
	return success("已在浏览器中打开")
}

// GetReleasePageURL 返回项目发布页地址。
//
// 供前端在检查更新失败时提供「手动前往查看」的兜底入口，避免把 URL
// 常量复制一份到前端——两处各存一份，改域名时必漏其一。
func (a *App) GetReleasePageURL() string {
	return releasePageURL
}

// githubRelease 是 GitHub Release 接口响应中本工具用到的字段子集。
//
// 只声明所需字段，其余交给 json 解码器忽略——接口日后新增字段不会
// 使解析失败。
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

// CheckUpdate 查询 GitHub 上的最新 Release 并与当前版本比对。
//
// 返回值：
//   - *UpdateInfo: 查询成功时返回，字段含义见类型定义
//   - error:       网络不可达、接口异常或响应无法解析时返回
//
// 关于发布渠道的错位：本项目的实际发布顺序是「蓝奏云先行，GitHub Release
// 收尾」，故此处一旦报出新版本，安装包必然已经就绪——正是选择
// GitHub 作为版本信息源的原因，它天然晚于蓝奏云。
//
// 关于失败处理：本方法返回 error 而非 OperationResult。检查更新失败是
// 常态而非异常（国内访问 api.github.com 经常直接超时），前端应把它当作
// 「暂时查不到」而非「操作失败」，配合 GetReleasePageURL 引导手动查看。
//
// NOTE: 不做结果缓存。这是用户主动点击的操作，缓存反而会让「我刚发了新版
// 怎么还提示是旧的」变成新的困惑来源。
func (a *App) CheckUpdate() (*UpdateInfo, error) {
	rel, err := fetchLatestRelease()
	if err != nil {
		a.logger.Warn("检查更新失败: %v", err)
		return nil, err
	}

	latest := normalizeVersion(rel.TagName)
	info := &UpdateInfo{
		CurrentVersion: appVersion,
		LatestVersion:  latest,
		ReleaseName:    rel.Name,
		ReleaseNotes:   rel.Body,
		ReleaseURL:     rel.HTMLURL,
		PublishedAt:    formatReleaseDate(rel.PublishedAt),
	}

	// 开发构建不参与比对：dev 与任何版本号都无法比较大小，
	// 强行比出个结果只会在开发过程中反复弹出无意义的更新提示。
	if appVersion == devVersionMarker {
		a.logger.Info("开发构建跳过版本比对，远端最新为 %s", latest)
		return info, nil
	}

	info.HasUpdate = compareVersions(latest, normalizeVersion(appVersion)) > 0
	a.logger.Info("检查更新完成：当前 %s，远端 %s，需更新=%v",
		appVersion, latest, info.HasUpdate)
	return info, nil
}

// fetchLatestRelease 请求 GitHub 接口并解码最新 Release。
//
// 显式声明 Accept 与 User-Agent：GitHub 对未带 UA 的请求直接返回 403，
// 这是个只在真机上才会暴露的失败。
func fetchLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, releaseAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kazeusa-update-check")

	client := &http.Client{Timeout: updateCheckTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接更新服务器: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// 404 有明确含义：仓库尚未发布任何 Release。与网络故障区分开来，
	// 否则用户会以为是自己网络的问题而反复重试。
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("远端尚未发布任何版本")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新服务器返回异常状态 %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("解析更新信息失败: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return nil, fmt.Errorf("更新信息缺少版本号")
	}

	return &rel, nil
}

// normalizeVersion 去除版本号的 v 前缀与首尾空白。
//
// tag 的书写习惯在 v2.0.0 与 2.0.0 之间摇摆，而 ldflags 注入的值通常
// 不带前缀。统一在比对前归一，避免因一个字符判成「有新版本」。
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// formatReleaseDate 把 ISO 8601 时间戳转为 2006-01-02。
//
// 解析失败返回空字符串而非原文——半截的时间戳显示在界面上比不显示更糟。
func formatReleaseDate(raw string) string {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// compareVersions 比较两个点分版本号，返回 -1 / 0 / 1。
//
// 参数：
//   - a, b: 已经过 normalizeVersion 处理的版本号
//
// 比较规则：
//   - 按点分段逐段比较数值，段数不等时缺失段视为 0（2.0 == 2.0.0）
//   - 非数字段（如 2.0.0-beta 的后缀）解析为 0，即预发布版与正式版等同
//
// 为何不引入 semver 库：本项目的版本号形态完全由自己掌握，
// 恒为 x.y.z 三段数字。为一处比较拉进一个依赖不划算。
//
// NOTE: 若日后真的发布 2.0.0-rc1 之类的预发布版，此处会把它判为与
// 2.0.0 相同而不提示更新。届时应改用完整的 semver 实现，而不是在这里
// 打补丁——预发布版的排序规则远比看上去复杂。
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")

	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}

	for i := 0; i < n; i++ {
		av := versionSegment(as, i)
		bv := versionSegment(bs, i)
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// versionSegment 取版本号第 i 段的数值，越界或非数字均返回 0。
//
// 对 2.0.0-beta 这样的段，只截取前导数字部分——直接 Atoi 会失败并
// 使整段退化为 0，那样 2.1.0-beta 会被判为小于 2.0.5。
func versionSegment(segs []string, i int) int {
	if i >= len(segs) {
		return 0
	}

	s := segs[i]
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}

	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0
	}
	return n
}
