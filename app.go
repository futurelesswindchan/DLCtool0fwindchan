// app.go
//
// 本文件是前端 API 的编排层：所有暴露给 Vue 调用的方法都在这里，
// 但方法体内只做参数校验、模块调用与结果组装，不实现业务逻辑。
//
// 依赖的六个模块及其职责：
//   - ConfigManager   配置读写
//   - HistoryManager  安装历史
//   - Deployer        把清单脚本放到注入器监控目录（接口）
//   - Detector        检测注入器环境是否就绪（接口）
//   - Logger          日志
//   - lua_parser.go   清单脚本解析（函数式，无需实例）
//
// 关于 Steam 路径的单一来源：
//
//	App 不再持有 steamPath 字段。路径的唯一权威是 config.json，
//	需要时经 a.config.Get().SteamPath 读取。v1.4 曾在 App 与
//	配置中各存一份，导致用户改了设置而部分操作仍用旧路径。
//
// 关于 Deployer 的重建：
//
//	Deployer 在构造时固定 steamPath。用户修改 Steam 路径后，
//	必须调用 rebuildDeployer 重建实例，而非原地改字段——
//	这样部署器内部不存在可变状态，也就不存在竞态。
//	改路径是极低频操作，重建的开销可以忽略。

package main

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows/registry"
)

// App 是应用主结构体，通过 Wails 绑定到前端。
//
// 所有字段在 NewApp 中初始化完毕，除 ctx 与 deployer 外均不再变动。
// ctx 由 startup 回调注入；deployer 在 Steam 路径变更时被替换。
type App struct {
	ctx      context.Context
	logger   *Logger
	config   *ConfigManager
	history  *HistoryManager
	deployer Deployer
	detector Detector
	store    *StoreClient
	repo     *RepoClient
	packages *PackageStore
}

// NewApp 创建并初始化 App 实例。
//
// 初始化顺序有依赖关系：logger 最先（后续模块要用它记录问题），
// 然后是 config（deployer 需要其中的 steamPath），最后是其余模块。
//
// 配置或历史加载失败不会导致构造失败——它们各自会降级为默认值
// 并记录日志。用户宁可在功能受限的状态下打开应用，也不愿看到
// 一个因为读不到 config.json 而拒绝启动的工具。
func NewApp() *App {
	logger := NewLogger()

	config, err := NewConfigManager(logger)
	if err != nil {
		logger.Warn("配置管理器初始化异常，将使用默认配置: %v", err)
	}

	history, err := NewHistoryManager(logger)
	if err != nil {
		logger.Warn("历史管理器初始化异常，安装记录将无法保存: %v", err)
	}

	app := &App{
		logger:   logger,
		config:   config,
		history:  history,
		detector: NewOSTDetector(logger),
		store:    NewStoreClient(logger),
		repo:     NewRepoClient(config, logger),
		packages: NewPackageStore(logger),
	}
	app.rebuildDeployer()

	return app
}

// rebuildDeployer 依据配置中的当前 Steam 路径重建部署器实例。
//
// 必须在以下时机调用：
//   - App 构造时
//   - Steam 路径发生变更后（SetSteamPath / GetSteamPath / SaveConfig）
//
// 若忘记调用，部署器会继续往旧路径写文件，而界面上显示的是新路径——
// 这类不一致极难排查，故所有改动 steamPath 的方法都应在末尾调用本方法。
func (a *App) rebuildDeployer() {
	steamPath := ""
	if a.config != nil {
		steamPath = a.config.Get().SteamPath
	}
	a.deployer = NewOSTDeployer(steamPath, a.logger)
}

// steamPath 返回配置中记录的 Steam 安装路径。
//
// 所有需要 Steam 路径的方法都应经由此处读取，不要缓存返回值——
// 用户可能在任意时刻通过设置面板修改它。
func (a *App) steamPath() string {
	if a.config == nil {
		return ""
	}
	return a.config.Get().SteamPath
}

// ============================================================
// 生命周期
// ============================================================

// startup 是 Wails 的生命周期回调，在应用窗口创建后调用。
//
// 除保存运行时上下文外，还负责两项启动维护工作：
//  1. 清理进程被强制结束时遗留在 %TEMP% 下的解压目录
//  2. 若配置中尚无 Steam 路径，尝试从注册表自动识别
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.logger.Info("应用启动，日志位置: %s", a.logger.Path())

	// 24 小时门槛可避免误删另一个同时运行的实例正在使用的目录。
	if n := cleanStaleTempDirs(24 * time.Hour); n > 0 {
		a.logger.Info("已清理 %d 个遗留的临时解压目录", n)
	}

	if a.steamPath() == "" {
		if path, err := a.GetSteamPath(); err != nil {
			a.logger.Warn("Steam 路径自动识别失败，需用户手动指定: %v", err)
		} else {
			a.logger.Info("Steam 路径自动识别成功: %s", path)
		}
	}
}

// shutdown 是 Wails 的生命周期回调，在应用即将退出时调用。
//
// NOTE: 后续新增需要优雅关闭的资源（下载协程、缓存刷写等）
// 都应在此处登记，这是应用唯一的退出钩子。
func (a *App) shutdown(ctx context.Context) {
	a.logger.Info("应用退出")
	a.logger.Close()
}

// ============================================================
// 配置
// ============================================================

// GetConfig 返回当前的完整配置。
//
// 前端设置面板据此渲染各项设置的当前值。
func (a *App) GetConfig() *AppConfig {
	if a.config == nil {
		return defaultConfig()
	}
	return a.config.Get()
}

// SaveConfig 保存前端提交的完整配置。
//
// 参数：
//   - cfg: 完整配置对象，前端应基于 GetConfig 的返回值修改后回传
//
// 返回值：
//   - *OperationResult: 保存结果，Message 可直接展示给用户
//
// 若 SteamPath 发生变更，会重建部署器以保证后续写入落到新路径。
func (a *App) SaveConfig(cfg *AppConfig) *OperationResult {
	if a.config == nil {
		return failure("配置系统不可用，无法保存设置")
	}
	if cfg == nil {
		return failure("配置内容为空")
	}

	oldPath := a.steamPath()

	if err := a.config.Save(cfg); err != nil {
		a.logger.Error("保存配置失败: %v", err)
		return failure(fmt.Sprintf("保存设置失败：%v", err))
	}

	if cfg.SteamPath != oldPath {
		a.rebuildDeployer()
		a.logger.Info("Steam 路径已变更，部署器已重建: %s", cfg.SteamPath)
	}

	return success("设置已保存")
}

// GetSteamPath 从 Windows 注册表自动识别 Steam 安装路径并写入配置。
//
// 返回值：
//   - string: 识别到的 Steam 安装目录
//   - error:  注册表访问失败或值不存在时返回
//
// 局限性：仅能识别注册表记录的主安装路径，无法覆盖手动迁移的场景。
// 这类情况需用户通过 SetSteamPath 指定。
func (a *App) GetSteamPath() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, SteamRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		a.logger.Error("打开注册表失败: %v", err)
		return "", fmt.Errorf("无法读取注册表，请手动指定 Steam 路径")
	}
	defer k.Close()

	path, _, err := k.GetStringValue(SteamRegistryValueName)
	if err != nil {
		a.logger.Error("读取注册表 Steam 路径失败: %v", err)
		return "", fmt.Errorf("注册表中未找到 Steam 路径，请手动指定")
	}

	normalized := filepath.FromSlash(path)
	if err := a.persistSteamPath(normalized); err != nil {
		// 路径识别成功但保存失败：仍返回路径让本次会话可用，
		// 只是下次启动需要重新识别。
		a.logger.Warn("Steam 路径已识别但保存失败: %v", err)
	}

	return normalized, nil
}

// SetSteamPath 手动指定 Steam 安装路径。
//
// 参数：
//   - path: 用户选择的 Steam 安装目录
//
// 返回值：
//   - *OperationResult: 校验与保存结果
//
// 校验逻辑：确认路径存在、是目录、且其下有 config 子目录。
// 后者是 Steam 目录的基本特征，能挡住用户误选上级目录的情况。
func (a *App) SetSteamPath(path string) *OperationResult {
	if path == "" {
		return failure("路径不能为空")
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		a.logger.Error("手动指定的 Steam 路径无效: %s", path)
		return failure("该路径不存在或不是文件夹")
	}

	if _, err := os.Stat(filepath.Join(path, ConfigDir)); err != nil {
		a.logger.Error("指定路径下未找到 config 目录: %s", path)
		return failure("这里没有找到 config 文件夹，请确认选择的是 Steam 安装目录")
	}

	if err := a.persistSteamPath(path); err != nil {
		return failure(fmt.Sprintf("保存 Steam 路径失败：%v", err))
	}

	a.logger.Info("Steam 路径已手动设置: %s", path)
	return success("Steam 路径已保存")
}

// persistSteamPath 将 Steam 路径写入配置并重建部署器。
//
// 抽取为独立方法是因为 GetSteamPath 与 SetSteamPath 都需要这套
// 「写配置 + 重建部署器」的组合动作，漏掉后者会导致部署到旧路径。
func (a *App) persistSteamPath(path string) error {
	if a.config == nil {
		return fmt.Errorf("配置系统不可用")
	}

	if err := a.config.Update(func(c *AppConfig) {
		c.SteamPath = path
	}); err != nil {
		return err
	}

	a.rebuildDeployer()
	return nil
}

// ============================================================
// 文件对话框
// ============================================================

// SelectDirectory 打开文件夹选择对话框，用于让用户指定 Steam 安装目录。
//
// 返回值：
//   - string: 用户选择的路径；取消选择时返回空字符串
//   - error:  对话框调用失败时返回
func (a *App) SelectDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "请选择 Steam 安装目录",
		DefaultDirectory: a.steamPath(),
	})
}

// SelectZipFile 打开文件选择对话框，用于让用户选择清单包。
//
// 对话框会定位到用户上次选包的目录，省去每次重新翻找的麻烦。
//
// 返回值：
//   - string: 用户选择的文件路径；取消选择时返回空字符串
//   - error:  对话框调用失败时返回
func (a *App) SelectZipFile() (string, error) {
	var lastDir string
	if a.config != nil {
		lastDir = a.config.Get().LastZipDir
	}

	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "选择清单包",
		DefaultDirectory: lastDir,
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP 压缩包 (*.zip)", Pattern: "*.zip"},
		},
	})
}

// ============================================================
// 环境检测
// ============================================================

// DetectEnvironment 检测注入器环境是否就绪。
//
// 返回值永不为 nil。Steam 路径未设置时返回 unknown 状态，
// 与「已检测但未安装」的 missing 状态区分开——前者要引导用户
// 设置路径，后者要引导用户安装注入器，是两条不同的指引。
func (a *App) DetectEnvironment() *DetectorResult {
	return a.detector.Detect(a.steamPath())
}

// GetDeployDir 返回清单文件将被写入的目录。
//
// 供前端在界面上展示「文件会放到哪里」，让用户对工具的行为有数。
func (a *App) GetDeployDir() string {
	return a.deployer.DeployDir()
}

// ============================================================
// 清单包处理
// ============================================================

// ProcessZipFile 解析用户通过对话框选择的清单包。
//
// 参数：
//   - zipPath: 清单包完整路径
//
// 返回值：
//   - *GamePackage: 解析结果，含主游戏信息、Depot 与 DLC 列表
//   - error:        路径为空、包格式无效或解析失败时返回
//
// 解析成功后会记住该文件所在目录，下次打开对话框时直接定位到那里。
func (a *App) ProcessZipFile(zipPath string) (*GamePackage, error) {
	if zipPath == "" {
		return nil, fmt.Errorf("未选择文件")
	}

	gp, err := a.processZipFromPath(zipPath)
	if err != nil {
		return nil, err
	}

	a.rememberZipDir(zipPath)
	return gp, nil
}

// ProcessDroppedFile 解析用户拖拽进窗口的清单包。
//
// 与 ProcessZipFile 的差异在于数据来源：拖拽的文件以二进制形式
// 由前端传入，需先落盘为临时文件才能交给 zip 解析器。
//
// 参数：
//   - fileName: 原始文件名，用于格式校验
//   - fileData: 文件的完整二进制内容
//
// 返回值：
//   - *GamePackage: 解析结果
//   - error:        格式不支持、落盘失败或解析失败时返回
func (a *App) ProcessDroppedFile(fileName string, fileData []byte) (*GamePackage, error) {
	if fileName == "" || len(fileData) == 0 {
		return nil, fmt.Errorf("文件内容为空")
	}

	if !strings.HasSuffix(strings.ToLower(fileName), ".zip") {
		return nil, fmt.Errorf("只支持 .zip 格式的清单包")
	}

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	tempZipPath := filepath.Join(tempDir, filepath.Base(fileName))
	if err := os.WriteFile(tempZipPath, fileData, 0o644); err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, fmt.Errorf("保存临时文件失败: %w", err)
	}

	gp, err := a.processZipFromPath(tempZipPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		return nil, err
	}
	return gp, nil
}

// rememberZipDir 记录清单包所在目录，供下次打开对话框时定位。
//
// 保存失败只记日志不打扰用户——这是纯粹的便利性功能，
// 为它弹一个错误提示反而扰人。
func (a *App) rememberZipDir(zipPath string) {
	if a.config == nil {
		return
	}

	dir := filepath.Dir(zipPath)
	if err := a.config.Update(func(c *AppConfig) {
		c.LastZipDir = dir
	}); err != nil {
		a.logger.Warn("记录上次选包目录失败: %v", err)
	}
}

// processZipFromPath 是两条导入路径共用的解析流程。
//
// 步骤：创建临时目录 → 解压 → Lua VM 解析 → 检测已部署状态。
//
// 临时目录的生命周期：
//
//	解析成功后不立即清理，因为 GamePackage.ManifestFiles 中的路径
//	指向这些临时文件，前端可能需要展示它们。目录会由下次启动时的
//	cleanStaleTempDirs 回收（24 小时门槛）。
//	解析失败则立即清理，避免为无效的包留下垃圾。
func (a *App) processZipFromPath(zipPath string) (*GamePackage, error) {
	a.logger.Info("开始处理清单包: %s", filepath.Base(zipPath))

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		a.logger.Error("创建临时目录失败: %v", err)
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	luaPath, manifestFiles, err := a.unzipFile(zipPath, tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		a.logger.Error("解压失败: %v", err)
		return nil, err
	}

	gp, err := a.parseLuaFile(luaPath)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		a.logger.Error("清单解析失败: %v", err)
		return nil, err
	}

	gp.ManifestFiles = manifestFiles
	gp.Source = SourceLocalImport
	a.detectInstalledDLCs(gp)

	a.logger.Info("解析完成：%s (AppID %s)，DLC %d 项，Depot %d 项",
		gp.GameName, gp.MainAppID, len(gp.DLCs), len(gp.Depots))

	return gp, nil
}

// ============================================================
// 安装与卸载
// ============================================================

// InstallDLCs 将清单脚本部署到注入器的监控目录。
//
// 参数：
//   - gp:             已解析的清单包
//   - selectedAppIDs: 用户勾选的 DLC AppID 列表，可为空（仅注册主游戏）
//
// 返回值：
//   - *OperationResult: 操作结果，Message 可直接展示
//
// 与 v1.4 的根本差异：不再关闭 Steam、不写 config.vdf、不复制 manifest。
// 只写一个 .lua 文件，注入器会在 500ms 内自行热重载并刷新 Steam 库，
// 因此提示文案可以直接告诉用户「已添加到库」而无需要求重启。
//
// 部署成功后才写历史记录——顺序颠倒会导致部署失败却留下记录，
// 用户在历史里看到一个实际不存在的条目。
func (a *App) InstallDLCs(gp *GamePackage, selectedAppIDs []string) *OperationResult {
	if gp == nil || gp.MainAppID == "" {
		return failure("清单包无效，请重新导入")
	}
	if a.steamPath() == "" {
		return failure("尚未设置 Steam 路径，请先在设置中指定")
	}

	a.logger.Info("开始部署：%s (AppID %s)，选中 DLC %d 项",
		gp.GameName, gp.MainAppID, len(selectedAppIDs))

	deployedPath, err := a.deployer.Deploy(gp, selectedAppIDs)
	if err != nil {
		a.logger.Error("部署失败: %v", err)
		return failure(fmt.Sprintf("部署失败：%v", err))
	}

	if a.history != nil {
		if err := a.history.Record(gp, selectedAppIDs, filepath.Base(deployedPath), gp.Source); err != nil {
			// 历史是辅助功能，写失败不该让用户以为部署没成功。
			a.logger.Warn("安装历史写入失败: %v", err)
		}
	}

	// 留存清单解析结果，使用户重启应用后仍能调整 DLC 勾选。
	// 与历史同理，写失败不影响部署已然成功这一事实。
	if a.packages != nil {
		if err := a.packages.Save(gp, gp.Source); err != nil {
			a.logger.Warn("清单留存写入失败，重启后将需重新获取才能调整勾选: %v", err)
		}
	}

	a.logger.Info("部署完成: %s", deployedPath)

	// 部署成功仍需检查冲突：外部清单可能携带过期或错误的密钥，而注入器
	// 对同一 AppID 的重复声明取其一且不输出任何警告。一旦落盘便不可观测，
	// 症状表现为「一切正常，直到下载时解密失败」，故此处是唯一的告知时机。
	if external := a.externalDeclarations(gp.MainAppID); len(external) > 0 {
		a.logger.Warn("AppID %s 另被 %d 个外部清单声明: %s",
			gp.MainAppID, len(external), strings.Join(external, ", "))
		return success(fmt.Sprintf(
			"已入库 %d 个 DLC。注意：另有 %d 个清单文件也声明了这个游戏（%s），"+
				"注入器只会采用其中一份密钥。若下载时提示解密失败，请删除多余的文件",
			len(selectedAppIDs), len(external), strings.Join(external, "、")))
	}

	return success(fmt.Sprintf(
		"已入库 %d 个 DLC，Steam 库将在几秒内自动刷新", len(selectedAppIDs)))
}

// RemoveDLCs 移除指定游戏的清单脚本与历史记录。
//
// 参数：
//   - mainAppID: 主游戏 AppID
//
// 返回值：
//   - *OperationResult: 操作结果
//
// 调用顺序为「先删文件，再删记录」。若反过来，删记录成功而删文件失败
// 会留下一个孤儿清单文件——它不出现在历史里，界面上也没有任何入口
// 能清理它，用户只能手动去 Steam 目录翻找。
func (a *App) RemoveDLCs(mainAppID string) *OperationResult {
	if mainAppID == "" {
		return failure("缺少游戏 AppID")
	}
	if a.steamPath() == "" {
		return failure("尚未设置 Steam 路径，请先在设置中指定")
	}

	a.logger.Info("开始移除 AppID %s 的清单", mainAppID)

	// 删除前先记录本工具产物之外还有谁声明了这个游戏。
	// 必须在删除前查，删除后自己的文件已不在，无从区分「本就只有外部文件」
	// 与「删掉了自己的那份」。
	external := a.externalDeclarations(mainAppID)

	if err := a.deployer.Remove(mainAppID); err != nil {
		a.logger.Error("移除清单失败: %v", err)
		return failure(fmt.Sprintf("移除失败：%v", err))
	}

	if a.history != nil {
		if err := a.history.Delete(mainAppID); err != nil {
			a.logger.Warn("移除安装记录失败: %v", err)
		}
	}

	// 清单留存随记录一并清理。留着它会让「已安装」页出现一个既无部署
	// 文件也无历史记录、却仍能读出清单的幽灵条目。
	if a.packages != nil {
		if err := a.packages.Delete(mainAppID); err != nil {
			a.logger.Warn("移除清单留存失败: %v", err)
		}
	}

	a.logger.Info("移除完成: AppID %s", mainAppID)

	// 存在外部声明时不得报告「已移除」。注入器按引用计数管理许可证，
	// 另有文件声明该游戏时计数不归零，游戏仍留在 Steam 库中且重启不消失。
	// 谎报成功会让用户以为工具失灵，而真正的原因无从察觉。
	if len(external) > 0 {
		a.logger.Warn("AppID %s 仍被 %d 个外部清单声明: %s",
			mainAppID, len(external), strings.Join(external, ", "))
		return failure(fmt.Sprintf(
			"本工具的清单已删除，但检测到另外 %d 个清单文件也声明了这个游戏，"+
				"游戏可能仍留在 Steam 库中。如需彻底移除，请手动删除：%s",
			len(external), strings.Join(external, "、")))
	}

	return success("已从库中移除，Steam 将在几秒内自动更新")
}

// externalDeclarations 返回声明了指定游戏、但不属于本工具产物的清单文件名。
//
// 判定依据是文件名后缀：本工具生成的文件一律形如 `<游戏名>_<AppID>.lua`，
// 不匹配该形态者即视为外部文件。之所以用命名而非其他标记来区分，是因为
// 外部文件的内容形态完全不可控，无从植入可靠的归属标识。
//
// 检测失败（目录不可读等）时返回空切片而非报错：冲突提示是增强信息，
// 拿不到不应阻断卸载流程本身。
func (a *App) externalDeclarations(mainAppID string) []string {
	all, err := a.findLuaFilesDeclaring(mainAppID)
	if err != nil {
		a.logger.Warn("外部清单检测失败，已跳过: %v", err)
		return []string{}
	}

	ownSuffix := "_" + mainAppID + LuaFileExt
	out := make([]string, 0, len(all))
	for _, name := range all {
		if !strings.HasSuffix(name, ownSuffix) {
			out = append(out, name)
		}
	}
	return out
}

// ============================================================
// 安装历史
// ============================================================

// GetHistory 返回全部安装历史，按最近安装时间倒序。
//
// 返回空切片而非 nil：Wails 会把 nil 切片序列化为 JSON null，
// 前端遍历时会报错。
func (a *App) GetHistory() []GameRecord {
	if a.history == nil {
		return []GameRecord{}
	}
	return a.history.List()
}

// FindHistory 按主游戏 AppID 查询单条历史记录。
//
// 用途是用户再次导入同一游戏的清单包时，带出上次勾选的 DLC
// 作为默认选中项，免得重新一个个点。未找到时返回 nil。
func (a *App) FindHistory(mainAppID string) *GameRecord {
	if a.history == nil || mainAppID == "" {
		return nil
	}
	return a.history.Find(mainAppID)
}

// ClearHistory 清空全部安装历史。
//
// NOTE: 只清空记录，不会移除已部署的清单文件——用户点「清空历史」
// 的意图是整理列表，不应因此让已入库的游戏悄悄消失。
func (a *App) ClearHistory() *OperationResult {
	if a.history == nil {
		return failure("历史系统不可用")
	}

	if err := a.history.Clear(); err != nil {
		a.logger.Error("清空安装历史失败: %v", err)
		return failure(fmt.Sprintf("清空失败：%v", err))
	}
	return success("安装历史已清空")
}

// GetPackage 读取指定游戏的留存清单。
//
// 存在意义是让用户重启应用后仍能调整已入库游戏的 DLC 勾选。压缩包在
// 处理后即删，若无留存，唯一的办法是重新联网下载一次——既耗流量也
// 消耗认证型源的每日额度。
//
// 参数：
//   - mainAppID: 主游戏 AppID
//
// 返回值：
//   - *StoredPackage: 留存内容，含写入时刻与来源。无留存时为 nil
//   - error:          文件存在但无法使用时返回。前端应把「返回 nil 且
//     无错误」与「出错」区别对待：前者引导用户获取清单，后者提示重试
//
// NOTE: 不做过期判定。清单旧不等于无效，是否重新获取由用户按 SavedAt
// 自行决定——界面应表述为「获取于 X 天前」而非「已过期」。
func (a *App) GetPackage(mainAppID string) (*StoredPackage, error) {
	if a.packages == nil {
		return nil, fmt.Errorf("清单留存系统不可用")
	}

	stored, err := a.packages.Load(mainAppID)
	if err != nil {
		a.logger.Warn("读取 AppID %s 的清单留存失败: %v", mainAppID, err)
		return nil, err
	}
	return stored, nil
}

// ============================================================
// 部署目录对账
// ============================================================

// ScanDeployed 扫描注入器目录，列出所有清单文件及其归属。
//
// 存在意义是让界面反映磁盘的真实状态，而非本工具的记忆。注入器会加载
// 目录内全部 .lua 的并集，因此仅凭安装历史渲染列表会漏掉外部文件——
// 用户看到「未安装」而 Steam 中确有，或卸载后游戏仍在库中而界面已清空。
//
// 返回值：
//   - []DeployedEntry: 每个文件一条，按目录遍历序。目录不存在或为空时
//     返回空切片
//
// 无法读取的文件会被跳过并记录警告，不影响其余条目。
//
// NOTE: 本方法只读，不修改也不删除任何文件。外部清单的处置须由用户明确
// 发起——它们可能含用户特意配置的内容，代为清理属越权。
func (a *App) ScanDeployed() []DeployedEntry {
	out := []DeployedEntry{}
	if a.steamPath() == "" {
		return out
	}

	dir := a.deployer.DeployDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			a.logger.Warn("扫描部署目录失败 %s: %v", dir, err)
		}
		return out
	}

	// 预取历史中的 AppID 集合，用于判定「本工具是否记得这个游戏」。
	known := make(map[string]struct{})
	if a.history != nil {
		for _, rec := range a.history.List() {
			known[rec.MainAppID] = struct{}{}
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), LuaFileExt) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			a.logger.Warn("读取清单文件 %s 失败，已跳过: %v", entry.Name(), err)
			continue
		}

		out = append(out, buildDeployedEntry(entry.Name(), string(data), known))
	}

	return out
}

// buildDeployedEntry 依据文件名与内容构造一条对账结果。
//
// 主 AppID 取自文件名后缀，取不到时退而使用内容中第一个被声明的 AppID：
// 外部文件常以 `<AppID>.lua` 命名，此时文件名本身即主 AppID；而完全不合
// 命名惯例的文件只能靠内容推断，其首个 addappid 按清单脚本的书写惯例
// 即为主游戏。
func buildDeployedEntry(fileName, content string, known map[string]struct{}) DeployedEntry {
	declared := luaDeclaredAppIDs(content)

	mainAppID := mainAppIDFromFileName(fileName)
	if mainAppID == "" && len(declared) > 0 {
		mainAppID = declared[0]
	}

	_, isKnown := known[mainAppID]
	return DeployedEntry{
		FileName:   fileName,
		MainAppID:  mainAppID,
		AppIDs:     declared,
		IsExternal: !strings.HasSuffix(fileName, "_"+mainAppID+LuaFileExt),
		InHistory:  isKnown,
	}
}

// mainAppIDFromFileName 从本工具的命名格式中提取主 AppID。
//
// 仅识别 `<任意>_<数字>.lua` 形态，不匹配时返回空字符串。
// 纯数字命名（如 `2399830.lua`）也会被识别——外部文件多为此形态，
// 而它同样是可靠的 AppID 来源。
func mainAppIDFromFileName(fileName string) string {
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	if idx := strings.LastIndex(base, "_"); idx >= 0 {
		if tail := base[idx+1:]; isNumeric(tail) {
			return tail
		}
		return ""
	}

	if isNumeric(base) {
		return base
	}
	return ""
}

// ============================================================
// 在线商店元数据
// ============================================================

// SearchGames 按关键词搜索 Steam 游戏，供前端搜索页调用。
//
// 参数：
//   - term: 搜索关键词，或一个纯数字 AppID
//
// 返回值：
//   - []GameSearchResult: 搜索结果，无匹配或查询失败时为空切片
//   - error:              网络或解析失败的原因
//
// NOTE: 结果中的 Available 字段恒为 false。收录检测在用户进入详情页时
// 单独执行（见 ⑤ 的 RepoClient），不对整列结果预先探测。
func (a *App) SearchGames(term string) ([]GameSearchResult, error) {
	results, err := a.store.Search(term)
	if err != nil {
		a.logger.Warn("搜索 %q 失败: %v", term, err)
		return []GameSearchResult{}, err
	}
	return results, nil
}

// GetGameDetail 获取指定 AppID 的游戏详情，供前端游戏页调用。
//
// 参数：
//   - appID: 游戏的 Steam AppID
//
// 返回值：
//   - *GameDetail: 详情数据，任何情况下均非 nil。查询失败时为仅含 AppID
//     与封面 URL 的降级结果
//   - error: 查询失败的原因。前端可选择忽略，直接渲染降级结果
func (a *App) GetGameDetail(appID string) (*GameDetail, error) {
	detail, err := a.store.Detail(appID)
	if err != nil {
		a.logger.Warn("获取 AppID %s 详情失败: %v", appID, err)
	}
	return detail, err
}

// ============================================================
// 在线清单仓库
// ============================================================

// LookupRepos 查询哪些清单源收录了指定 AppID，供前端游戏页调用。
//
// 参数：
//   - appID: 游戏的 Steam AppID
//
// 返回值：
//   - []string: 收录该 AppID 的源名称，未收录时为空切片
//   - error:    AppID 格式非法时返回
//
// 返回空切片意味着三源均未收录，此时界面应就近引导用户改用本地导入，
// 而非只显示一句「未找到」把人堵在死路上。
func (a *App) LookupRepos(appID string) ([]string, error) {
	names, err := a.repo.Lookup(appID)
	if err != nil {
		a.logger.Warn("查询 AppID %s 的收录情况失败: %v", appID, err)
		return []string{}, err
	}
	return names, nil
}

// DownloadFromRepo 从在线仓库下载并解析清单包。
//
// 参数：
//   - appID:      游戏的 Steam AppID
//   - sourceName: 指定源名称，留空表示自动尝试所有启用的源
//
// 返回值：
//   - *GamePackage: 已解析的清单包，与本地导入的产出完全一致
//   - error:        下载或解析失败的原因
//
// 下载得到的压缩包在解析完成后立即删除——GamePackage 已包含全部所需信息，
// 而仓库会随游戏更新刷新 manifest，留着旧包只会诱使将来误用过期清单。
func (a *App) DownloadFromRepo(appID string, sourceName string) (*GamePackage, error) {
	zipPath, hitSource, err := a.repo.Fetch(appID, sourceName)
	if err != nil {
		a.logger.Error("下载 AppID %s 的清单包失败: %v", appID, err)
		return nil, err
	}

	// 无论解析成败都清理下载目录：解析产物中的 manifest 路径指向的是
	// 解析流程自建的另一个临时目录，与此处无关。
	defer func() { _ = os.RemoveAll(filepath.Dir(zipPath)) }()

	// 按包内实际内容分派，不按来源假定形态。
	//
	// 三个源的包结构互不相同，且上游随时可能调整。若按源名称硬编码
	// 解析方式，任一源改了打包脚本就会静默失效——而按内容判别的话，
	// 只要包里有 .lua 就走 Lua VM，没有就试 MAU 形态。
	hasLua, err := zipContainsLua(zipPath)
	if err != nil {
		a.logger.Error("检查清单包格式失败: %v", err)
		return nil, err
	}

	var gp *GamePackage
	if hasLua {
		gp, err = a.processZipFromPath(zipPath)
	} else {
		gp, err = a.processMAUZip(zipPath)
	}
	if err != nil {
		return nil, err
	}

	// 在解析之后覆写：Lua 路径复用的 processZipFromPath 会将 Source
	// 填为本地导入（它本是离线入口），此处必须纠正为实际命中的源名。
	gp.Source = hitSource
	return gp, nil
}

// processMAUZip 处理 MAU 形态的清单包：解压 → 结构化解析 → 回填名称。
//
// 临时目录的生命周期与 Lua 路径一致：解析成功后不清理（GamePackage 的
// ManifestFiles 指向其中的文件），由下次启动的 cleanStaleTempDirs 回收；
// 失败则立即清理。
func (a *App) processMAUZip(zipPath string) (*GamePackage, error) {
	a.logger.Info("清单包内无 .lua，按 MAU 形态解析: %s", filepath.Base(zipPath))

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	count, err := unzipMAUPackage(zipPath, tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		a.logger.Error("解压失败: %v", err)
		return nil, err
	}

	gp, pending, err := parseMAUPackage(tempDir)
	if err != nil {
		_ = os.RemoveAll(tempDir)
		a.logger.Error("MAU 清单解析失败: %v", err)
		return nil, err
	}

	// 独立 Depot 的 DLC 密钥不在主游戏分支内，需各拉一次自己的分支。
	if len(pending) > 0 {
		a.enrichPackageDLCs(gp, pending)
	}

	// MAU 包不含游戏名与 DLC 名，向商店补齐。
	//
	// 失败不影响可用性：名称只用于界面展示，AppID 才是部署的依据。
	// 故此处忽略错误，仅在拿到结果时覆盖占位名称。
	a.fillNamesFromStore(gp)
	a.detectInstalledDLCs(gp)

	a.logger.Info("MAU 解析完成：%s (AppID %s)，Depot %d 项，DLC %d 项，解压 %d 个文件",
		gp.GameName, gp.MainAppID, len(gp.Depots), len(gp.DLCs), count)

	return gp, nil
}

// enrichPackageDLCs 为带独立 Depot 的 DLC 补齐密钥与 manifest ID。
//
// 为何需要额外下载：
//
//	MAU 为每个独立 Depot 的 DLC 单独开了一个以其 AppID 命名的分支，
//	主游戏分支内只有主 Depot 的密钥。实测 SF6（1364780）的分支里没有
//	1792750 的密钥，而 1792750 自己的分支里有。
//
//	缺这两项的后果不是「少显示一个体积」——生成脚本会退化为单行注册，
//	Steam 拿不到密钥便无法解密该 DLC 的内容，用户勾选了却装不上。
//
// 并发拉取但限制并发数：DLC 数量可能达到二十以上（ARK 系列），
// 全部同时发起会让公益代理直接限流，反而更慢。
//
// 参数：
//   - gp:      待补齐的清单包，其 DLCs 字段会被就地更新
//   - appIDs:  需补齐的 DLC AppID 列表
//
// 单个 DLC 补齐失败不中断整体：该 DLC 退化为单行注册，其余仍然可用。
// 这比让整个游戏因为一个 DLC 装不上而完全失败要好。
func (a *App) enrichPackageDLCs(gp *GamePackage, appIDs []string) {
	const maxConcurrent = 4

	type result struct {
		appID      string
		key        string
		manifestID string
		size       int64
	}

	results := make([]result, len(appIDs))
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, appID := range appIDs {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			key, manifestID, size, err := a.fetchDLCDepot(id)
			if err != nil {
				a.logger.Warn("补齐 DLC %s 的密钥失败，将退化为单行注册: %v", id, err)
				return
			}
			results[idx] = result{appID: id, key: key, manifestID: manifestID, size: size}
		}(i, appID)
	}
	wg.Wait()

	filled := 0
	for _, res := range results {
		if res.appID == "" || res.key == "" {
			continue
		}
		for i := range gp.DLCs {
			if gp.DLCs[i].AppID != res.appID {
				continue
			}
			gp.DLCs[i].HasKey = true
			gp.DLCs[i].DecryptionKey = res.key
			if res.manifestID != "" {
				gp.DLCs[i].ManifestID = res.manifestID
				gp.DLCs[i].FileSize = res.size
			}
			filled++
			break
		}
	}

	a.logger.Info("独立 Depot 的 DLC 补齐完成：%d/%d 项", filled, len(appIDs))
}

// fetchDLCDepot 拉取单个 DLC 自己的分支，取出其密钥与 manifest ID。
//
// 返回值：
//   - string: 解密密钥
//   - string: manifest ID
//   - int64:  manifest 文件体积
//   - error:  未收录或解析失败时返回
func (a *App) fetchDLCDepot(appID string) (string, string, int64, error) {
	// 忽略返回的源名：此处只为补齐单个 DLC 的密钥，来源已由主包记录。
	zipPath, _, err := a.repo.Fetch(appID, "")
	if err != nil {
		return "", "", 0, err
	}
	defer func() { _ = os.RemoveAll(filepath.Dir(zipPath)) }()

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return "", "", 0, err
	}
	// DLC 分支的产物只为取两个值，用完即删——不像主包那样需要保留
	// manifest 文件路径供前端展示。
	defer func() { _ = os.RemoveAll(tempDir) }()

	if _, err := unzipMAUPackage(zipPath, tempDir); err != nil {
		return "", "", 0, err
	}
	return readDepotCredentials(tempDir, appID)
}

// fillNamesFromStore 用商店元数据回填 GamePackage 中缺失的名称。
//
// MAU 形态的包只有数字 ID，界面若直接展示「DLC 2224460」用户无从判断
// 该不该勾选。主游戏名同时决定部署文件名，缺失时会退化为「游戏 123456」。
//
// 只回填主游戏名与 DLC 名，不改动任何 ID——名称是展示层信息，
// 错了不影响功能；ID 错了则会部署出无效清单。
func (a *App) fillNamesFromStore(gp *GamePackage) {
	detail, err := a.store.Detail(gp.MainAppID)
	if err == nil && detail.Name != "" {
		gp.GameName = detail.Name
	}
	if gp.GameName == "" {
		gp.GameName = "游戏 " + gp.MainAppID
	}

	// DLC 名称逐个查询会产生数十次请求，故不在此处做。
	// 界面进入详情页后可按需补齐单个 DLC 的名称。
	//
	// TODO(⑧ 阶段): 前端按需查询 DLC 名称，或调研 appdetails 的批量形式。
}

// zipContainsLua 检查压缩包内是否存在 .lua 文件。
//
// 只读中央目录，不解压任何内容，故开销与包体积无关。
func zipContainsLua(zipPath string) (bool, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, fmt.Errorf("无法打开压缩包: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(f.Name), LuaFileExt) {
			return true, nil
		}
	}
	return false, nil
}

// ============================================================
// 认证型清单源（可选增强）
// ============================================================

// SetRepoToken 保存指定清单源的 API 凭据。
//
// 参数：
//   - sourceName: 源名称
//   - token:      API 凭据。传空字符串即清除，该源随即停止参与工作
//
// 返回值：
//   - *OperationResult: 操作结果，Message 可直接展示
//
// NOTE: 凭据以明文存于 config.json。这在桌面应用中属常规做法，但意味着
// 日志中绝不可输出它——本方法只记录「已设置/已清除」而不记内容。
func (a *App) SetRepoToken(sourceName string, token string) *OperationResult {
	if a.config == nil {
		return failure("配置不可用，无法保存凭据")
	}

	token = strings.TrimSpace(token)
	found := false
	err := a.config.Update(func(c *AppConfig) {
		for i := range c.RepoSources {
			if !strings.EqualFold(c.RepoSources[i].Name, sourceName) {
				continue
			}
			c.RepoSources[i].Token = token
			found = true
			return
		}
	})
	if err != nil {
		return failure(fmt.Sprintf("保存凭据失败：%v", err))
	}
	if !found {
		return failure(fmt.Sprintf("未找到名为 %q 的清单源", sourceName))
	}

	if token == "" {
		a.logger.Info("已清除源 %s 的 API 凭据", sourceName)
		return success("已清除凭据，该源不再参与工作")
	}
	a.logger.Info("已设置源 %s 的 API 凭据", sourceName)
	return success("凭据已保存")
}

// GetMSiteStats 查询认证型清单源的额度与凭据状态。
//
// 走该站的免额度端点，可在启动时与设置页打开时安全调用。
//
// 返回值：
//   - *MSiteStats: 账户状态。未配置凭据时为 nil，前端据此隐藏相关区块
//   - error:       凭据无效或网络失败时返回
//
// 界面应在 ExpiringSoon 为真时挂顶部横幅：该站凭据默认仅 7 天有效，
// 不主动提示的话用户只会在某次下载失败时才发现，且无从判断原因。
func (a *App) GetMSiteStats() (*MSiteStats, error) {
	stats, err := a.repo.MSiteAccountStats()
	if err != nil {
		a.logger.Warn("查询清单源额度失败: %v", err)
		return nil, err
	}
	return stats, nil
}

// ============================================================
// 结果构造
// ============================================================

// success 构造一个成功的操作结果。
func success(message string) *OperationResult {
	return &OperationResult{Success: true, Message: message}
}

// failure 构造一个失败的操作结果。
//
// 传入的 message 会直接展示给用户，故应写成面向用户的说明
// （「尚未设置 Steam 路径」）而非技术描述（「steamPath is empty」）。
func failure(message string) *OperationResult {
	return &OperationResult{Success: false, Message: message}
}
