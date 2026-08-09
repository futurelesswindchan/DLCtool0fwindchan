// msite_client.go
//
// 本文件封装 Hubcap Manifest（以下称 M 站）的 API 访问，是 KindAPIZip
// 形态的具体实现。
//
// 它在架构中的位置与本地导入等同：都是可选增强，不是底层主逻辑。
// 用户不提供凭据时整条链路自动跳过，MAU 仍然是默认路径。
//
// 端点选用（实测 2026-07-27，样本 ARK 2399830）：
//
//	/api/v1/status/{app_id}    免额度，收录检测。附带 game_name 与 file_age_days
//	/api/v1/manifest/{app_id}  计额度，返回含 .lua + .manifest 的 zip
//	/api/v1/user/stats         免额度，读额度与凭据到期日
//
// 为何用 /manifest 而不用 /lua：
//
//	两者同样消耗 1 次额度，但 /lua 返回的脚本内 setManifestid 数量为 0，
//	而 /manifest 包内的 .lua 有 13 个（对应 13 个 depot）。缺少 setManifestid
//	不只是「版本不钉住」，还违反部署脚本的格式契约。多传 11MB 换取数据
//	完整性是值得的——包内的 .manifest 文件解析完即丢，OST 会自行下载。
//
// NOTE: 该站明确禁止爬库（Scraping our database will result in a permanent
// ban without refund）。故本工具只按用户指定的单个 AppID 请求，绝不遍历
// /api/v1/library。ARCHITECTURE 早已移除 FetchRepoList，此处与之一致。

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	// 内嵌 IANA 时区数据库。
	//
	// 该站返回的时间戳不带时区，实际为美东时间（lua 注释头标注 EDT）。
	// 美东有夏令时，夏季 UTC-4、冬季 UTC-5，写死固定偏移会在换季后错 1 小时。
	// Windows 无系统 zoneinfo，故必须内嵌，代价约 450KB。
	_ "time/tzdata"
)

const (
	// msiteSourceName 是 M 站在源列表与 GameRecord.Source 中的显示名称。
	msiteSourceName = "Hubcap Manifest"

	// msiteBaseURL 是 M 站 API 的基址。
	//
	// NOTE: 该站历史上更换过域名，旧域名在国内不可访问。故此值存于配置
	// 而非硬编码到请求逻辑中，域名再变时用户可自行修改而无需等新版本。
	msiteBaseURL = "https://hubcapmanifest.com"

	// msiteStatusPath 是收录检测端点，不消耗每日额度。
	msiteStatusPath = "/api/v1/status/"

	// msiteManifestPath 是清单包下载端点，消耗 1 次每日额度。
	msiteManifestPath = "/api/v1/manifest/"

	// msiteStatsPath 是账户额度查询端点，不消耗每日额度。
	msiteStatsPath = "/api/v1/user/stats"

	// msiteExpiryWarnDays 是凭据到期提醒的提前天数。
	//
	// 取 3 天：该站凭据默认有效期仅 7 天，且 30 天以内的延期为自动批准。
	// 提前太久会变成常驻噪音，太晚则用户来不及处理。
	msiteExpiryWarnDays = 3
)

// MSiteStats 是 M 站账户的额度与凭据状态，供设置页展示。
//
// 字段说明：
//   - Username:        账户名
//   - DailyUsage:      今日已用次数
//   - DailyLimit:      每日上限
//   - CanMakeRequests: 服务端判定的当前可否请求，额度耗尽时为 false
//   - ExpiresAt:       凭据到期时刻，RFC 3339 字符串。无法解析时为空
//   - ExpiringSoon:    是否即将到期（不足 msiteExpiryWarnDays 天）
type MSiteStats struct {
	Username        string `json:"username"`
	DailyUsage      int    `json:"dailyUsage"`
	DailyLimit      int    `json:"dailyLimit"`
	CanMakeRequests bool   `json:"canMakeRequests"`
	ExpiresAt       string `json:"expiresAt"`
	ExpiringSoon    bool   `json:"expiringSoon"`
}

// msiteStatusResponse 对应 /api/v1/status/{app_id} 的返回结构。
type msiteStatusResponse struct {
	AppID              string  `json:"app_id"`
	GameName           string  `json:"game_name"`
	Status             string  `json:"status"`
	ManifestFileExists bool    `json:"manifest_file_exists"`
	FileAgeDays        float64 `json:"file_age_days"`
	NeedsUpdate        bool    `json:"needs_update"`
	UpdateReason       string  `json:"update_reason"`
}

// msiteStatsResponse 对应 /api/v1/user/stats 的返回结构。
type msiteStatsResponse struct {
	Username        string `json:"username"`
	DailyUsage      int    `json:"daily_usage"`
	DailyLimit      int    `json:"daily_limit"`
	CanMakeRequests bool   `json:"can_make_requests"`
	APIKeyExpiresAt string `json:"api_key_expires_at"`
}

// msiteErrorResponse 对应该站错误响应的结构。
//
// 401 的 detail 文案可区分「未提供凭据」与「凭据失效或过期」，
// 两者需要给用户的引导完全不同，故必须读出来而非只看状态码。
type msiteErrorResponse struct {
	Detail string `json:"detail"`
}

// msiteRequest 构造一个带 Bearer 凭据的 GET 请求。
//
// 参数：
//   - baseURL: API 基址，尾部斜杠会被去除
//   - path:    端点路径，须以斜杠开头
//   - token:   API 凭据
//
// NOTE: 凭据只经请求头传递，绝不作为查询参数——查询参数会出现在服务端
// 访问日志与各级代理日志中。该站文档虽也支持 api_key 查询参数，但那是
// 为 curl 便利提供的，不适合程序使用。
func msiteRequest(baseURL, path, token string) (*http.Request, error) {
	url := strings.TrimSuffix(baseURL, "/") + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}

// msiteAuthError 依据响应状态码与正文构造面向用户的错误。
//
// 该站的 401 分两种情形，给用户的指引完全不同：
//   - "API key required"           → 尚未配置，应引导去填写
//   - "API key not found or expired" → 已失效，应引导去续期或重新生成
//
// 429 表示额度耗尽。此时必须说清「今日额度已用尽、UTC 零点重置」，
// 而非笼统地报下载失败——后者会让用户以为工具坏了而不是额度问题。
func msiteAuthError(statusCode int, body []byte) error {
	detail := ""
	var e msiteErrorResponse
	if json.Unmarshal(body, &e) == nil {
		detail = e.Detail
	}

	switch statusCode {
	case http.StatusUnauthorized:
		if strings.Contains(detail, "required") {
			return fmt.Errorf("尚未配置 %s 的 API 凭据", msiteSourceName)
		}
		return fmt.Errorf("%s 的 API 凭据已失效或过期，需在其网站续期或重新生成", msiteSourceName)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s 今日额度已用尽，将于 UTC 零点重置", msiteSourceName)
	case http.StatusNotFound:
		return fmt.Errorf("%s 未收录该游戏", msiteSourceName)
	}

	if detail != "" {
		return fmt.Errorf("%s 返回 %d: %s", msiteSourceName, statusCode, detail)
	}
	return fmt.Errorf("%s 返回状态码 %d", msiteSourceName, statusCode)
}

// msiteStatus 查询 M 站是否收录指定 AppID。
//
// 走免额度端点，可安全地在每次进入详情页时调用。
//
// 返回值：
//   - *msiteStatusResponse: 收录信息
//   - error:                未收录、凭据问题或网络失败时返回
func (r *RepoClient) msiteStatus(src RepoSource, appID string) (*msiteStatusResponse, error) {
	req, err := msiteRequest(src.Repo, msiteStatusPath+appID, src.Token)
	if err != nil {
		return nil, err
	}

	resp, err := r.lookupHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", msiteSourceName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, msiteAuthError(resp.StatusCode, body)
	}

	var status msiteStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("解析 %s 的响应失败: %w", msiteSourceName, err)
	}

	// status 为 200 但清单不存在的情形确实存在（服务端知道这个游戏，
	// 但尚未生成清单），此时不能算作收录。
	if !status.ManifestFileExists || status.Status != "available" {
		return nil, fmt.Errorf("%s 尚未生成该游戏的清单", msiteSourceName)
	}
	return &status, nil
}

// MSiteAccountStats 查询 M 站账户的额度与凭据状态。
//
// 走免额度端点。凭据未配置时返回 nil 而不报错——「没配」是正常状态，
// 不该在界面上表现为错误。
//
// 返回值：
//   - *MSiteStats: 账户状态。凭据未配置时为 nil
//   - error:       凭据无效或网络失败时返回
func (r *RepoClient) MSiteAccountStats() (*MSiteStats, error) {
	src, ok := r.findMSiteSource()
	if !ok || src.Token == "" {
		return nil, nil
	}

	req, err := msiteRequest(src.Repo, msiteStatsPath, src.Token)
	if err != nil {
		return nil, err
	}

	resp, err := r.lookupHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 %s 失败: %w", msiteSourceName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, msiteAuthError(resp.StatusCode, body)
	}

	var raw msiteStatsResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析 %s 的响应失败: %w", msiteSourceName, err)
	}

	stats := &MSiteStats{
		Username:        raw.Username,
		DailyUsage:      raw.DailyUsage,
		DailyLimit:      raw.DailyLimit,
		CanMakeRequests: raw.CanMakeRequests,
	}

	// 到期时刻的格式为不带时区的 ISO 8601（如 2026-08-03T14:58:19.528769）。
	// 解析失败不影响其余字段可用，故只是留空而非返回错误。
	if t, err := parseMSiteTime(raw.APIKeyExpiresAt); err == nil {
		stats.ExpiresAt = t.Format(time.RFC3339)
		stats.ExpiringSoon = time.Until(t) < msiteExpiryWarnDays*24*time.Hour
	}
	return stats, nil
}

// msiteLocation 是该站服务端所在的时区。
//
// 该站返回的时间戳不带时区标识，实测其 lua 注释头写作 EDT，据凭据生成
// 时刻反推亦与美东时间吻合。故按 America/New_York 解析。
//
// 不写死 UTC-4 或 UTC-5：美东行夏令时，夏季为 EDT(UTC-4)、冬季为 EST(UTC-5)，
// 任一固定偏移都会在换季后产生 1 小时误差。用 IANA 时区名可自动处理。
//
// 加载失败时退回 UTC。这只会让到期时刻偏早数小时，提醒略微提前，
// 不至于让整个额度面板不可用。
var msiteLocation = func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}()

// parseMSiteTime 解析该站返回的时间字符串。
//
// 实测格式为 2026-08-03T14:58:19.528769，无时区标识也无 Z 后缀，
// 故不能直接用 time.RFC3339 解析。按美东时间处理，见 msiteLocation。
//
// 返回的 time.Time 带正确的时区信息，调用方可直接 Format 或与 time.Now
// 比较，无需再做偏移换算。
func parseMSiteTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("时间字符串为空")
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		time.RFC3339,
	} {
		if t, err := time.ParseInLocation(layout, s, msiteLocation); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %q", s)
}

// findMSiteSource 从配置中找出 M 站源。
//
// 返回值：
//   - RepoSource: 找到的源
//   - bool:       是否找到且已启用
func (r *RepoClient) findMSiteSource() (RepoSource, bool) {
	var all []RepoSource
	if r.config != nil {
		all = r.config.Get().RepoSources
	}
	if len(all) == 0 {
		all = defaultRepoSources()
	}
	for _, src := range all {
		if src.Kind == KindAPIZip && src.Enabled {
			return src, true
		}
	}
	return RepoSource{}, false
}
