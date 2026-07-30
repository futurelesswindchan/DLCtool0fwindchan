// app_trial.go
//
// 源试下载与实得对比。解决一个纯界面手段无法解决的问题：
//
//	7 个源全部报「确认收录」，实得 DLC 数从 200 到 0。
//	用户看到「收录 7/7」却拿到 0 个 DLC，必然认为是本工具坏了。
//
// 根因在于 probe 只能回答「该源存在这个游戏的文件」，而用户真正关心的是
// 「这个源能给我多少 DLC」。后者非下载并解析不可知——没有任何轻量手段能
// 从 HEAD 请求里推断出内容完整度。
//
// 因此本文件的做法是：真下载、真解析、把结果摆出来让用户自己选。等待换来
// 的不只是对比表，还有免二次下载的安装——试下载拿到的包会被缓存复用。
//
// 两条设计约束（见 DECISIONS-2）：
//
//  1. **认证型源不参与自动试下载**。它消耗用户自己申请的 API 额度，
//     替用户自动花掉是不可接受的。改为单列 + 用户主动点击
//  2. **不写死任何源的优先级**。实测反例：Kingdom Rush Vengeance
//     (1367550) 上通常最优的 Hubcap 只给 2 个 DLC，而快照源给 4 个

package main

import "time"

const (
	// trialMaxConcurrent 是试下载的并发上限。
	//
	// 与 probeAll 和 enrichPackageDLCs 取同值（4）：三处都是对同一批公益
	// 代理与 GitHub 端点的并发访问，取同值便于将来统一调整。
	//
	// 不取更高：试下载是完整下载而非 HEAD，突发并发触发限流后，被限的源
	// 会落入「网络未探明」，用户看到的对比表反而更不完整。
	trialMaxConcurrent = 4

	// trialCacheTTL 是试下载结果的有效期。
	//
	// 取 30 分钟而非数小时：对比表是决策依据，用户看到的数字必须与当下
	// 的源状态基本一致。过长的缓存会让用户按着几小时前的数据选源，而
	// 上游随时可能刷新清单。
	//
	// 也不取更短：用户在对比表与游戏详情间来回切换是常见操作，每次都
	// 重下一遍既慢又浪费上游流量。
	trialCacheTTL = 30 * time.Minute
)

// 试下载的结果类别。前端据此决定呈现方式——**三种「没结果」必须视觉分家**，
// 否则用户无从判断该换源还是该检查网络。
const (
	// TrialOK 表示下载并解析成功，DLCCount 有效。
	TrialOK = "ok"

	// TrialEmpty 表示解析成功但该源确实没有 DLC。
	//
	// 与 TrialFailed 分开的理由：这是源的内容贫瘠，不是任何一方的故障。
	// 用户该做的是换源，而看到「失败」会让他去检查自己的网络。
	TrialEmpty = "empty"

	// TrialUnsupported 表示包结构不适用（典型为 MAU 形态缺 config.json）。
	//
	// 实测 MAU 镜像与 bingyu50 两个源在 MHW 上即为此种。这同样不是故障，
	// 而是该源对这个游戏的打包不完整。
	TrialUnsupported = "unsupported"

	// TrialMiss 表示该源明确未收录（probe 返回 404）。
	TrialMiss = "miss"

	// TrialFailed 表示网络或其他原因导致未能取得结论。
	//
	// 只有这一类值得提示用户重试。把上面几类也归到这里，是当前
	// 「收录 7/7 却拿到 0 个 DLC」困惑的直接来源。
	TrialFailed = "failed"

	// TrialSkipped 表示该源被有意跳过，未实际请求。
	//
	// 目前仅用于认证型源：它消耗用户额度，须由用户主动触发。
	TrialSkipped = "skipped"
)

// SourceTrial 描述单个源的试下载结果。
//
// 跨 Wails 边界的 DTO，字段全为基础类型（Wails 不认 time.Time）。
type SourceTrial struct {
	// Source 是源的显示名称。
	Source string `json:"source"`

	// Status 取值见 TrialOK 等常量。
	Status string `json:"status"`

	// DLCCount 是实得 DLC 数量，仅 Status 为 ok 时有意义。
	DLCCount int `json:"dlcCount"`

	// DepotCount 是实得 Depot 数量。
	DepotCount int `json:"depotCount"`

	// HasMainKey 标识主游戏那行是否带密钥。
	//
	// 实测三个源（hansaes / 快照 / bingyu50）主游戏均无密钥且成功部署，
	// 未见 Steam 崩溃，故这不构成「不可用」。但已装本体的正版用户这一
	// 情形仍未验证，故如实呈现让用户自行判断，而非替他隐去。
	HasMainKey bool `json:"hasMainKey"`

	// GameName 是从清单中解析出的游戏名，可能与商店名不同。
	GameName string `json:"gameName"`

	// NeedsQuota 标识该源的调用是否消耗用户的 API 额度。
	//
	// 前端据此把它排在自动试下载之外，并显式提示消耗额度。
	NeedsQuota bool `json:"needsQuota"`

	// Message 是面向用户的一句话说明，失败时含原因摘要。
	//
	// 由后端组装而非前端按 Status 拼装：同一状态在不同源上的具体原因
	// 不同（例如 failed 可能是超时也可能是连接重置），只有后端知道。
	Message string `json:"message"`

	// Cached 标识本条结果来自缓存而非本次实际请求。
	Cached bool `json:"cached"`
}

// TrialReport 是一次全源试下载的汇总。
type TrialReport struct {
	// AppID 是被查询的游戏。
	AppID string `json:"appID"`

	// Trials 是各源的结果，顺序与配置顺序一致。
	//
	// **有意不按 DLC 数排序**。排序等于替用户表达「多即是好」，而实测
	// 存在反例（Kingdom Rush Vengeance 上 DLC 少的源反而更全）。保持
	// 配置顺序则用户每次看到的排列相同，便于形成记忆。
	Trials []SourceTrial `json:"trials"`

	// BestSource 是 DLC 最多的源名，全部失败时为空。
	//
	// 仅作提示，不自动选用。命名为 Best 而界面上只说「实得最多」——
	// 「最好」是价值判断，而本工具没有依据做这个判断。
	BestSource string `json:"bestSource"`

	// MaxDLC 是各源中最高的 DLC 数。
	MaxDLC int `json:"maxDLC"`

	// UsableCount 是可用（ok 或 empty）的源数量。
	UsableCount int `json:"usableCount"`

	// QuotaSources 是需要用户主动触发的认证型源名称。
	QuotaSources []string `json:"quotaSources"`
}
