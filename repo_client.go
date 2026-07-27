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

	// lookupTimeout 是单次收录检测（HEAD 请求）的超时上限。
	//
	// 比下载超时短得多：HEAD 只取响应头，正常应在 1~3 秒内完成。
	// 三个源并发检测，取短超时可让「全都没收录」这个结论尽快给出，
	// 用户不必对着转圈等三十秒才知道要改用本地导入。
	lookupTimeout = 8 * time.Second

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
		// XXX: ManifestHub 目前已被清空，`git ls-remote --heads` 只返回 main
		// 一个分支，所有 AppID 分支均返回 404。故默认停用，不让它参与工作——
		// 启用只会让每次查找都多等一个必然失败的探测。
		//
		// 保留条目而非删除：该仓库本体仍存在，将来若恢复分支只需把 Enabled
		// 改回 true。删掉了反而要重新考证一遍仓库地址与形态。
		{Name: "ManifestHub", Kind: KindGitHubBranch, Repo: "SteamAutoCracks/ManifestHub", Enabled: false},
		{Name: "MAU", Kind: KindGitHubBranch, Repo: "Auiowu/ManifestAutoUpdate", Enabled: true},
		{Name: "MAU 镜像", Kind: KindGitHubBranch, Repo: "Satisl/MAU", Enabled: true},
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
// 三个源并发执行，总耗时取决于最慢的那个而非三者之和。
//
// 参数：
//   - appID: 游戏的 Steam AppID，纯数字字符串
//
// 返回值：
//   - []string: 收录该 AppID 的源名称，按 defaultRepoSources 的顺序排列。
//     全部未收录时为空切片而非 nil
//   - error: 仅在 appID 格式非法时返回。单个源探测失败不视为错误——
//     那与「该源未收录」对用户而言是同一件事，都只能换源
//
// 用法示例：
//
//	names, _ := rc.Lookup("1364780")
//	if len(names) == 0 {
//	    // 三源均未收录，引导用户改用本地导入
//	}
func (r *RepoClient) Lookup(appID string) ([]string, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return []string{}, fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	sources := r.enabledSources()
	// 按下标写入固定长度的切片，而非在 goroutine 中 append——
	// 这样结果顺序与源的配置顺序一致，用户每次看到的排列都相同。
	hits := make([]bool, len(sources))

	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func(idx int, s RepoSource) {
			defer wg.Done()
			hits[idx] = r.probe(s, appID)
		}(i, src)
	}
	wg.Wait()

	names := make([]string, 0, len(sources))
	for i, ok := range hits {
		if ok {
			names = append(names, sources[i].Name)
		}
	}

	r.log("AppID %s 收录检测完成：%d/%d 个源命中 %v", appID, len(names), len(sources), names)
	return names, nil
}

// probe 检测单个源是否收录指定 AppID。
//
// 只试直连地址，不走镜像链：镜像的作用是提升下载成功率，而检测阶段
// 若因代理故障误报未收录，用户会被引向本地导入这条更麻烦的路。
// 直连 HEAD 失败时宁可返回 false 并在下载阶段由镜像链兜底重试。
func (r *RepoClient) probe(src RepoSource, appID string) bool {
	// 认证型源有专用的免额度检测端点，比 HEAD 可靠：它明确区分
	// 「未收录」与「已知但尚未生成清单」，还附带清单年龄。
	if src.Kind == KindAPIZip {
		status, err := r.msiteStatus(src, appID)
		if err != nil {
			r.log("源 %s 检测 AppID %s: %v", src.Name, appID, err)
			return false
		}
		r.log("源 %s 收录 %s（%s），清单 %.1f 天前生成",
			src.Name, appID, status.GameName, status.FileAgeDays)
		return true
	}

	rawURL := sourceDownloadURL(src, appID)
	if rawURL == "" {
		return false
	}

	req, err := http.NewRequest(http.MethodHead, rawURL, nil)
	if err != nil {
		return false
	}

	resp, err := r.lookupHTTP.Do(req)
	if err != nil {
		r.log("源 %s 探测 AppID %s 失败: %v", src.Name, appID, err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
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
//   - error:  所有源与镜像均失败时返回，Message 已含尝试次数
//
// NOTE: 调用方必须负责清理返回路径所在的临时目录，无论后续解析是否成功。
// 推荐写法：
//
//	zipPath, err := rc.Fetch(appID, "")
//	if err == nil {
//	    defer func() { _ = os.RemoveAll(filepath.Dir(zipPath)) }()
//	}
func (r *RepoClient) Fetch(appID string, sourceName string) (string, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return "", fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	candidates := r.enabledSources()
	if len(candidates) == 0 {
		return "", fmt.Errorf("没有可用的清单源")
	}

	if sourceName != "" {
		candidates = filterByName(candidates, sourceName)
		if len(candidates) == 0 {
			return "", fmt.Errorf("源 %q 不存在或未启用", sourceName)
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
		return "", fmt.Errorf("创建临时目录失败: %w", err)
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
			return zipPath, nil
		}
	}

	// 全败时立即清理，不留空目录等 24 小时后才被回收。
	_ = os.RemoveAll(tempDir)
	return "", fmt.Errorf("清单包下载失败，已尝试 %d 个地址。可改用本地导入", attempts)
}

// narrowToHits 依据收录检测结果缩小候选源范围。
//
// 检测结果为空时原样返回全部候选，而非返回空列表直接判定失败：
// HEAD 请求可能因网络抖动或代理拦截而全部失败，此时仍应让下载链路
// 试一遍——检测只是优化手段，不该拥有否决下载的权力。
func (r *RepoClient) narrowToHits(candidates []RepoSource, appID string) []RepoSource {
	names, err := r.Lookup(appID)
	if err != nil || len(names) == 0 {
		return candidates
	}

	hit := make(map[string]bool, len(names))
	for _, n := range names {
		hit[n] = true
	}

	narrowed := make([]RepoSource, 0, len(names))
	for _, src := range candidates {
		if hit[src.Name] {
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
