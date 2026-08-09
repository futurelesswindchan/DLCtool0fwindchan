// repo_package.go
//
// 本文件负责把 MAU 形态的清单包解析为 GamePackage，与 lua_parser.go 并列
// 为两条独立的解析入口。
//
// 为何需要第二条入口：
//
//	M 站（hubcapmanifest）提供的包内含现成的 .lua 脚本，交给 Lua VM 执行
//	即可取得全部数据。而 MAU 系仓库的包里根本没有 .lua，只有：
//
//	    Key.vdf       depot 解密密钥
//	    config.json   主 AppID、depot 列表、DLC 列表
//	    *.manifest    文件名形如 <depotID>_<manifestID>.manifest
//	    appinfo.vdf   Steam 应用元信息（本工具不使用）
//
//	所需信息一项不缺，只是分散在三处、且需要自行拼装。故本文件做的事是
//	「用结构化数据直接构建 GamePackage」，而非「先生成 .lua 再解析」——
//	绕一圈生成中间脚本只会多一层出错的机会。
//
// 与 Lua 路径的一处实质差异：
//
//	MAU 的 config.json 用 packagedlcs 显式标出「带独立 Depot 的 DLC」，
//	比从 Lua 注释里启发式推断可靠得多。这类 DLC 正是 5.1 格式契约中
//	必须写三行、且取消勾选需要模态框警告的那一类。
//
// NOTE: 本解析器不产出 LuaContent（无源脚本可留），该字段留空。
// 部署时的脚本由 deployer_ost.go 依据 GamePackage 现场生成，不依赖它。

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	// mauConfigFileName 是 MAU 包内描述 AppID 与 DLC 构成的文件名。
	mauConfigFileName = "config.json"

	// mauKeyFileName 是 MAU 包内存放 depot 解密密钥的文件名。
	//
	// NOTE: 实测两个源的命名不一致——Auiowu/ManifestAutoUpdate 用 Key.vdf，
	// Satisl/MAU 用 config.vdf。故匹配时只认扩展名与内容结构，不认文件名，
	// 本常量仅用于日志描述。
	mauKeyFileName = "Key.vdf"

	// manifestFileExt 是 manifest 文件的扩展名。
	manifestFileExt = ".manifest"
)

// mauConfig 对应 MAU 包内 config.json 的结构。
//
// 实测样本（AppID 1364780）：
//
//	{"appId": 1364780, "depots": [1364781],
//	 "dlcs": [2224460, 2224461, ...],
//	 "packagedlcs": [1792750, 1792751]}
//
// 字段说明：
//   - AppID:       主游戏 AppID
//   - Depots:      归属主游戏的 depot ID 列表
//   - DLCs:        无独立 Depot 的 DLC，内容随本体下载
//   - PackageDLCs: 带独立 Depot 的 DLC，需单独下载内容
type mauConfig struct {
	AppID       int64   `json:"appId"`
	Depots      []int64 `json:"depots"`
	DLCs        []int64 `json:"dlcs"`
	PackageDLCs []int64 `json:"packagedlcs"`
}

// manifestNamePattern 从 manifest 文件名中提取 depot ID 与 manifest ID。
//
// 文件名形如 1364781_7340441221814650125.manifest，下划线前为 depot ID，
// 后为 manifest ID。两者都是纯数字且长度不定，故不能按固定宽度切分。
var manifestNamePattern = regexp.MustCompile(`^(\d+)_(\d+)` + regexp.QuoteMeta(manifestFileExt) + `$`)

// vdfKeyPattern 从 VDF 文本中提取 depot ID 与其解密密钥。
//
// 匹配形如以下的片段，允许任意空白与换行：
//
//	"1364781"
//	{
//	    "DecryptionKey"    "cfec3971..."
//	}
//
// 用正则而非引入 VDF 解析库：本工具只需从这一种固定形态中取一对值，
// 而 ⑨ 阶段刚为了减小依赖面移除了 andygrunwald/vdf。为读三行文本
// 再把它请回来并不值得。
//
// XXX: 此正则依赖 depot 块内紧跟 DecryptionKey 的结构。若上游改变
// VDF 的字段顺序或在块内插入其他键，匹配会失败——表现为密钥全部为空，
// 进而在部署时被 deployer 记为「缺少主游戏密钥」警告。
var vdfKeyPattern = regexp.MustCompile(
	`"(\d+)"\s*\{\s*"[Dd]ecryptionKey"\s*"([0-9a-fA-F]+)"`)

// mauSources 汇总从 MAU 包中收集到的原始素材。
//
// 分成「收集」与「组装」两步而非边扫边建：manifest 与密钥的出现顺序
// 不确定，而组装 DLC 时需要同时查询两者。
type mauSources struct {
	config    *mauConfig
	keys      map[string]string // depot ID → 解密密钥
	manifests map[string]string // depot ID → manifest ID
	sizes     map[string]int64  // depot ID → manifest 文件体积
	files     []string          // manifest 文件的完整路径
}

// parseMAUPackage 将已解压的 MAU 清单包目录解析为 GamePackage。
//
// 参数：
//   - dir: 解压后的目录。会递归遍历，因为不同源的包内层级不一致
//     （Auiowu 的包内容在单层子目录下，Satisl 的还额外嵌了一份项目源码）
//
// 返回值：
//   - *GamePackage: 组装完成的数据包。GameName 留空，由调用方用商店元数据回填
//   - []string: 尚缺密钥或 manifest 的独立 Depot DLC 的 AppID 列表。
//     调用方应对这些 ID 各拉一次自己的分支补齐，理由见 enrichPackageDLCs
//   - error: 缺少 config.json，或其中的 appId 非法时返回
//
// NOTE: 缺少密钥不视为错误。部分游戏（本体免费或无加密内容）确实没有
// depot 密钥，此时仍应产出可用的 GamePackage——DLC 注册本身不需要密钥。
func parseMAUPackage(dir string) (*GamePackage, []string, error) {
	src, err := collectMAUSources(dir)
	if err != nil {
		return nil, nil, err
	}

	mainAppID := strconv.FormatInt(src.config.AppID, 10)
	gp := &GamePackage{
		MainAppID:     mainAppID,
		Depots:        []DepotInfo{},
		DLCs:          []DLCInfo{},
		ManifestFiles: src.files,
	}

	// 主游戏自身的密钥。
	//
	// MAU 的 Key.vdf 里只有 depot 条目，主 AppID 通常不在其中——主游戏的
	// 内容实际存放于它的 depot（如 1364780 的内容在 depot 1364781）。
	// 因此这里先按主 AppID 查一次，查不到再退用首个 depot 的密钥：
	// 生成脚本要求主游戏那行必须带密钥，否则已装本体的游戏会崩。
	gp.MainKey = src.keys[mainAppID]

	dlcSet := make(map[string]bool, len(src.config.DLCs)+len(src.config.PackageDLCs))
	for _, id := range src.config.PackageDLCs {
		dlcSet[strconv.FormatInt(id, 10)] = true
	}
	for _, id := range src.config.DLCs {
		dlcSet[strconv.FormatInt(id, 10)] = true
	}

	// Depots：跳过归属 DLC 的条目，与 deployer 的输出规则保持一致。
	// 此处不跳的话，DLC 自有 depot 会同时出现在两个列表里，导致用户
	// 取消勾选后密钥仍被写出。
	for _, id := range src.config.Depots {
		depotID := strconv.FormatInt(id, 10)
		if dlcSet[depotID] {
			continue
		}
		gp.Depots = append(gp.Depots, DepotInfo{
			DepotID:       depotID,
			DecryptionKey: src.keys[depotID],
			ManifestID:    src.manifests[depotID],
			FileSize:      src.sizes[depotID],
		})
	}

	if gp.MainKey == "" {
		gp.MainKey = fallbackMainKey(gp.Depots, src.keys, src.config.Depots)
	}

	// DLC：packagedlcs 先行，它们带独立 Depot 需要输出三行；
	// 普通 dlcs 只需单行注册。
	//
	// 实测 MAU 为每个独立 Depot 的 DLC 单独开了一个以其 AppID 命名的分支，
	// 主游戏分支内并不含这些 DLC 的密钥与 manifest。故此处先按主包内已有的
	// 数据填充，缺失者记入 pending 交由调用方补齐。
	var pending []string
	for _, id := range src.config.PackageDLCs {
		appID := strconv.FormatInt(id, 10)
		key := src.keys[appID]
		manifestID := src.manifests[appID]
		if key == "" || manifestID == "" {
			pending = append(pending, appID)
		}
		gp.DLCs = append(gp.DLCs, DLCInfo{
			AppID:         appID,
			Name:          "DLC " + appID,
			HasKey:        key != "",
			DecryptionKey: key,
			ManifestID:    manifestID,
			FileSize:      src.sizes[appID],
		})
	}
	for _, id := range src.config.DLCs {
		appID := strconv.FormatInt(id, 10)
		gp.DLCs = append(gp.DLCs, DLCInfo{
			AppID: appID,
			Name:  "DLC " + appID,
		})
	}

	if len(pending) == 0 {
		pending = []string{}
	}
	return gp, pending, nil
}

// fallbackMainKey 在主 AppID 无密钥时挑一个 depot 密钥顶替。
//
// 取 config.json 中 depots 列表的首个可用密钥，而非遍历 map——
// map 迭代顺序随机，会让同一个包每次解析产出不同的脚本。
func fallbackMainKey(depots []DepotInfo, keys map[string]string, order []int64) string {
	for _, id := range order {
		if k := keys[strconv.FormatInt(id, 10)]; k != "" {
			return k
		}
	}
	for i := range depots {
		if depots[i].DecryptionKey != "" {
			return depots[i].DecryptionKey
		}
	}
	return ""
}

// readDepotCredentials 从已解压的 DLC 分支目录中取出指定 depot 的凭据。
//
// 用于补齐带独立 Depot 的 DLC——这类 DLC 的 AppID 与其 DepotID 相同，
// 故直接按 appID 查即可。
//
// 参数：
//   - dir:   已解压的目录
//   - appID: 目标 DLC 的 AppID，同时也是其 DepotID
//
// 返回值：
//   - string: 解密密钥
//   - string: manifest ID
//   - int64:  manifest 文件体积
//   - error:  目录中不含该 depot 的密钥时返回
func readDepotCredentials(dir string, appID string) (string, string, int64, error) {
	keys := map[string]string{}
	manifestID := ""
	var size int64

	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(info.Name())) {
		case ".vdf":
			mergeVDFKeys(path, keys)
		case manifestFileExt:
			m := manifestNamePattern.FindStringSubmatch(info.Name())
			if m != nil && m[1] == appID {
				manifestID = m[2]
				size = info.Size()
			}
		}
		return nil
	})
	if walkErr != nil {
		return "", "", 0, fmt.Errorf("遍历 DLC 分支目录失败: %w", walkErr)
	}

	key := keys[appID]
	if key == "" {
		return "", "", 0, fmt.Errorf("DLC %s 的分支中未找到解密密钥", appID)
	}
	return key, manifestID, size, nil
}

// ============================================================
// 解压
// ============================================================

// unzipMAUPackage 解压 MAU 形态的清单包，提取 json / vdf / manifest 三类文件。
//
// 与 unzipFile 的差异只在允许的扩展名：那个函数服务 Lua 形态的包，
// 只放行 .lua 与 .manifest，遇到 MAU 包会直接报「未找到 .lua」。
// 安全校验（路径遍历防护、隐藏文件、目录条目）沿用同一套规则。
//
// 参数：
//   - zipPath: 压缩包完整路径
//   - destDir: 解压目标目录，应为临时目录
//
// 返回值：
//   - int:   成功提取的文件数
//   - error: 压缩包无法打开或写出失败时返回
//
// NOTE: 同名文件只保留首个。包内目录结构被抹平（统一用 filepath.Base），
// 而 Satisl 的包嵌了一份项目源码，其中可能存在同名的 config.json——
// 顶层条目在 zip 中先于嵌套条目出现，故「首个优先」正好取到本包的那份。
func unzipMAUPackage(zipPath string, destDir string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, fmt.Errorf("无法打开压缩包: %w", err)
	}
	defer func() { _ = r.Close() }()

	extracted := 0
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		fileName := filepath.Base(f.Name)
		if fileName == "" || fileName == "." || strings.HasPrefix(fileName, ".") {
			continue
		}
		if strings.Contains(f.Name, "..") {
			continue
		}

		switch strings.ToLower(filepath.Ext(fileName)) {
		case ".json", ".vdf", manifestFileExt:
		default:
			continue
		}

		destPath := filepath.Join(destDir, fileName)
		if !withinDir(destPath, destDir) {
			continue
		}
		if fileExists(destPath) {
			continue
		}

		if err := extractZipEntry(f, destPath); err != nil {
			return extracted, err
		}
		extracted++
	}

	if extracted == 0 {
		return 0, fmt.Errorf("压缩包中没有可识别的清单文件")
	}
	return extracted, nil
}

// withinDir 校验目标路径解析后仍位于指定目录内。
//
// 清单包来源不可控，即使已用 filepath.Base 抹平路径，仍显式再验一次——
// 这类防护的代价极低，而漏掉一次的后果是任意文件写入。
func withinDir(destPath string, dir string) bool {
	absDest, err := filepath.Abs(destPath)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absDest, absDir+string(filepath.Separator))
}

// ============================================================
// 素材收集
// ============================================================

// collectMAUSources 递归遍历解压目录，收集 config.json、密钥与 manifest。
//
// 递归而非只扫顶层：Auiowu 的包把内容放在单层子目录下，Satisl 的包还额外
// 嵌套了一份项目源码目录。按文件名与内容特征识别比假定层级稳妥。
//
// 返回值：
//   - *mauSources: 收集结果，config 字段保证非 nil
//   - error:       未找到 config.json，或其内容无法解析、appId 非法时返回
func collectMAUSources(dir string) (*mauSources, error) {
	src := &mauSources{
		keys:      map[string]string{},
		manifests: map[string]string{},
		sizes:     map[string]int64{},
		files:     []string{},
	}

	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}

		base := info.Name()
		switch strings.ToLower(filepath.Ext(base)) {
		case ".json":
			// 只认 config.json：Satisl 的包内嵌的项目源码里也可能有其他 json。
			if strings.EqualFold(base, mauConfigFileName) && src.config == nil {
				if cfg := readMAUConfig(path); cfg != nil {
					src.config = cfg
				}
			}

		case ".vdf":
			// 不按文件名匹配：两个源分别叫 Key.vdf 与 config.vdf。
			// appinfo.vdf 也会走到这里，但它不含 DecryptionKey 结构，
			// 正则自然匹配不到，无需特意排除。
			mergeVDFKeys(path, src.keys)

		case manifestFileExt:
			m := manifestNamePattern.FindStringSubmatch(base)
			if m == nil {
				return nil
			}
			depotID, manifestID := m[1], m[2]
			src.manifests[depotID] = manifestID
			// manifest 文件自身体积顶替 setManifestid 的第三参数。
			//
			// NOTE: 这与 Lua 路径的语义不同——Lua 里该值是 depot 的内容
			// 总大小，此处是 manifest 文件大小，两者差几个数量级。该参数
			// 仅用于界面展示，OST 不依赖它做校验，故可以接受；但界面上
			// 不应把它当作「下载体积」展示给用户。
			src.sizes[depotID] = info.Size()
			src.files = append(src.files, path)
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("遍历清单包目录失败: %w", walkErr)
	}

	if src.config == nil {
		return nil, fmt.Errorf("清单包中未找到有效的 %s，无法识别包格式", mauConfigFileName)
	}
	if src.config.AppID <= 0 {
		return nil, fmt.Errorf("%s 中的 appId 非法: %d", mauConfigFileName, src.config.AppID)
	}
	return src, nil
}

// readMAUConfig 读取并解析 config.json。
//
// 解析失败返回 nil 而非错误：遍历过程中遇到的可能是同名但无关的文件，
// 由调用方在遍历结束后统一判断「一个都没找到」。
func readMAUConfig(path string) *mauConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg mauConfig
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.AppID <= 0 {
		return nil
	}
	return &cfg
}

// mergeVDFKeys 从 VDF 文件中提取所有 depot 密钥并合并到 dst。
//
// 已存在的 depot ID 不覆盖：先遇到的文件优先。包内若有多份 VDF，
// 顶层的那份通常才是本包的密钥文件，嵌套目录里的属于其他内容。
func mergeVDFKeys(path string, dst map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, m := range vdfKeyPattern.FindAllStringSubmatch(string(data), -1) {
		if _, exists := dst[m[1]]; !exists {
			dst[m[1]] = m[2]
		}
	}
}
