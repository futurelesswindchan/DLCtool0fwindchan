// repo_client.go
//
// 本文件负责从在线清单仓库查找与下载清单包，是「在线入口」的第二环——
// store_client 让用户找到游戏，本模块判断该游戏是否有清单可用并取回来。
//
// 设计要点：
//   - 聚合三个社区维护的 GitHub 分支型仓库，不自建。清单内容的生产与
//     维护属于社区的事，本工具只负责把文件放对位置。
//   - 走 codeload.github.com 而非 GitHub API：分支 zip 的下载地址可由
//     AppID 直接拼出，无需先查询分支列表，因此零 API 配额消耗，
//     也就无需引入 token 配置。用户不必为了用一个 DLC 工具去申请凭据。
//   - 收录检测用 HEAD 请求同一地址，同样免配额。仅对用户进入详情页的
//     单个 AppID 执行，不对整列搜索结果预检。
//   - 压缩包全程在 %TEMP% 内处理、用后即删。解析产物 GamePackage 序列化后
//     比原包小两三个数量级，且已是可直接使用的状态，留着原包毫无价值。
//
// NOTE: 清单包不做缓存。仓库会随游戏更新而刷新 manifest 版本，
// 缓存旧包等同于向用户提供过期清单——那比让他重新下载一次糟糕得多。

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RepoKind 区分仓库的访问形态，决定采用哪套下载逻辑。
type RepoKind string

const (
	// KindGitHubBranch 表示以 AppID 作为分支名的仓库，走 codeload 下载分支 zip。
	// 三个内置源均为此形态。
	KindGitHubBranch RepoKind = "github-branch"

	// KindZipTemplate 表示用 {app_id} 占位符拼出直链 zip 的仓库。
	//
	// NOTE: v2.0 无内置源使用此形态，仅为将来开放自定义源预留。
	// 之所以现在就留出这个分支，是为了让 Kind 字段从第一天起就有实际语义，
	// 而非等到新增第二种形态时再回头改数据结构。
	KindZipTemplate RepoKind = "zip-template"

	// KindAPIZip 表示需 Bearer 凭据的 API，返回含 .lua 与 .manifest 的 zip。
	//
	// 与前两种形态的实质差异：不走镜像链（认证请求经第三方代理转发等于
	// 把凭据交给代理），且收录检测有专用的免额度端点，不必用 HEAD 猜。
	KindAPIZip RepoKind = "api-zip"
)

const (
	// codeloadBase 是 GitHub 分支 zip 的下载基址。
	//
	// 这是 GitHub 网页端「Download ZIP」按钮实际调用的地址，走网页路由
	// 而非 API，故不受 60 次/小时的未认证配额限制。
	codeloadBase = "https://codeload.github.com"

	// appIDPlaceholder 是 KindZipTemplate 形态的 URL 中被替换的占位符。
	appIDPlaceholder = "{app_id}"

	// maxProbeConcurrency 是收录检测的并发上限。
	//
	// 源数量增至七个后，无限制并发会同时向 codeload 发六个请求。同一 IP 的
	// 突发请求可能触发限流，而被限流的响应（429/403）在检测阶段会被判为
	// probeUnknown，使该源白走一遍下载阶段的镜像链——比串行更慢。
	//
	// 取 4 是在「检测总耗时」与「限流风险」之间的折中：七个源分两批完成，
	// 最坏情形为两倍单次超时，仍在用户可接受的等待范围内。
	maxProbeConcurrency = 4

	// lookupTimeout 是单次收录检测（HEAD 请求）的超时上限。
	//
	// 比下载超时短：HEAD 只取响应头，网络通畅时 1~3 秒即可完成。
	// 各源并发检测，故此值即整个检测阶段的耗时上限。
	//
	// 取 15 秒而非更短：大陆直连 codeload 的握慢手是常态（实测 git ls-remote
	// 曾耗 21 秒才报连接失败）。过短的超时会把「网络慢」变成「探测不明」，
	// 虽有 narrowToHits 保留候选资格兜底，但每个未探明的源都要在下载阶段
	// 白走一遍四级镜像链，反而更慢。
	lookupTimeout = 15 * time.Second

	// fetchTimeout 是单次清单包下载的超时上限。
	//
	// 清单包通常在数百 KB 到数 MB 之间，但国内直连 codeload 的速度
	// 波动极大，留足余量避免把慢速但可完成的下载判为失败。
	fetchTimeout = 120 * time.Second

	// maxPackageSize 是允许下载的清单包体积上限。
	//
	// XXX: 这是防御性限制。若某个 AppID 的分支被塞入大文件（无论是误操作
	// 还是恶意），无上限的下载会耗尽磁盘。
	//
	// 取 512MB 是因为含 manifest 的完整包体积可观：实测 ARK 为 11.32MB，
	// 而其单个主 Depot 的 manifest 就有 9.7MB——体量更大的游戏会显著更高。
	maxPackageSize = 512 * 1024 * 1024
)

// downloadMirrors 是 codeload 地址的前置代理列表，按顺序尝试。
//
// 末位的空字符串代表直连。把直连放在最后而非最前，是因为国内环境下
// 直连 codeload 的失败率显著高于代理，先试代理能减少平均等待时间；
// 而代理全挂时直连仍是有效兜底。
//
// NOTE: 这些代理均为社区公益服务，随时可能失效。任一失效只表现为
// 多花几秒走到下一个，不会导致功能不可用——这正是保留四级链的意义。
var downloadMirrors = []string{
	"https://gh-proxy.org/",
	"https://cdn.gh-proxy.org/",
	"https://edgeone.gh-proxy.org/",
	"",
}

// defaultRepoSources 返回内置的清单仓库源列表。
//
// 顺序即查找与下载的优先级。M 站置首位是因为其数据完整度显著更高——
// 实测 ARK(2399830)：M 站给出 19 个 DLC 且 setManifestid 齐全，
// MAU 只给出 4 个且仅主 Depot 有 manifest。
//
// 但 M 站需用户自备凭据，Token 为空时自动跳过，故 MAU 仍是默认路径。
// 免凭据即可使用是底线，不能出现「没有 key 就不能用」的状态。
//
// 返回值：
//   - []RepoSource: 内置源，每次调用返回独立的新切片
func defaultRepoSources() []RepoSource {
	return []RepoSource{
		{Name: msiteSourceName, Kind: KindAPIZip, Repo: msiteBaseURL, Enabled: true},

		// 以下为免凭据的 GitHub 分支型源，按「单游戏数据完整度」而非
		// 「分支总数」排序。
		//
		// 这个取舍来自实测对照（样本 ARK 2399830）：
		//
		//	Hubcap          19 个 DLC / 13 个 setManifestid
		//	MAU              4 个 DLC /  1 个 setManifestid
		//	ManifestHub 快照  1 个 DLC /  3 个 setManifestid
		//
		// 快照源虽有 6.2 万分支（收录广度是 MAU 的 15 倍），但单个游戏的
		// DLC 覆盖反而更少。广度决定「找不找得到」，完整度决定「找到了够不够
		// 用」，后者对已经找到清单的用户更重要，故 MAU 系仍居前。
		{Name: "MAU", Kind: KindGitHubBranch, Repo: "Auiowu/ManifestAutoUpdate", Enabled: true},
		{Name: "MAU 镜像", Kind: KindGitHubBranch, Repo: "Satisl/MAU", Enabled: true},

		// MAU 形态的活跃 fork，作可用性冗余。
		//
		// 三者与 MAU 同为 Key.vdf + .manifest 结构，解析器按扩展名与内容
		// 结构匹配而不认文件名，故无需任何解析改动即可接入。
		//
		// 分支数经 git ls-remote 实测（2026-07-29）：bingyu50 13131、
		// hansaes 6336、tymolu233 3140。均超过 MAU 本体的 2591——MAU 本体
		// 自 2026-02 起停更，这些 fork 承接了更新。
		{Name: "MAU fork · bingyu50", Kind: KindGitHubBranch, Repo: "bingyu50/ManifestAutoUpdate", Enabled: true},
		{Name: "MAU fork · hansaes", Kind: KindGitHubBranch, Repo: "hansaes/ManifestAutoUpdate", Enabled: true},
		{Name: "MAU fork · tymolu233", Kind: KindGitHubBranch, Repo: "tymolu233/ManifestAutoUpdate", Enabled: true},

		// ManifestHub 被清空前的快照，lua 形态。
		//
		// 6.2 万分支，收录广度最大，用于兜住前面各源都没有的冷门游戏。
		// 置于末位有两个原因：数据停在 2025-07（清单版本偏旧），且单游戏
		// 的 DLC 覆盖不如 MAU 系。
		//
		// 其 lua 有两处与 Hubcap 格式的差异，均已在解析层处理：
		// setManifestid 只有两个参数（无 fileSize）、主游戏的 addappid
		// 不带密钥。
		{Name: "ManifestHub 快照", Kind: KindGitHubBranch, Repo: "SSMGAlt/ManifestHub2", Enabled: true},

		// XXX: ManifestHub 本体已被清空，`git ls-remote --heads` 只返回 main
		// 一个分支，所有 AppID 分支均返回 404。故默认停用，不让它参与工作——
		// 启用只会让每次查找都多等一个必然失败的探测。
		//
		// 保留条目而非删除：该仓库本体仍存在，将来若恢复分支只需把 Enabled
		// 改回 true。删掉了反而要重新考证一遍仓库地址与形态。
		{Name: "ManifestHub", Kind: KindGitHubBranch, Repo: "SteamAutoCracks/ManifestHub", Enabled: false},
	}
}

// RepoClient 提供清单包的查找与下载。
//
// 通过 NewRepoClient 创建。源列表在每次操作时从 ConfigManager 实时读取
// 而非构造时固化——用户可能在设置页改动启用状态，缓存一份会让改动不生效。
//
// 典型用法：
//
//	rc := NewRepoClient(config, logger)
//	names, _ := rc.Lookup("1364780")     // 哪些源收录了
//	zipPath, _ := rc.Fetch("1364780", names[0])
//	defer os.RemoveAll(filepath.Dir(zipPath))
type RepoClient struct {
	config     *ConfigManager
	lookupHTTP *http.Client
	fetchHTTP  *http.Client
	logger     *Logger
}

// NewRepoClient 创建清单仓库客户端。
//
// 两个 http.Client 分开持有是因为收录检测与下载的合理超时相差一个数量级
// （见 lookupTimeout 与 fetchTimeout），共用一个会迫使检测也等到两分钟。
//
// 参数：
//   - config: 配置管理器，用于读取源列表。可为 nil，此时退化为使用内置源
//   - logger: 日志记录器，可为 nil（此时静默运行，便于单元测试）
//
// 返回值：
//   - *RepoClient: 可立即使用的客户端，任何情况下均非 nil
func NewRepoClient(config *ConfigManager, logger *Logger) *RepoClient {
	return &RepoClient{
		config:     config,
		lookupHTTP: &http.Client{Timeout: lookupTimeout},
		fetchHTTP:  &http.Client{Timeout: fetchTimeout},
		logger:     logger,
	}
}

// enabledSources 返回当前启用的源列表。
//
// 配置不可用或列表为空时回退到内置源：在线功能是本工具的主路径，
// 让它因为一份读不到的配置而彻底失效并不合理。
func (r *RepoClient) enabledSources() []RepoSource {
	var all []RepoSource
	if r.config != nil {
		all = r.config.Get().RepoSources
	}
	if len(all) == 0 {
		all = defaultRepoSources()
	}

	enabled := make([]RepoSource, 0, len(all))
	for _, src := range all {
		if !src.Enabled || src.Repo == "" {
			continue
		}
		// 需凭据的源在未配置凭据时静默跳过。
		//
		// 跳过而非报错：用户从未打算使用它，界面上不该出现「凭据缺失」
		// 这类看似故障的提示。M 站对多数用户就是不存在的。
		if src.Kind == KindAPIZip && strings.TrimSpace(src.Token) == "" {
			continue
		}
		enabled = append(enabled, src)
	}
	return enabled
}

// ============================================================
// 收录检测
// ============================================================

// Lookup 并发询问所有启用的源，返回收录了该 AppID 的源名称。
//
// 用 HEAD 请求探测下载地址是否存在，不消耗 GitHub API 配额。
// 所有源并发执行，总耗时取决于最慢的那个而非各源之和。
//
// 参数：
//   - appID: 游戏的 Steam AppID，纯数字字符串
//
// 返回值：
//   - []string: 收录该 AppID 的源名称，按配置顺序排列。全部未收录时为空切片而非 nil
//   - error: 仅在 appID 格式非法时返回。单个源探测失败不视为错误——
//     那与「该源未收录」对用户而言是同一件事，都只能换源
//
// NOTE: 本方法只报告「确定收录」的源，探测未得出结论者不计入。故其返回值
// 适合展示给用户，但不适合用来排除下载候选——那需要区分「确定未收录」与
// 「没探明白」，见 narrowToHits。
//
// 用法示例：
//
//	names, _ := rc.Lookup("1364780")
//	if len(names) == 0 {
//	    // 无源确认收录，引导用户改用本地导入
//	}
func (r *RepoClient) Lookup(appID string) ([]string, error) {
	sources, results, err := r.probeAll(appID)
	if err != nil {
		return []string{}, err
	}

	names := make([]string, 0, len(sources))
	for i, res := range results {
		if res == probeHit {
			names = append(names, sources[i].Name)
		}
	}

	r.log("AppID %s 收录检测完成：%d/%d 个源确认收录 %v", appID, len(names), len(sources), names)
	return names, nil
}

// probeAll 并发探测所有启用的源，返回源列表与与之下标对应的探测结果。
//
// 两个返回切片长度相同且下标一一对应。按下标写入固定长度的切片而非在
// goroutine 中 append，以保证结果顺序与源的配置顺序一致——用户每次看到的
// 排列都相同。
func (r *RepoClient) probeAll(appID string) ([]RepoSource, []probeResult, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return nil, nil, fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	sources := r.enabledSources()
	results := make([]probeResult, len(sources))

	// 以带缓冲的 channel 作信号量限制并发，避免同时向 codeload 发起
	// 过多请求而触发限流。
	sem := make(chan struct{}, maxProbeConcurrency)

	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func(idx int, s RepoSource) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = r.probe(s, appID)
		}(i, src)
	}
	wg.Wait()

	return sources, results, nil
}

// probeResult 是单次收录检测的三态结果。
//
// 之所以需要三态而非布尔值：「确定未收录」与「探测不成功」对下载阶段的
// 含义完全不同。前者可安全跳过该源，后者必须保留候选资格——大陆直连
// codeload 超时是常态（实测 git ls-remote 曾耗 21 秒仍失败，而同一时刻
// raw.githubusercontent 可正常访问），若把超时当作未收录，用户会在明明
// 有清单的情况下被引向本地导入这条更麻烦的路。
type probeResult int

const (
	// probeMiss 表示服务端明确回应了「无此资源」，可信地排除该源。
	probeMiss probeResult = iota

	// probeHit 表示确认收录。
	probeHit

	// probeUnknown 表示探测未得出结论（超时、连接重置、DNS 失败等）。
	// 该源保留候选资格，交由下载阶段的镜像链重试。
	probeUnknown
)

// probe 检测单个源是否收录指定 AppID。
//
// 只试直连地址，不走镜像链：镜像的作用是提升下载成功率，而检测阶段
// 若因代理故障误报未收录，用户会被引向本地导入这条更麻烦的路。
// 直连不通时返回 probeUnknown，由下载阶段的镜像链兜底。
func (r *RepoClient) probe(src RepoSource, appID string) probeResult {
	// 认证型源有专用的免额度检测端点，比 HEAD 可靠：它明确区分
	// 「未收录」与「已知但尚未生成清单」，还附带清单年龄。
	if src.Kind == KindAPIZip {
		status, err := r.msiteStatus(src, appID)
		if err != nil {
			// 认证型源的专用端点能明确区分「未收录」与其他故障，
			// 故此处的错误一律视为未得出结论而非未收录。
			// 凭据失效、额度耗尽都属此类——它们与该游戏有无清单无关。
			r.log("源 %s 检测 AppID %s: %v", src.Name, appID, err)
			return probeUnknown
		}
		r.log("源 %s 收录 %s（%s），清单 %.1f 天前生成",
			src.Name, appID, status.GameName, status.FileAgeDays)
		return probeHit
	}

	rawURL := sourceDownloadURL(src, appID)
	if rawURL == "" {
		return probeMiss
	}

	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return probeMiss
	}

	resp, err := r.lookupHTTP.Do(req)
	if err != nil {
		// 传输层失败（超时、连接重置、DNS）无从判断资源是否存在。
		r.log("源 %s 探测 AppID %s 未得出结论: %v", src.Name, appID, err)
		return probeUnknown
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
		return probeHit
	case resp.StatusCode == http.StatusNotFound:
		// 唯一可信的「未收录」信号：服务端明确回应了无此分支。
		return probeMiss
	default:
		// 5xx、429、403 等均属服务端或中间设备的临时状态，
		// 与该 AppID 有无清单无关。
		r.log("源 %s 探测 AppID %s 返回状态码 %d，不作未收录处理",
			src.Name, appID, resp.StatusCode)
		return probeUnknown
	}
}

// sourceDownloadURL 依据源的形态拼出清单包的直连下载地址。
//
// 返回空字符串表示该源的 Kind 无法识别，调用方应跳过它。
func sourceDownloadURL(src RepoSource, appID string) string {
	switch src.Kind {
	case KindGitHubBranch:
		return fmt.Sprintf("%s/%s/zip/refs/heads/%s", codeloadBase, src.Repo, appID)
	case KindZipTemplate:
		return strings.ReplaceAll(src.Repo, appIDPlaceholder, appID)
	case KindAPIZip:
		return strings.TrimSuffix(src.Repo, "/") + msiteManifestPath + appID
	default:
		return ""
	}
}

// ============================================================
// 下载
// ============================================================

// Fetch 下载指定 AppID 的清单包到临时目录。
//
// 本方法只负责取回压缩包，不做解压与解析——那两步复用本地导入的既有
// 流程（processZipFromPath），使在线与离线两条路径产出完全一致的
// GamePackage，避免出现「在线装的和本地装的行为不同」这类问题。
//
// 参数：
//   - appID:      游戏的 Steam AppID，纯数字字符串
//   - sourceName: 指定源名称。留空表示按配置顺序自动尝试所有启用的源
//
// 返回值：
//   - string: 下载得到的 zip 文件完整路径
//   - string: 实际命中的源名称。自动模式下调用方无从预知，故一并返回，
//     供 GamePackage.Source 记录真实来源而非「自动」这类无信息量的值
//   - error:  所有源与镜像均失败时返回，消息已含尝试次数
//
// NOTE: 调用方必须负责清理返回路径所在的临时目录，无论后续解析是否成功。
// 推荐写法：
//
//	zipPath, srcName, err := rc.Fetch(appID, "")
//	if err == nil {
//	    defer func() { _ = os.RemoveAll(filepath.Dir(zipPath)) }()
//	}
func (r *RepoClient) Fetch(appID string, sourceName string) (string, string, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return "", "", fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	candidates := r.enabledSources()
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("没有可用的清单源")
	}

	if sourceName != "" {
		candidates = filterByName(candidates, sourceName)
		if len(candidates) == 0 {
			return "", "", fmt.Errorf("源 %q 不存在或未启用", sourceName)
		}
	} else {
		// 自动模式下先做一次收录检测，把未收录的源排除在外。
		//
		// 不做这一步的话，未收录的源会把四级镜像链完整走一遍才放弃：
		// 每个镜像都如实返回 404，而 404 与「代理故障」在下载逻辑里
		// 无法区分，只能继续重试。实测一个未收录的源要白耗约 20 秒。
		// 检测本身三源并发，代价不到 3 秒。
		candidates = r.narrowToHits(candidates, appID)
	}

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return "", "", fmt.Errorf("创建临时目录失败: %w", err)
	}

	zipPath := filepath.Join(tempDir, appID+".zip")
	attempts := 0

	for _, src := range candidates {
		rawURL := sourceDownloadURL(src, appID)
		if rawURL == "" {
			r.log("源 %s 的 Kind %q 无法识别，已跳过", src.Name, src.Kind)
			continue
		}

		// 认证型源不走镜像链：把带凭据的请求交给第三方公益代理转发，
		// 等于把用户的凭据暴露给代理运营方。宁可直连失败。
		mirrors := downloadMirrors
		if src.Kind == KindAPIZip {
			mirrors = []string{""}
		}

		for _, mirror := range mirrors {
			attempts++
			target := mirror + rawURL
			if err := r.download(target, zipPath, src.Token); err != nil {
				r.log("下载失败（源 %s / 第 %d 次尝试）: %v", src.Name, attempts, err)
				continue
			}

			r.log("清单包下载成功：AppID %s 来自源 %s，共尝试 %d 次", appID, src.Name, attempts)
			return zipPath, src.Name, nil
		}
	}

	// 全败时立即清理，不留空目录等 24 小时后才被回收。
	_ = os.RemoveAll(tempDir)
	return "", "", fmt.Errorf("清单包下载失败，已尝试 %d 个地址。可改用本地导入", attempts)
}

// narrowToHits 依据收录检测结果缩小候选源范围。
//
// 只排除**确定未收录**（服务端明确回应 404）的源；探测未得出结论者予以保留。
// 这个区分是必要的：大陆直连 codeload 超时是常态，若把超时当作未收录，
// 会出现「明明有清单却提示需要本地导入」——而下载阶段的镜像链本可救回。
//
// 排除后的候选保持原有顺序，确认收录的源不因排序而提前。故最终尝试顺序
// 仍是配置顺序，与用户在设置页看到的一致。
//
// 全部候选都被排除时原样返回全部候选：检测只是优化手段，不该拥有否决
// 下载的权力。
func (r *RepoClient) narrowToHits(candidates []RepoSource, appID string) []RepoSource {
	probed, results, err := r.probeAll(appID)
	if err != nil {
		return candidates
	}

	narrowed := narrowByProbeResults(candidates, probed, results)
	if len(narrowed) < len(candidates) {
		r.log("AppID %s 排除 %d 个明确未收录的源，保留 %d 个候选",
			appID, len(candidates)-len(narrowed), len(narrowed))
	}
	return narrowed
}

// narrowByProbeResults 依据探测结果从候选中排除确定未收录的源。
//
// 从 narrowToHits 中抽出的纯逻辑部分，不涉及网络，便于测试覆盖。
//
// 参数：
//   - candidates: 待筛选的候选源
//   - probed:     被探测的源列表，与 results 下标一一对应
//   - results:    探测结果，长度须与 probed 相同
//
// 返回值：
//   - []RepoSource: 保留下来的候选，维持 candidates 的原有顺序。
//     无源可排除、或排除后为空时，原样返回 candidates
func narrowByProbeResults(candidates []RepoSource, probed []RepoSource, results []probeResult) []RepoSource {
	missed := make(map[string]bool, len(probed))
	for i, res := range results {
		if i >= len(probed) {
			break
		}
		if res == probeMiss {
			missed[probed[i].Name] = true
		}
	}
	if len(missed) == 0 {
		return candidates
	}

	narrowed := make([]RepoSource, 0, len(candidates))
	for _, src := range candidates {
		if !missed[src.Name] {
			narrowed = append(narrowed, src)
		}
	}
	if len(narrowed) == 0 {
		return candidates
	}
	return narrowed
}

// download 从指定地址下载文件到本地路径。
//
// 下载至 .tmp 后重命名提交：中途失败时目标路径上不会留下半截 zip，
// 否则下一轮镜像重试会误以为文件已就绪。
// token 非空时附加 Bearer 认证头。
func (r *RepoClient) download(rawURL string, destPath string, token string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.fetchHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// 认证与额度问题需给出可操作的说明，不能只报状态码——
		// 「401」对用户毫无意义，「凭据已过期，需去续期」才有。
		if token != "" {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			return msiteAuthError(resp.StatusCode, body)
		}
		return fmt.Errorf("返回状态码 %d", resp.StatusCode)
	}

	tmpPath := destPath + TempFileSuffix
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}

	// LimitReader 多给 1 字节：读满上限说明源文件确实超限，
	// 而恰好等于上限则属合法。
	written, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxPackageSize+1))
	closeErr := f.Close()

	switch {
	case copyErr != nil:
		_ = os.Remove(tmpPath)
		return fmt.Errorf("写入失败: %w", copyErr)
	case closeErr != nil:
		_ = os.Remove(tmpPath)
		return fmt.Errorf("关闭文件失败: %w", closeErr)
	case written > maxPackageSize:
		_ = os.Remove(tmpPath)
		return fmt.Errorf("清单包体积超过 %d MB 上限", maxPackageSize/1024/1024)
	case written == 0:
		_ = os.Remove(tmpPath)
		return fmt.Errorf("下载内容为空")
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("提交文件失败: %w", err)
	}
	return nil
}

// ============================================================
// 内部工具
// ============================================================

// filterByName 从源列表中筛出指定名称的源。
//
// 名称比对忽略大小写与首尾空白：该值可能来自 GameRecord.Source，
// 而历史记录跨版本存在，源的显示名称大小写有过调整的可能。
func filterByName(sources []RepoSource, name string) []RepoSource {
	name = strings.TrimSpace(name)
	for _, src := range sources {
		if strings.EqualFold(strings.TrimSpace(src.Name), name) {
			return []RepoSource{src}
		}
	}
	return nil
}

// log 在 logger 可用时记录一条信息级日志。
func (r *RepoClient) log(format string, args ...any) {
	if r.logger != nil {
		r.logger.Info(format, args...)
	}
}
