// app_diag_mask.go
//
// 诊断包的内容生成：配置脱敏、日志选取、环境信息、zip 写入。
//
// 脱敏采取「白名单式复制」而非「黑名单式擦除」：先构造一个只含已知
// 安全字段的新结构，再逐项填入，而不是拷贝整个配置再删敏感项。
// 二者的差别在日后新增字段时显现——黑名单方式下，新增的敏感字段会
// 默认泄露，且没有任何编译期或运行期信号提醒开发者补规则。

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// maskedRepoSource 是 RepoSource 的脱敏投影。
//
// 有意不复用 RepoSource 并置空 Token：那样做的话，日后 RepoSource
// 新增敏感字段（如 Cookie、RefreshToken）会自动出现在诊断包里。
// 独立结构使「哪些字段可以外流」成为一处显式声明。
type maskedRepoSource struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Repo    string `json:"repo"`
	Enabled bool   `json:"enabled"`

	// TokenState 只描述凭据的存在性，不含任何原值片段。
	//
	// 排障需要区分的仅三种情形：没填、已填、已填但格式可疑。
	// 连 maskSecret 的前 8 位也不给——日志里出现前 8 位可以接受
	// （用于比对是否同一密钥），但诊断包的流传范围更广，且此处
	// 没有任何需要比对密钥的场景。
	TokenState string `json:"tokenState"`
}

// maskedConfig 是 AppConfig 的脱敏投影。
type maskedConfig struct {
	SteamPath   string             `json:"steamPath"`
	Theme       string             `json:"theme"`
	LastZipDir  string             `json:"lastZipDir"`
	AutoDetect  bool               `json:"autoDetect"`
	RepoSources []maskedRepoSource `json:"repoSources"`

	// Note 写在文件里而非仅靠文件名，因为用户很可能只把 json 内容
	// 粘贴到群里，此时文件名的 .masked 提示就丢失了。
	Note string `json:"_说明"`
}

// describeTokenState 把凭据映射为不含原值的状态描述。
func describeTokenState(token string) (state string, present bool) {
	switch {
	case token == "":
		return "未填写", false
	case len(token) < 16:
		return fmt.Sprintf("已填写（仅 %d 位，长度可疑）", len(token)), true
	default:
		return fmt.Sprintf("已填写（%d 位，内容已脱敏）", len(token)), true
	}
}

// writeMaskedConfig 把脱敏后的配置副本写入 zip。
//
// 返回值：
//   - bool:  是否存在被脱敏的凭据，供前端决定提示措辞
//   - error: 序列化或写入失败
//
// 不读磁盘上的 config.json 而取内存中的配置：磁盘文件可能含用户手工
// 添加的未知字段，原样读入再序列化等于把未知内容一并带出。经内存结构
// 走一遭，等于用 Go 的类型定义做了一次强制过滤。
func (a *App) writeMaskedConfig(zw *zip.Writer) (bool, error) {
	cfg := a.config.Get()

	out := maskedConfig{
		SteamPath:  cfg.SteamPath,
		Theme:      cfg.Theme,
		LastZipDir: cfg.LastZipDir,
		AutoDetect: cfg.AutoDetect,
		Note: "本文件是配置的脱敏副本，API 凭据已被移除，可安全提供给开发者。" +
			"原始 config.json 含你自己的凭据，请勿直接分享。",
		RepoSources: make([]maskedRepoSource, 0, len(cfg.RepoSources)),
	}

	anyMasked := false
	for _, s := range cfg.RepoSources {
		state, present := describeTokenState(s.Token)
		if present {
			anyMasked = true
		}
		out.RepoSources = append(out.RepoSources, maskedRepoSource{
			Name:       s.Name,
			Kind:       string(s.Kind),
			Repo:       s.Repo,
			Enabled:    s.Enabled,
			TokenState: state,
		})
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return false, fmt.Errorf("配置序列化失败：%w", err)
	}

	if err := writeZipEntry(zw, "config.masked.json", string(data)); err != nil {
		return false, err
	}
	return anyMasked, nil
}

// writeRecentLogs 把最近的若干个日志文件写入 zip 的 logs/ 下。
//
// 返回值：
//   - int: 成功写入的日志数量
//
// 单个文件读取失败只跳过：诊断包宁可少一个日志也不该整体失败，
// 而日志正被写入时的读取冲突在 Windows 上并非罕见。
func (a *App) writeRecentLogs(zw *zip.Writer, logDir string) int {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		a.logger.Warn("读取日志目录失败 %s: %v", logDir, err)
		return 0
	}

	type logFile struct {
		name string
		mod  time.Time
	}
	files := make([]logFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, logFile{name: e.Name(), mod: info.ModTime()})
	}

	// 按修改时间降序，取最新的几个。不按文件名排序——轮转命名规则
	// 一旦调整，字典序与时间序就会脱钩。
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	if len(files) > diagLogCount {
		files = files[:diagLogCount]
	}

	written := 0
	for _, lf := range files {
		data, err := os.ReadFile(filepath.Join(logDir, lf.name))
		if err != nil {
			a.logger.Warn("读取日志失败 %s: %v", lf.name, err)
			continue
		}
		if err := writeZipEntry(zw, "logs/"+lf.name, string(data)); err != nil {
			a.logger.Warn("写入日志到诊断包失败 %s: %v", lf.name, err)
			continue
		}
		written++
	}
	return written
}

// environmentReport 生成排障所需的环境上下文文本。
//
// 收录项的选择依据是「用户报障时最常被追问的信息」：版本号（对齐是哪
// 一版）、系统与架构（Win10/11 行为差异）、数据目录实际落点（三级选址
// 的结果无法从外部推断）、注入器就绪状态。
//
// 不含用户名。数据目录路径在回退到主目录时会含用户名，这一项确实必要
// 且难以规避，但除此之外不主动收集任何身份信息。
func (a *App) environmentReport(dataDir string, logCount int) string {
	var b strings.Builder

	b.WriteString("风兔盒 kazeusa 诊断信息\n")
	b.WriteString("========================================\n\n")

	bi := currentBuildInfo()

	// 构建标识置于最前：这是排障时第一个要确认的信息，
	// 也是唯一无法通过追问用户获得的信息（用户不知道自己的包是哪次构建）。
	fmt.Fprintf(&b, "构建标识     : %s\n", bi.Label)
	fmt.Fprintf(&b, "生成时间     : %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "程序版本     : %s\n", bi.Version)
	fmt.Fprintf(&b, "提交哈希     : %s\n", bi.Commit)
	fmt.Fprintf(&b, "构建时刻     : %s\n", bi.BuiltAt)
	if bi.Dirty {
		b.WriteString("⚠ 工作树有未提交改动，此包对应的代码不在仓库中\n")
	}
	fmt.Fprintf(&b, "构建模式     : %s\n", buildModeLabel())
	fmt.Fprintf(&b, "操作系统     : %s / %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&b, "Go 运行时    : %s\n", runtime.Version())
	fmt.Fprintf(&b, "CPU 核心     : %d\n\n", runtime.NumCPU())

	fmt.Fprintf(&b, "数据目录     : %s\n", dataDir)
	fmt.Fprintf(&b, "日志文件     : %s\n", fallbackText(a.logger.Path(), "（日志未启用）"))
	fmt.Fprintf(&b, "收录日志数   : %d\n\n", logCount)

	cfg := a.config.Get()
	fmt.Fprintf(&b, "Steam 路径   : %s\n", fallbackText(cfg.SteamPath, "（未设置）"))
	fmt.Fprintf(&b, "部署目录     : %s\n", fallbackText(a.GetDeployDir(), "（不可用）"))
	fmt.Fprintf(&b, "启动时自动检测: %v\n", cfg.AutoDetect)
	fmt.Fprintf(&b, "清单源总数   : %d（启用 %d）\n\n",
		len(cfg.RepoSources), countEnabledSources(cfg.RepoSources))

	b.WriteString("----------------------------------------\n")
	b.WriteString("本文件不含 API 凭据、安装历史与清单内容。\n")
	b.WriteString("如需补充信息，请在群内说明操作步骤与出现的提示原文。\n")

	return b.String()
}

// countEnabledSources 统计启用中的源数量。
func countEnabledSources(sources []RepoSource) int {
	n := 0
	for _, s := range sources {
		if s.Enabled {
			n++
		}
	}
	return n
}

// fallbackText 在字符串为空时返回占位文本。
//
// 用于环境报告：空白行会让阅读者误以为信息采集失败，
// 明确的「（未设置）」才能表达「确实是空的」。
func fallbackText(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// buildModeLabel 返回可读的构建模式标识。
func buildModeLabel() string {
	if isDevBuild {
		return "开发构建（wails dev）"
	}
	return "正式构建"
}

// writeZipEntry 向 zip 写入一个文本条目。
//
// 统一走此函数而非各处自行 Create：便于将来统一处理编码问题。
// 当前一律以 UTF-8 原样写入——记事本对无 BOM 的 UTF-8 已能正确识别，
// 而加 BOM 会让 json 解析器报错。
func writeZipEntry(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("创建压缩条目 %s 失败：%w", name, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		return fmt.Errorf("写入压缩条目 %s 失败：%w", name, err)
	}
	return nil
}
