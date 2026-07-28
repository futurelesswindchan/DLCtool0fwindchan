// config.go
//
// 本文件负责用户配置的持久化：读取、修改与原子落盘。
//
// 设计要点：
//   - 配置存放于数据目录下的 config.json，与 Steam 目录完全隔离，
//     避免 Steam 更新或注入器重装时被连带清除。数据目录默认为 exe 同级
//     的 .kazeusa/，选址规则见 appDataDir。
//   - 采用 JSON 而非 SQLite：数据量极小（一份配置 + 数十条历史），
//     且 SQLite 会引入 CGO 依赖，令 Wails 的交叉编译复杂化。
//   - 写入一律走「临时文件 + rename」，防止进程在写入中途崩溃时
//     留下半截 JSON 导致下次启动无法解析。
//   - 首次运行或文件损坏时回退到默认配置，绝不因配置问题阻断启动。

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RepoSource 表示一个在线清单仓库源。
//
// 三个内置源互为备份，任一收录目标 AppID 即可完成入库，
// 因此「多源」在本工具中是可用性冗余而非内容互补。
//
// 字段说明：
//   - Name:    源的显示名称，同时作为 GameRecord.Source 的取值
//   - Kind:    访问形态，决定采用哪套下载逻辑，取值见 RepoKind
//   - Repo:    仓库标识。Kind 为 KindGitHubBranch 时形如 "owner/name"；
//     为 KindZipTemplate 时是含 {app_id} 占位符的完整 URL；
//     为 KindAPIZip 时是 API 的基址
//   - Token:   API 凭据，仅 KindAPIZip 使用。留空表示该源不参与工作
//   - Enabled: 是否启用，禁用的源在查找与下载时都会被跳过
//
// NOTE: 此结构曾用 Type/URL/Mirror 三字段，镜像作为单个地址内联其中。
// 改为 Kind/Repo 是因为实际的镜像回退是一条四级链（见 repo_client.go 的
// downloadMirrors），塞不进一个字段，且回退链对所有 GitHub 源都相同，
// 逐源重复配置没有意义。
// NOTE: Token 是用户自愿提供的、代表其自身账户额度的凭据，性质等同于
// 本地导入——都是可选增强而非底层主逻辑。盒子不内置任何共享凭据：
// 分发的 exe 里的密钥必然被提取，且共享流量的特征与爬库无法区分。
type RepoSource struct {
	Name    string   `json:"name"`
	Kind    RepoKind `json:"kind"`
	Repo    string   `json:"repo"`
	Token   string   `json:"token,omitempty"`
	Enabled bool     `json:"enabled"`
}

// AppConfig 表示本工具的全部用户配置。
//
// 该结构体会被序列化到 config.json，同时通过 Wails 暴露给前端，
// 前端的设置面板直接读写这些字段。
//
// 字段说明：
//   - SteamPath:   Steam 安装目录。为空表示尚未识别，启动时会尝试从注册表自动获取
//   - Theme:       界面主题，取值 "dark" 或 "light"
//   - LastZipDir:  上次选择清单包的目录，用于让文件对话框记住位置
//   - RepoSources: 在线仓库源列表
//   - AutoDetect:  启动时是否自动检测注入器环境
type AppConfig struct {
	SteamPath  string       `json:"steamPath"`
	Theme      string       `json:"theme"`
	LastZipDir string       `json:"lastZipDir"`
	RepoSources []RepoSource `json:"repoSources"`
	AutoDetect bool         `json:"autoDetect"`
}

// defaultConfig 返回一份可直接使用的默认配置。
//
// 用于首次运行、配置文件缺失或解析失败时的回退。
// SteamPath 故意留空——调用方应在拿到默认配置后
// 尝试通过注册表自动识别，识别失败再由用户手动指定。
func defaultConfig() *AppConfig {
	return &AppConfig{
		SteamPath:   "",
		Theme:       "dark",
		LastZipDir:  "",
		RepoSources: []RepoSource{},
		AutoDetect:  true,
	}
}

// ConfigManager 管理配置的生命周期，是配置数据的唯一权威来源。
//
// 内部持有一份配置副本并以读写锁保护，前端的并发调用（Wails 每个
// 请求在独立 goroutine 中处理）不会造成数据竞争。
//
// 典型用法：
//
//	cm, err := NewConfigManager(logger)
//	cfg := cm.Get()                  // 读取快照
//	cfg.Theme = "light"
//	err = cm.Save(cfg)               // 整体覆盖并落盘
type ConfigManager struct {
	mu     sync.RWMutex
	cfg    *AppConfig
	path   string
	logger *Logger
}

// appDataDir 返回本工具的数据目录完整路径，并确保该目录已存在。
//
// 选址顺序：
//  1. 开发构建（wails dev，带 dev 构建标签）→ 用户主目录下的 .kazeusa/
//  2. 正式构建 → exe 所在目录下的 .kazeusa/
//  3. exe 目录不可写（装在 Program Files、只读介质等）→ 回退用户主目录
//
// 之所以默认跟随 exe：本工具定位为绿色软件，用户期望「拷走一个文件夹
// 即带走全部数据」。放在主目录则换机器时配置与历史全部丢失，而用户
// 通常不知道 ~/.kazeusa 的存在。
//
// 之所以用构建标签而非环境变量或路径特征判别开发模式：
//   - 路径特征（如目录名含 build）会误伤把程序放在同名目录下的正常用户
//   - 环境变量需人工配置，忘记设置便会把数据写进构建输出目录，
//     随下次清理一并消失
//   - 构建标签由构建方式决定，wails dev 必然携带，无从遗漏或误设
//
// 返回值：
//   - string: 数据目录路径，示例 D:\kazeusa\.kazeusa
//   - error:  两个候选位置均不可用时返回
//
// NOTE: 不实现从旧位置迁移数据。v1.4（时称 dlctool）不产生任何本地数据
// 文件，其操作直接作用于 Steam，故「迁移旧数据」并无对象。
func appDataDir() (string, error) {
	if isDevBuild {
		return homeDataDir()
	}

	if dir, err := exeDataDir(); err == nil {
		return dir, nil
	}

	// exe 目录不可写时必须回退，否则程序在只读位置将完全不可用。
	return homeDataDir()
}

// exeDataDir 返回 exe 同级的数据目录，并以实际写入验证其可用性。
//
// 仅靠 MkdirAll 成功不足以判定可写：目录可能已存在但拒绝写入文件
// （典型情形是 Program Files 下的 UAC 虚拟化）。故额外做一次探针写入。
func exeDataDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位程序自身路径: %w", err)
	}

	dir := filepath.Join(filepath.Dir(exePath), AppDataDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("无法在程序目录创建数据目录 %s: %w", dir, err)
	}

	probe := filepath.Join(dir, ".writable")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return "", fmt.Errorf("程序目录不可写 %s: %w", dir, err)
	}
	_ = os.Remove(probe)

	return dir, nil
}

// homeDataDir 返回用户主目录下的数据目录，作为 exe 目录不可写时的回退。
func homeDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户主目录: %w", err)
	}

	dir := filepath.Join(home, AppDataDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("无法创建数据目录 %s: %w", dir, err)
	}
	return dir, nil
}

// NewConfigManager 创建配置管理器并立即加载磁盘上的配置。
//
// 参数：
//   - logger: 日志记录器，可为 nil（此时静默运行，便于单元测试）
//
// 返回值：
//   - *ConfigManager: 已完成加载的管理器，任何情况下均非 nil
//   - error:          仅当数据目录无法创建时返回；配置文件本身的
//                     缺失或损坏会被降级处理为「使用默认配置」而不报错
//
// NOTE: 即使返回 error，也不应视为致命——调用方可选择用内存中的
// 默认配置继续运行，只是无法持久化。
func NewConfigManager(logger *Logger) (*ConfigManager, error) {
	cm := &ConfigManager{
		cfg:    defaultConfig(),
		logger: logger,
	}

	dir, err := appDataDir()
	if err != nil {
		cm.logf("警告：数据目录不可用，配置将无法持久化: %v", err)
		return cm, err
	}
	cm.path = filepath.Join(dir, ConfigFileName)

	cm.load()
	return cm, nil
}

// load 从磁盘读取配置到内存。
//
// 出错时保留已有的默认配置并记录日志，不向上传播错误：
// 配置读不出来不该让整个应用起不来。
func (cm *ConfigManager) load() {
	data, err := os.ReadFile(cm.path)
	if err != nil {
		if os.IsNotExist(err) {
			cm.logf("配置文件不存在，使用默认配置: %s", cm.path)
		} else {
			cm.logf("读取配置文件失败，使用默认配置: %v", err)
		}
		return
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// 文件存在但内容损坏（如上次写入被强杀打断、用户手动编辑出错）。
		// 保留损坏文件供排查，不覆盖，仅在内存中回退到默认值。
		cm.logf("配置文件解析失败，已回退默认配置（原文件保留待查）: %v", err)
		return
	}

	cm.normalize(&cfg)
	cm.cfg = &cfg
	cm.logf("配置加载成功: %s", cm.path)
}

// normalize 修补配置中的空值与非法取值，保证下游代码拿到的字段总是可用。
//
// JSON 反序列化时缺失的字段会是零值（空字符串、nil slice），
// 若不归一化，前端可能收到 null 而在遍历时报错。
func (cm *ConfigManager) normalize(cfg *AppConfig) {
	if cfg.Theme != "dark" && cfg.Theme != "light" {
		cfg.Theme = "dark"
	}
	// 空列表一律回填内置源。v2.0 不提供自定义源的界面入口，故「一个源都
	// 没有」只可能来自旧版配置或人工误删，此时保持空会让在线功能彻底失效，
	// 而用户从界面上无从修复。
	if len(cfg.RepoSources) == 0 {
		cfg.RepoSources = defaultRepoSources()
	}
}

// logf 在 logger 可用时记录一条信息级日志。
//
// 封装 nil 判断，避免每个调用点都写一遍守卫语句。
func (cm *ConfigManager) logf(format string, args ...any) {
	if cm.logger != nil {
		cm.logger.Info(format, args...)
	}
}

// ============================================================
// 公开 API
// ============================================================

// Get 返回当前配置的一份深拷贝快照。
//
// 返回拷贝而非内部指针，调用方可以随意修改返回值而不影响
// 管理器内部状态——只有显式调用 Save 才会生效。
// RepoSources 切片同样被复制，避免调用方通过切片元素间接改写内部数据。
func (cm *ConfigManager) Get() *AppConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	snapshot := *cm.cfg
	snapshot.RepoSources = make([]RepoSource, len(cm.cfg.RepoSources))
	copy(snapshot.RepoSources, cm.cfg.RepoSources)
	return &snapshot
}

// Save 用传入的配置整体覆盖现有配置并落盘。
//
// 参数：
//   - cfg: 完整的新配置。为 nil 时直接返回错误，不做任何改动
//
// 返回值：
//   - error: 落盘失败时返回。此时内存中的配置仍会被更新，
//     以保证本次运行期间用户的修改立即生效，只是重启后会丢失
//
// NOTE: 内存先行更新是有意的取舍——用户改了主题却因磁盘满而
// 看不到变化，体验比"改了但重启丢失"更糟。
func (cm *ConfigManager) Save(cfg *AppConfig) error {
	if cfg == nil {
		return fmt.Errorf("配置不能为空")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.normalize(cfg)
	cm.cfg = cfg

	return cm.persist()
}

// Update 以函数式方式局部修改配置并立即落盘。
//
// 相比"Get 改字段再 Save"的三步走，Update 在同一把锁内完成
// 读取-修改-写入，避免两个并发调用互相覆盖对方的改动。
//
// 参数：
//   - mutate: 修改函数，接收可直接改写的配置指针
//
// 返回值：
//   - error: mutate 为 nil 或落盘失败时返回
//
// 示例：
//
//	err := cm.Update(func(c *AppConfig) {
//	    c.LastZipDir = filepath.Dir(zipPath)
//	})
func (cm *ConfigManager) Update(mutate func(*AppConfig)) error {
	if mutate == nil {
		return fmt.Errorf("修改函数不能为空")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	mutate(cm.cfg)
	cm.normalize(cm.cfg)

	return cm.persist()
}

// Path 返回配置文件的完整路径。
//
// 主要供日志输出与前端的"打开配置目录"功能使用。
// 若数据目录初始化失败，返回空字符串。
func (cm *ConfigManager) Path() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.path
}

// persist 将内存中的配置序列化写入磁盘。
//
// 调用方必须已持有写锁。使用缩进格式输出，方便用户手动查看和编辑。
func (cm *ConfigManager) persist() error {
	if cm.path == "" {
		return fmt.Errorf("数据目录不可用，配置无法持久化")
	}

	data, err := json.MarshalIndent(cm.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := atomicWriteFile(cm.path, data); err != nil {
		cm.logf("配置写入失败: %v", err)
		return err
	}

	cm.logf("配置已保存: %s", cm.path)
	return nil
}