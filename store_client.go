// store_client.go
//
// 本文件负责从 Steam 官方公开接口获取游戏元数据（搜索与详情），
// 是「在线入口」的第一环——用户先在这里找到游戏，再由 repo_client
// 判断该游戏是否被清单仓库收录。
//
// 设计要点：
//   - 只用 Steam 官方的 storesearch / appdetails / CDN 三处，不接入任何
//     第三方自建 API。第三方接口随时可能停服或投毒，而官方接口即便变更
//     也有社区第一时间发现。
//   - 本模块只认 AppID，完全不知道清单仓库的存在。GameSearchResult.Available
//     由 RepoClient 回填，两者的职责不交叉。
//   - 封面图 URL 由 AppID 直接拼接，不发请求验证。Steam 的 CDN 路径规则稳定，
//     多一次 HEAD 请求换取的确定性不值得。
//   - 详情结果落盘缓存 7 天。游戏的名称、简介、DLC 列表变动极慢，
//     而用户在搜索页与详情页之间来回跳转是常态。
//
// NOTE: 本模块的任何失败都不应让界面空白。Detail 在请求失败时返回仅含
// AppID 与封面 URL 的降级结果——用户至少还能看到图并继续操作。

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// storeSearchAPI 是 Steam 商店的搜索接口。
	//
	// NOTE: 这是商店页面自用的非公开接口，无需 key 也无配额限制，
	// 但也无任何稳定性承诺。它一旦失效，表现为搜索无结果而非报错，
	// 因此调用处必须记录返回条数以便排查。
	storeSearchAPI = "https://store.steampowered.com/api/storesearch/"

	// storeDetailAPI 是 Steam 商店的应用详情接口。
	storeDetailAPI = "https://store.steampowered.com/api/appdetails"

	// steamCDNBase 是 Steam 静态资源 CDN 的基址，用于拼接封面图。
	steamCDNBase = "https://cdn.cloudflare.steamstatic.com/steam/apps"

	// storeLanguage 是请求元数据时使用的语言，决定返回的名称与简介语种。
	storeLanguage = "schinese"
	// storeCountryCode 是请求时声明的区域。
	//
	// 区域会影响商店接口对部分游戏的可见性。选 CN 与目标用户一致，
	// 使界面显示的名称、发行日期与用户在自己 Steam 中看到的相符。
	storeCountryCode = "CN"

	// storeHTTPTimeout 是单次元数据请求的超时上限。
	//
	// 取 15 秒是因为国内直连 store.steampowered.com 常在 5~10 秒区间，
	// 设得过短会让本可成功的请求被误判为失败。
	storeHTTPTimeout = 15 * time.Second

	// detailCacheTTL 是商店详情缓存的有效期。
	detailCacheTTL = 7 * 24 * time.Hour

	// cacheDirName 是在线元数据缓存的子目录名，位于数据目录下。
	cacheDirName = "cache"

	// detailCacheDirName 是商店详情缓存的子目录名，位于 cacheDirName 下。
	detailCacheDirName = "detail"

	// appTypeGame 是 appdetails 接口中代表「游戏本体」的 type 取值。
	//
	// 该接口的 type 还可能是 dlc / demo / music / video / hardware 等。
	// 清单包以游戏本体为单位组织，对非本体条目查询清单必然落空，
	// 故搜索结果只保留本体。
	appTypeGame = "game"

	// searchTypeProbeConcurrency 是查询搜索结果类型时的并发上限。
	//
	// appdetails 不支持批量查询（实测 appids=a,b,c 返回空），10 条结果
	// 就得发 10 个请求。并发压到 5 是在响应速度与「别把商店接口打出
	// 限流」之间取的折中；命中详情缓存的条目不产生请求。
	searchTypeProbeConcurrency = 5

	// searchTypeProbeTimeout 是整批类型探查的总超时。
	//
	// 超时后放弃过滤、原样返回搜索结果——宁可让用户看到几个干扰项，
	// 也不能因为过滤这一步失败而让搜索整体不可用。
	searchTypeProbeTimeout = 12 * time.Second
)

// GameSearchResult 是搜索结果列表项，只含渲染一张卡片所需的最小字段。
//
// 字段说明：
//   - AppID:       游戏的 Steam AppID
//   - Name:        游戏名称（受 storeLanguage 影响）
//   - HeaderImage: 横版封面图 URL
//   - Available:   清单仓库是否收录该游戏
//
// NOTE: Available 恒为 false —— 本模块不查询仓库。该字段由 RepoClient
// 在用户进入详情页时回填，搜索阶段不对整列结果做收录检测（那会产生
// 数十个 HEAD 请求，且绝大多数结果用户根本不会点开）。
type GameSearchResult struct {
	AppID       string `json:"appID"`
	Name        string `json:"name"`
	HeaderImage string `json:"headerImage"`
	Available   bool   `json:"available"`
}

// GameDetail 是游戏详情页所需的完整元数据。
//
// 字段说明：
//   - AppID:       游戏的 Steam AppID
//   - Name:        游戏名称
//   - HeaderImage: 横版封面图 URL
//   - Description: 简短描述（纯文本，已由接口去除 HTML）
//   - Developers:  开发商列表
//   - Publishers:  发行商列表
//   - ReleaseDate: 发行日期的原始展示字符串（如「2023 年 6 月 2 日」）
//   - Screenshots: 截图缩略图 URL 列表
//   - DLCIDs:      官方声明的 DLC AppID 列表
//   - Type:        应用形态，game / dlc / demo 等。搜索过滤依赖此字段
//   - IsFree:      是否免费。与 Type 配合识别独立上架的序章、试玩版
//
// NOTE: 所有切片字段在任何情况下都不为 nil。Wails 会把 nil 切片序列化为
// JSON null，前端 v-for 遍历时直接抛错。
type GameDetail struct {
	AppID       string   `json:"appID"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	IsFree      bool     `json:"isFree"`
	HeaderImage string   `json:"headerImage"`
	Description string   `json:"description"`
	Developers  []string `json:"developers"`
	Publishers  []string `json:"publishers"`
	ReleaseDate string   `json:"releaseDate"`
	Screenshots []string `json:"screenshots"`
	DLCIDs      []string `json:"dlcIDs"`
}

// StoreClient 提供 Steam 商店元数据查询。
//
// 通过 NewStoreClient 创建。内部的 http.Client 可被并发复用，
// 因此单个实例即可服务所有前端调用，无需每次请求新建。
//
// 典型用法：
//
//	sc := NewStoreClient(logger)
//	results, err := sc.Search("Street Fighter 6")
//	detail, _ := sc.Detail(results[0].AppID)
type StoreClient struct {
	http   *http.Client
	logger *Logger
}

// NewStoreClient 创建商店元数据客户端。
//
// 参数：
//   - logger: 日志记录器，可为 nil（此时静默运行，便于单元测试）
//
// 返回值：
//   - *StoreClient: 可立即使用的客户端，任何情况下均非 nil
func NewStoreClient(logger *Logger) *StoreClient {
	return &StoreClient{
		http:   &http.Client{Timeout: storeHTTPTimeout},
		logger: logger,
	}
}

// HeaderImageURL 依据 AppID 拼接横版封面图地址。
//
// 不发起任何网络请求，故对不存在的 AppID 也会返回一个 URL——
// 此时图片加载失败，由前端的占位图兜底。
func HeaderImageURL(appID string) string {
	return fmt.Sprintf("%s/%s/header.jpg", steamCDNBase, appID)
}

// LibraryImageURL 依据 AppID 拼接竖版库容图地址。
//
// 与 HeaderImageURL 同理，不验证存在性。
func LibraryImageURL(appID string) string {
	return fmt.Sprintf("%s/%s/library_600x900.jpg", steamCDNBase, appID)
}

// ============================================================
// 搜索
// ============================================================

// storeSearchResponse 对应 storesearch 接口的返回结构。
//
// 只声明本工具用得到的字段，其余（价格、平台、评价等）一概忽略——
// 多声明一个字段就多一处随接口变动而失效的可能。
type storeSearchResponse struct {
	Total int `json:"total"`
	Items []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		// Tiny 是接口给出的小尺寸封面。此处不用它，统一由 AppID 拼
		// header.jpg，以保证列表与详情页的图源一致。
		Tiny string `json:"tiny_image"`
	} `json:"items"`
}

// Search 按关键词搜索 Steam 游戏。
//
// 纯数字输入被直接视为 AppID，跳过搜索接口转为查详情：用户手里已有
// AppID 时（社区帖子里最常见的分享形式）搜索接口往往反而搜不到它。
//
// 参数：
//   - term: 搜索关键词，或一个纯数字 AppID。前后空白会被去除
//
// 返回值：
//   - []GameSearchResult: 搜索结果，无匹配时为空切片而非 nil
//   - error:              仅在网络或解析失败时返回；「搜不到」不是错误
func (s *StoreClient) Search(term string) ([]GameSearchResult, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return []GameSearchResult{}, nil
	}

	if isNumeric(term) {
		return s.searchByAppID(term)
	}

	endpoint := fmt.Sprintf("%s?%s", storeSearchAPI, url.Values{
		"term": {term},
		"l":    {storeLanguage},
		"cc":   {storeCountryCode},
	}.Encode())

	var resp storeSearchResponse
	if err := s.getJSON(endpoint, &resp); err != nil {
		return nil, fmt.Errorf("搜索失败: %w", err)
	}

	results := make([]GameSearchResult, 0, len(resp.Items))
	for _, item := range resp.Items {
		if item.ID <= 0 {
			continue
		}
		appID := strconv.Itoa(item.ID)
		results = append(results, GameSearchResult{
			AppID:       appID,
			Name:        item.Name,
			HeaderImage: HeaderImageURL(appID),
		})
	}

	kept := s.keepGamesOnly(results)
	s.log("搜索 %q 返回 %d 条结果（过滤掉 %d 条非本体）",
		term, len(kept), len(results)-len(kept))
	return kept, nil
}

// isMainGame 判定一条详情是否为「用户真正想找的游戏本体」。
//
// 两级判据：
//
//  1. Type 必须是 game。dlc / demo / music / video 等一概排除——清单包以
//     游戏本体为单位组织，对这些条目查清单必然落空。
//  2. 免费且名称含衍生品标记者排除。「序章」「试玩版」这类内容常被作为
//     独立的免费游戏上架，Type 同样是 game（实测 The Riftbreaker 的序章
//     AppID 1293860 即如此），仅凭 Type 无从分辨。
//
// 第二级刻意收得很窄：只在「免费」的前提下才看名称。付费游戏名里带
// 「序章」的（如以序章为正式副标题的作品）不会被误杀，代价是少数免费
// 正片会漏进来——宁可多显示一条，不可把用户要找的游戏藏起来。
//
// Type 为空视为本体：那是降级结果或写于 Type 字段引入之前的旧缓存，
// 无从判定时放行。
func isMainGame(d *GameDetail) bool {
	if d == nil {
		return true
	}
	if d.Type != "" && d.Type != appTypeGame {
		return false
	}
	if d.IsFree && hasDerivativeMarker(d.Name) {
		return false
	}
	return true
}

// derivativeMarkers 是免费衍生品在名称中的常见标记。
//
// 中英文并列，因为同一游戏在不同区域设置下返回的名称语种不同。
// 「序章」而非「前传」：后者多为独立的付费作品，不该排除。
var derivativeMarkers = []string{
	"序章", "试玩", "体验版", "演示版",
	"prologue", "demo", "playtest", "beta test",
}

// hasDerivativeMarker 判断名称中是否含免费衍生品标记，大小写不敏感。
func hasDerivativeMarker(name string) bool {
	lower := strings.ToLower(name)
	for _, m := range derivativeMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// keepGamesOnly 剔除搜索结果中的 DLC、demo、原声音轨等非游戏本体条目。
//
// storesearch 接口本身不返回应用形态，只能逐条查 appdetails 的 type
// 字段。因此这一步的代价是 N 个额外请求（受 detailCacheTTL 缓存保护，
// 重复搜索同一批游戏时几乎无开销）。
//
// 失败即放行：任一条目查不到类型时保留它，整批超时则原样返回全部结果。
// 过滤是体验优化，不该成为搜索可用性的单点故障——宁可让用户看到几个
// 干扰项，也不能让搜索因此不可用。
func (s *StoreClient) keepGamesOnly(results []GameSearchResult) []GameSearchResult {
	if len(results) == 0 {
		return results
	}

	// 探查结果经 channel 回传而非直接写共享切片。
	//
	// XXX: 若让 goroutine 写 []bool，整批超时后主协程读该切片时仍有
	// goroutine 在写，构成数据竞态（go test -race 可复现）。channel
	// 天然避免了这一点：超时路径直接丢弃未收到的结果。
	type probe struct {
		idx    int
		isGame bool
	}
	ch := make(chan probe, len(results))

	sem := make(chan struct{}, searchTypeProbeConcurrency)
	for i, r := range results {
		go func(idx int, appID string) {
			sem <- struct{}{}
			defer func() { <-sem }()

			detail, err := s.Detail(appID)
			// 查不到类型就当本体放行；Type 为空说明是降级结果或写于
			// 本字段引入之前的旧缓存，同样放行
			ch <- probe{
				idx:    idx,
				isGame: err != nil || isMainGame(detail),
			}
		}(i, r.AppID)
	}

	isGame := make([]bool, len(results))
	// 未收到探查结果的条目默认放行，与「查不到就当本体」的策略一致
	for i := range isGame {
		isGame[i] = true
	}

	deadline := time.After(searchTypeProbeTimeout)
	probed := 0

	// 超时后不再等待剩余结果，但已收到的判定仍然生效。
	//
	// XXX: 最初的实现是超时即整批放行，实测在慢网下（大陆直连商店接口
	// 常有 10 秒以上的延迟）等同于过滤完全失效——用户看到的是一堆 DLC。
	// 改为部分生效后，即使只查出前几条也能滤掉相应的干扰项。
collect:
	for range results {
		select {
		case p := <-ch:
			isGame[p.idx] = p.isGame
			probed++
		case <-deadline:
			s.log("类型探查超时，已判定 %d/%d 条，其余按本体放行",
				probed, len(results))
			break collect
		}
	}

	kept := make([]GameSearchResult, 0, len(results))
	for i, r := range results {
		if isGame[i] {
			kept = append(kept, r)
		}
	}
	return kept
}

// searchByAppID 把一个纯数字输入当作 AppID 处理，返回单条结果。
//
// 查详情是为了拿到游戏名——只显示一个光秃秃的数字，用户无法确认
// 自己输入的是否正确。详情查询失败时仍返回一条以 AppID 为名的结果，
// 让用户可以继续进入详情页（那里也许能查到，或可直接尝试下载清单）。
func (s *StoreClient) searchByAppID(appID string) ([]GameSearchResult, error) {
	detail, err := s.Detail(appID)
	if err != nil {
		s.log("AppID %s 详情查询失败，返回降级结果: %v", appID, err)
		return []GameSearchResult{{
			AppID:       appID,
			Name:        appID,
			HeaderImage: HeaderImageURL(appID),
		}}, nil
	}

	return []GameSearchResult{{
		AppID:       appID,
		Name:        detail.Name,
		HeaderImage: detail.HeaderImage,
	}}, nil
}

// ============================================================
// 详情
// ============================================================

// storeDetailResponse 对应 appdetails 接口的返回结构。
//
// 该接口的外层是以 AppID 字符串为键的 map，故此处用 map 接收后再取值。
// Success 为 false 表示该 AppID 不存在或在当前区域不可见，此时 Data 为空。
type storeDetailResponse struct {
	Success bool `json:"success"`
	Data    struct {
		// Type 区分应用形态：game / dlc / demo / music / video 等。
		// 搜索结果的过滤依赖此字段，是唯一可靠的判据——按名称猜
		// （找「DLC」「Demo」字样）会漏掉中文名与非常规命名。
		Type             string   `json:"type"`
		Name             string   `json:"name"`
		// IsFree 与 Type 配合识别「序章」「试玩版」这类独立上架的免费衍生品。
		// 实测 The Riftbreaker 的序章（AppID 1293860）type 就是 game 而非
		// demo，仅靠 type 无从分辨。
		IsFree           bool     `json:"is_free"`
		ShortDescription string   `json:"short_description"`
		HeaderImage      string   `json:"header_image"`
		Developers       []string `json:"developers"`
		Publishers       []string `json:"publishers"`
		// DLC 是官方声明的 DLC AppID 列表。
		//
		// NOTE: 此列表与清单包内的 DLC 列表常有出入——清单包可能收录
		// 已下架的 DLC，也可能缺失最新的。以清单包为准，本字段仅供参考。
		DLC         []int `json:"dlc"`
		ReleaseDate struct {
			Date string `json:"date"`
		} `json:"release_date"`
		Screenshots []struct {
			Thumbnail string `json:"path_thumbnail"`
		} `json:"screenshots"`
	} `json:"data"`
}

// Detail 获取指定 AppID 的游戏详情。
//
// 命中未过期的本地缓存时直接返回，不发起网络请求。
//
// 参数：
//   - appID: 游戏的 Steam AppID，纯数字字符串
//
// 返回值：
//   - *GameDetail: 详情数据，任何情况下均非 nil
//   - error:       查询失败的原因。此时第一个返回值是仅含 AppID 与封面的
//     降级结果，调用方可以选择忽略错误直接渲染
//
// 用法示例：
//
//	detail, err := sc.Detail("1364780")
//	if err != nil {
//	    // 仍可渲染 detail，只是字段不全
//	}
func (s *StoreClient) Detail(appID string) (*GameDetail, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return fallbackDetail(appID), fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	if cached := s.readDetailCache(appID); cached != nil {
		return cached, nil
	}

	endpoint := fmt.Sprintf("%s?%s", storeDetailAPI, url.Values{
		"appids": {appID},
		"l":      {storeLanguage},
		"cc":     {storeCountryCode},
	}.Encode())

	var payload map[string]storeDetailResponse
	if err := s.getJSON(endpoint, &payload); err != nil {
		return fallbackDetail(appID), fmt.Errorf("获取 AppID %s 的详情失败: %w", appID, err)
	}

	entry, ok := payload[appID]
	if !ok || !entry.Success {
		return fallbackDetail(appID), fmt.Errorf("AppID %s 在商店中不存在或当前区域不可见", appID)
	}

	detail := &GameDetail{
		AppID:       appID,
		Name:        entry.Data.Name,
		Type:        entry.Data.Type,
		IsFree:      entry.Data.IsFree,
		HeaderImage: HeaderImageURL(appID),
		Description: entry.Data.ShortDescription,
		Developers:  nonNilSlice(entry.Data.Developers),
		Publishers:  nonNilSlice(entry.Data.Publishers),
		ReleaseDate: entry.Data.ReleaseDate.Date,
		Screenshots: make([]string, 0, len(entry.Data.Screenshots)),
		DLCIDs:      make([]string, 0, len(entry.Data.DLC)),
	}
	for _, shot := range entry.Data.Screenshots {
		detail.Screenshots = append(detail.Screenshots, shot.Thumbnail)
	}
	for _, id := range entry.Data.DLC {
		detail.DLCIDs = append(detail.DLCIDs, strconv.Itoa(id))
	}

	s.writeDetailCache(appID, detail)
	s.log("已获取 AppID %s 的详情: %s（官方声明 %d 个 DLC）", appID, detail.Name, len(detail.DLCIDs))
	return detail, nil
}

// fallbackDetail 构造仅含 AppID 与封面 URL 的降级详情。
//
// 用于任何查询失败的场合，保证界面不出现空白页。名称填 AppID 本身，
// 使用户至少知道自己在看哪个游戏。
func fallbackDetail(appID string) *GameDetail {
	return &GameDetail{
		AppID:       appID,
		Name:        appID,
		HeaderImage: HeaderImageURL(appID),
		Developers:  []string{},
		Publishers:  []string{},
		Screenshots: []string{},
		DLCIDs:      []string{},
	}
}

// ============================================================
// 缓存
// ============================================================

// cachedDetail 是详情缓存的落盘格式。
//
// 外层包一层时间戳而非依赖文件的 mtime：备份还原、文件复制等操作都会
// 重置 mtime，而缓存是否过期应当由写入时刻决定。
type cachedDetail struct {
	CachedAt time.Time   `json:"cachedAt"`
	Detail   *GameDetail `json:"detail"`
}

// detailCachePath 返回指定 AppID 的详情缓存文件路径。
//
// 数据目录不可用时返回空字符串，调用方据此跳过缓存——
// 缓存是优化而非必需，拿不到目录就直接走网络。
func detailCachePath(appID string) string {
	dir, err := appDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, cacheDirName, detailCacheDirName, appID+".json")
}

// readDetailCache 读取未过期的详情缓存。
//
// 返回 nil 表示无可用缓存，包括文件不存在、内容损坏与已过期三种情形。
// 三者都不记为错误：任一情况下的正确应对都是重新联网获取。
func (s *StoreClient) readDetailCache(appID string) *GameDetail {
	path := detailCachePath(appID)
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cached cachedDetail
	if err := json.Unmarshal(data, &cached); err != nil || cached.Detail == nil {
		return nil
	}

	if time.Since(cached.CachedAt) > detailCacheTTL {
		return nil
	}

	return cached.Detail
}

// writeDetailCache 将详情写入缓存。
//
// 写入失败只记日志不返回错误：详情已经拿到了，用户的操作没有理由
// 因为一次缓存写入失败而中断。
func (s *StoreClient) writeDetailCache(appID string, detail *GameDetail) {
	path := detailCachePath(appID)
	if path == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.log("创建详情缓存目录失败: %v", err)
		return
	}

	data, err := json.MarshalIndent(cachedDetail{
		CachedAt: time.Now(),
		Detail:   detail,
	}, "", "  ")
	if err != nil {
		s.log("序列化详情缓存失败: %v", err)
		return
	}

	if err := atomicWriteFile(path, data); err != nil {
		s.log("写入详情缓存失败: %v", err)
	}
}

// ============================================================
// 内部工具
// ============================================================

// getJSON 发起 GET 请求并将响应体解析到 out。
//
// 显式携带 Accept-Language 头：storesearch 接口的语言判定同时参考
// 查询参数与请求头，只给参数时偶尔仍返回英文结果。
func (s *StoreClient) getJSON(endpoint string, out any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Steam 商店失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Steam 商店返回状态码 %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	return nil
}

// log 在 logger 可用时记录一条信息级日志。
//
// 集中判空，免得每个调用点都写一次 if s.logger != nil。
func (s *StoreClient) log(format string, args ...any) {
	if s.logger != nil {
		s.logger.Info(format, args...)
	}
}

// isNumeric 判断字符串是否为非空的纯十进制数字。
//
// 用于区分「用户输入的是 AppID」与「用户输入的是游戏名」。
// 不用 strconv.Atoi：它接受前导正负号，而 "-123" 不是合法 AppID。
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// nonNilSlice 保证返回的切片非 nil，供跨 Wails 边界的字段使用。
//
// Wails 将 nil 切片序列化为 JSON null，前端 v-for 遍历时会直接抛错。
func nonNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
