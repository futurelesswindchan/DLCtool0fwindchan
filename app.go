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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		if err := a.history.Record(gp, selectedAppIDs, filepath.Base(deployedPath)); err != nil {
			// 历史是辅助功能，写失败不该让用户以为部署没成功。
			a.logger.Warn("安装历史写入失败: %v", err)
		}
	}

	a.logger.Info("部署完成: %s", deployedPath)
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

	if err := a.deployer.Remove(mainAppID); err != nil {
		a.logger.Error("移除清单失败: %v", err)
		return failure(fmt.Sprintf("移除失败：%v", err))
	}

	if a.history != nil {
		if err := a.history.Delete(mainAppID); err != nil {
			a.logger.Warn("移除安装记录失败: %v", err)
		}
	}

	a.logger.Info("移除完成: AppID %s", mainAppID)
	return success("已从库中移除，Steam 将在几秒内自动更新")
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

// ============================================================
// 诊断
// ============================================================

// GetLogPath 返回当前日志文件的完整路径。
//
// 供前端「打开日志」功能使用。日志系统降级运行时返回空字符串。
func (a *App) GetLogPath() string {
	return a.logger.Path()
}

// OpenDataDir 在系统文件管理器中打开本工具的数据目录。
//
// 用户报障时常被要求「把日志发过来」，直接提供一个按钮跳转到
// 目录比让人手动输入 %USERPROFILE%\.kazeusa 友好得多。
func (a *App) OpenDataDir() *OperationResult {
	dir, err := appDataDir()
	if err != nil {
		return failure(fmt.Sprintf("数据目录不可用：%v", err))
	}

	// 用系统默认方式打开目录。Wails 的 BrowserOpenURL 对本地路径
	// 同样有效，且省去手动拼 explorer 命令的平台差异处理。
	runtime.BrowserOpenURL(a.ctx, dir)
	return success("已打开数据目录")
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
