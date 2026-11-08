// app_trial_run.go
//
// 试下载的执行、缓存与汇总。从 app_trial.go 分出是因为 DTO 定义需要能被
// 快速通读（它决定前端能看到什么），而执行逻辑含并发与缓存两层状态。

package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// trialCacheEntry 是单个源试下载结果的缓存条目。
type trialCacheEntry struct {
	trial SourceTrial

	// pkg 是试下载解析出的完整清单包。
	//
	// 缓存它而非只缓存计数，使用户选定源后可直接部署，无需二次下载——
	// 这让「等一轮试下载」的成本换来了实际收益，而非纯粹的等待。
	pkg *GamePackage

	at time.Time
}

// trialCache 是试下载结果的进程内缓存。
//
// 不落盘：试下载产物的价值仅限于当次决策，落盘则要处理失效、清理、
// 与 packages/ 的语义区分三件事，而这三件事的复杂度远超收益。进程退出
// 即丢弃是合适的生命周期。
type trialCache struct {
	mu      sync.Mutex
	entries map[string]trialCacheEntry
}

// newTrialCache 创建空缓存。
func newTrialCache() *trialCache {
	return &trialCache{entries: make(map[string]trialCacheEntry)}
}

// key 组装缓存键。源名可能含空格，故用不会出现在源名与 AppID 中的分隔符。
func (c *trialCache) key(appID, source string) string {
	return appID + "\x00" + source
}

// get 读取未过期的缓存条目。
func (c *trialCache) get(appID, source string) (trialCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.entries[c.key(appID, source)]
	if !ok || time.Since(e.at) > trialCacheTTL {
		return trialCacheEntry{}, false
	}
	return e, true
}

// put 写入缓存条目。
func (c *trialCache) put(appID, source string, trial SourceTrial, pkg *GamePackage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[c.key(appID, source)] = trialCacheEntry{
		trial: trial,
		pkg:   pkg,
		at:    time.Now(),
	}
}

// drop 清除某个游戏的全部缓存条目。
//
// 用户显式要求刷新时调用。只清指定 AppID 而非全部——用户想重查这个游戏，
// 不代表要放弃别的游戏已有的结果。
func (c *trialCache) drop(appID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := appID + "\x00"
	for k := range c.entries {
		if strings.HasPrefix(k, prefix) {
			delete(c.entries, k)
		}
	}
}

// TrialSources 对指定 AppID 的各源做试下载并汇总实得情况。
//
// 参数：
//   - appID:   游戏的 Steam AppID
//   - refresh: 为真时忽略缓存，强制重新下载
//
// 返回值：
//   - *TrialReport: 汇总结果，恒非 nil（个别源失败不影响整体）
//   - error:        AppID 非法或没有任何启用的源时返回
//
// 认证型源不参与自动试下载，其条目直接标为 skipped 并列入 QuotaSources，
// 由前端提示用户主动触发（见 TrialOneSource）。
//
// NOTE: 本方法耗时较长（最坏为两批超时之和）。前端应在调用期间展示进度，
// 且不应在列表页预调用——那会把一次浏览放大成数十次下载。
func (a *App) TrialSources(appID string, refresh bool) (*TrialReport, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return nil, fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	sources := a.repo.enabledSources()
	if len(sources) == 0 {
		return nil, fmt.Errorf("没有启用的清单源，请到设置页检查")
	}

	if refresh {
		a.trials.drop(appID)
	}

	report := &TrialReport{
		AppID:        appID,
		Trials:       make([]SourceTrial, len(sources)),
		QuotaSources: []string{},
	}

	// 推送所有源的初始状态（waiting）
	a.logger.Info("[source:progress] 开始推送初始 waiting 状态，appID=%s, 源数量=%d", appID, len(sources))
	for _, src := range sources {
		if src.Kind == KindAPIZip {
			// 认证型源直接标为 skipped，不推送 waiting
			continue
		}
		a.logger.Info("[source:progress] 推送 waiting 状态: appID=%s, source=%s", appID, src.Name)
		runtime.EventsEmit(a.ctx, "source:progress", map[string]any{
			"appID":  appID,
			"source": src.Name,
			"status": "waiting",
		})
	}

	// 按下标写入固定长度切片，不在 goroutine 中 append——保证结果顺序与
	// 配置顺序一致，用户每次看到的排列都相同。
	var wg sync.WaitGroup
	sem := make(chan struct{}, trialMaxConcurrent)

	for i, src := range sources {
		// 认证型源消耗用户自己申请的额度，不能替他自动花掉。
		if src.Kind == KindAPIZip {
			report.Trials[i] = SourceTrial{
				Source:     src.Name,
				Status:     TrialSkipped,
				NeedsQuota: true,
				Message:    "该源通常收录最全，但调用会消耗你的 API 额度，需手动获取",
			}
			report.QuotaSources = append(report.QuotaSources, src.Name)
			continue
		}

		if e, ok := a.trials.get(appID, src.Name); ok {
			t := e.trial
			t.Cached = true
			report.Trials[i] = t
			continue
		}

		wg.Add(1)
		go func(idx int, s RepoSource) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			report.Trials[idx] = a.trialOne(appID, s)
		}(i, src)
	}

	wg.Wait()

	summarizeTrials(report)
	a.logger.Info("AppID %s 试下载完成：%d 个源可用，最高 %d 个 DLC（%s）",
		appID, report.UsableCount, report.MaxDLC, fallbackText(report.BestSource, "无"))

	return report, nil
}

// TrialOneSource 对单个源做试下载，供认证型源由用户主动触发。
//
// 参数：
//   - appID:  游戏的 Steam AppID
//   - source: 源名称
//
// 返回值：
//   - *SourceTrial: 该源的结果，恒非 nil
//   - error:        参数非法时返回
//
// 与 TrialSources 分开暴露而非加一个「包含认证源」的开关：开关式设计下，
// 前端一旦传错参数就会静默消耗用户额度，而额度是不可退还的。两个方法则
// 使「花额度」这件事必须由一次明确的调用表达。
func (a *App) TrialOneSource(appID string, source string) (*SourceTrial, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return nil, fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("未指定源")
	}

	matched := filterByName(a.repo.enabledSources(), source)
	if len(matched) == 0 {
		return nil, fmt.Errorf("源 %q 不存在或未启用", source)
	}

	if e, ok := a.trials.get(appID, source); ok {
		t := e.trial
		t.Cached = true
		return &t, nil
	}

	t := a.trialOne(appID, matched[0])
	return &t, nil
}

// trialOne 下载并解析单个源，产出其实得情况。
//
// 复用 DownloadFromRepo 而非自行实现下载与解析：那会产生第三条解析路径
// （已有在线与本地导入两条），而三条路径的行为差异迟早会表现为「对比表
// 显示 64 个 DLC，装上却只有 60 个」这类无从排查的问题。
func (a *App) trialOne(appID string, src RepoSource) SourceTrial {
	t := SourceTrial{
		Source:     src.Name,
		NeedsQuota: src.Kind == KindAPIZip,
	}

	// 推送开始状态
	a.logger.Info("[source:progress] 推送 trying 状态: appID=%s, source=%s", appID, src.Name)
	runtime.EventsEmit(a.ctx, "source:progress", map[string]any{
		"appID":  appID,
		"source": src.Name,
		"status": "trying",
	})

	pkg, err := a.DownloadFromRepo(appID, src.Name)
	if err != nil {
		t.Status, t.Message = classifyTrialError(err)
		a.logger.Info("试下载 %s / %s：%s（%v）", appID, src.Name, t.Status, err)
		
		// 推送失败状态
		a.logger.Info("[source:progress] 推送 failed 状态: appID=%s, source=%s, message=%s", appID, src.Name, t.Message)
		runtime.EventsEmit(a.ctx, "source:progress", map[string]any{
			"appID":   appID,
			"source":  src.Name,
			"status":  "failed",
			"message": t.Message,
		})
		
		return t
	}

	t.DLCCount = len(pkg.DLCs)
	t.DepotCount = len(pkg.Depots)
	t.HasMainKey = strings.TrimSpace(pkg.MainKey) != ""
	t.GameName = pkg.GameName

	if t.DLCCount == 0 {
		t.Status = TrialEmpty
		t.Message = "该源只有本体，没有 DLC"
		if !t.HasMainKey {
			t.Message += "，且主游戏无密钥"
		}
	} else {
		t.Status = TrialOK
		t.Message = fmt.Sprintf("%d 个 DLC / %d 个 Depot", t.DLCCount, t.DepotCount)
		if !t.HasMainKey {
			t.Message += "（主游戏无密钥）"
		}
	}

	// 推送成功状态
	a.logger.Info("[source:progress] 推送 success 状态: appID=%s, source=%s, dlcCount=%d", appID, src.Name, t.DLCCount)
	runtime.EventsEmit(a.ctx, "source:progress", map[string]any{
		"appID":      appID,
		"source":     src.Name,
		"status":     "success",
		"dlcCount":   t.DLCCount,
		"depotCount": t.DepotCount,
	})

	a.trials.put(appID, src.Name, t, pkg)
	return t
}

// classifyTrialError 把下载或解析的错误归入用户可理解的类别。
//
// 分类的目的不是精确，而是**让用户知道下一步该做什么**：
//
//	miss / unsupported / empty → 换个源
//	failed                     → 检查网络后重试
//
// 当前把这几类全塌成一句「获取清单失败」，正是用户把源的贫瘠误判为工具
// 故障的直接原因。
func classifyTrialError(err error) (status, message string) {
	if err == nil {
		return TrialFailed, "未知原因"
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(msg, "config.json"),
		strings.Contains(msg, "无法识别"),
		strings.Contains(msg, "解析"):
		return TrialUnsupported, "该源的打包结构不适用于这个游戏"

	case strings.Contains(msg, "不存在或未启用"):
		return TrialMiss, "该源未收录这个游戏"

	case strings.Contains(lower, "404"),
		strings.Contains(msg, "未收录"):
		return TrialMiss, "该源未收录这个游戏"

	case strings.Contains(lower, "timeout"),
		strings.Contains(lower, "deadline"),
		strings.Contains(lower, "forcibly closed"),
		strings.Contains(lower, "connection reset"),
		strings.Contains(lower, "no such host"):
		return TrialFailed, "网络未能连通，可稍后重试"

	case strings.Contains(lower, "401"),
		strings.Contains(lower, "403"),
		strings.Contains(msg, "凭据"),
		strings.Contains(msg, "额度"):
		return TrialFailed, "凭据无效或额度不足，请到设置页检查"

	default:
		return TrialFailed, "获取失败，可稍后重试"
	}
}

// summarizeTrials 计算汇总字段。
//
// 独立为纯函数便于测试：它的失效方式是静默的——算错了 BestSource 不会
// 报错，只是把用户引向一个更差的源。
func summarizeTrials(report *TrialReport) {
	best := ""
	max := 0
	usable := 0

	for _, t := range report.Trials {
		if t.Status == TrialOK || t.Status == TrialEmpty {
			usable++
		}
		// 严格大于：并列时保留配置顺序在前的那个，使结果稳定可复现。
		if t.Status == TrialOK && t.DLCCount > max {
			max = t.DLCCount
			best = t.Source
		}
	}

	report.BestSource = best
	report.MaxDLC = max
	report.UsableCount = usable
}
