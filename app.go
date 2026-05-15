// app.go
//
// 本文件是应用的核心入口，定义了 App 结构体及其生命周期方法，
// 以及所有暴露给前端调用的公开 API。
//
// 职责划分：
//   - App 结构体管理应用状态（context、Steam 路径）
//   - 路径辅助方法统一提供各配置文件的完整路径（基于 constants.go 中的常量）
//   - 公开方法处理前端请求：文件选择、zip 处理、DLC 安装/卸载
//
// 数据结构定义已抽离至 types.go，路径常量已抽离至 constants.go。

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

// App 是应用的主结构体，持有运行时上下文和 Steam 安装路径。
//
// 该结构体的实例通过 Wails 框架绑定到前端，前端可直接调用其公开方法。
// steamPath 字段在首次调用 GetSteamPath() 或 SetSteamPath() 时初始化，
// 后续所有文件操作均依赖该路径。
type App struct {
	ctx       context.Context
	steamPath string
}

// NewApp 创建并返回一个新的 App 实例。
//
// 此时 steamPath 尚未初始化，需要在后续通过 GetSteamPath() 自动识别
// 或通过 SetSteamPath() 手动指定。
func NewApp() *App {
	return &App{}
}

// startup 是 Wails 框架的生命周期回调，在应用窗口创建后调用。
//
// 参数 ctx 是 Wails 运行时上下文，用于调用对话框、事件等框架功能。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ============================================================
// 路径辅助方法
// ============================================================
//
// 以下方法基于 steamPath 和 constants.go 中定义的常量，
// 统一生成各配置文件和目录的完整路径。
// 所有业务函数应通过这些方法获取路径，而非自行拼接字符串。

// configVDFPath 返回 Steam config.vdf 文件的完整路径。
//
// 示例返回值：C:\Program Files\Steam\config\config.vdf
func (a *App) configVDFPath() string {
	return filepath.Join(a.steamPath, ConfigDir, ConfigVDFFile)
}

// steamtoolsLuaPath 返回 Steamtools.lua 文件的完整路径。
//
// 示例返回值：C:\Program Files\Steam\config\stplug-in\Steamtools.lua
func (a *App) steamtoolsLuaPath() string {
	return filepath.Join(a.steamPath, ConfigDir, SteamtoolsPluginDir, SteamtoolsLuaFile)
}

// steamtoolsLuaDir 返回 Steamtools.lua 所在目录的完整路径。
//
// 用于在写入 Lua 文件前确保目录存在（os.MkdirAll）。
// 示例返回值：C:\Program Files\Steam\config\stplug-in\
func (a *App) steamtoolsLuaDir() string {
	return filepath.Join(a.steamPath, ConfigDir, SteamtoolsPluginDir)
}

// depotcachePath 返回 Steam depotcache 目录的完整路径。
//
// 示例返回值：C:\Program Files\Steam\depotcache\
func (a *App) depotcachePath() string {
	return filepath.Join(a.steamPath, DepotcacheDir)
}

// ============================================================
// 公开方法（供前端调用）
// ============================================================

// GetSteamPath 从 Windows 注册表自动识别 Steam 安装路径。
//
// 读取 HKEY_CURRENT_USER\Software\Valve\Steam 下的 SteamPath 值，
// 并将结果缓存到 App.steamPath 中供后续操作使用。
//
// 返回值：
//   - string: Steam 安装目录的本地路径（使用系统路径分隔符）
//   - error:  注册表访问失败或值不存在时返回错误
//
// 局限性：
//   仅能识别注册表中记录的路径，无法覆盖多盘安装或手动迁移场景。
//   对于这些情况，用户应通过 SetSteamPath() 手动指定。
func (a *App) GetSteamPath() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, SteamRegistryKey, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("无法打开注册表: %w", err)
	}
	defer k.Close()

	path, _, err := k.GetStringValue(SteamRegistryValueName)
	if err != nil {
		return "", fmt.Errorf("无法读取 Steam 路径: %w", err)
	}

	a.steamPath = filepath.FromSlash(path)
	return a.steamPath, nil
}

// SetSteamPath 允许用户手动指定 Steam 安装路径。
//
// 当自动识别（GetSteamPath）结果不正确时（如 Steam 被迁移到其他盘符、
// 存在多个 Steam 库目录等），前端可调用此方法让用户手动选择正确路径。
//
// 参数：
//   - path: 用户指定的 Steam 安装目录路径
//
// 返回值：
//   - error: 路径不存在或不包含预期的 Steam 目录结构时返回错误
//
// 校验逻辑：
//   检查指定路径下是否存在 config 子目录，作为基本的合法性验证。
func (a *App) SetSteamPath(path string) error {
	// 基本校验：确认路径存在
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("指定的路径不存在或不是目录: %s", path)
	}

	// 校验：确认路径下存在 config 目录（Steam 目录的基本特征）
	configDir := filepath.Join(path, ConfigDir)
	if _, err := os.Stat(configDir); err != nil {
		return fmt.Errorf("指定路径下未找到 config 目录，请确认这是正确的 Steam 安装路径: %s", path)
	}

	a.steamPath = path
	return nil
}

// SelectZipFile 打开系统文件选择对话框，让用户选择 DLC 压缩包。
//
// 使用 Wails 运行时提供的原生对话框，仅允许选择 .zip 格式文件。
//
// 返回值：
//   - string: 用户选择的文件完整路径；若用户取消选择则返回空字符串
//   - error:  对话框调用失败时返回错误
func (a *App) SelectZipFile() (string, error) {
	selection, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 DLC 压缩包",
		Filters: []runtime.FileFilter{
			{DisplayName: "ZIP 压缩包 (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return "", err
	}
	return selection, nil
}

// ProcessZipFile 处理用户通过文件对话框选择的 zip 文件。
//
// 该方法是对外暴露的公开 API，内部委托 processZipFromPath 完成实际工作。
//
// 参数：
//   - zipPath: zip 文件的完整路径
//
// 返回值：
//   - *GamePackage: 解析后的完整游戏数据包
//   - error:       文件为空或处理失败时返回错误
func (a *App) ProcessZipFile(zipPath string) (*GamePackage, error) {
	if zipPath == "" {
		return nil, fmt.Errorf("未选择文件")
	}

	return a.processZipFromPath(zipPath)
}

// InstallDLCs 执行 DLC 安装操作。
//
// 完整流程：
//   1. 关闭 Steam 进程（写入配置文件前必须确保 Steam 未锁定文件）
//   2. 复制 manifest 文件到 depotcache 目录
//   3. 在 config.vdf 的 depots 节点中写入解密密钥
//   4. 在 Steamtools.lua 中追加 addappid() 调用
//
// 参数：
//   - gamePackage:    解析后的游戏数据包
//   - selectedAppIDs: 用户选中要安装的 DLC AppID 列表
//
// 返回值：
//   - *OperationResult: 操作结果（成功/失败及描述信息）
//   - error:            Steam 路径未初始化等前置条件不满足时返回错误
func (a *App) InstallDLCs(gamePackage *GamePackage, selectedAppIDs []string) (*OperationResult, error) {
	if a.steamPath == "" {
		return nil, fmt.Errorf("Steam 路径未初始化")
	}

	// 关闭 Steam 进程
	a.killSteam()
	time.Sleep(time.Duration(KillSteamWaitDuration) * time.Second)

	// 构建选中的 DLC 集合
	selectedSet := make(map[string]bool)
	for _, id := range selectedAppIDs {
		selectedSet[id] = true
	}

	// 步骤 1：复制 Manifest 文件到 depotcache
	if err := a.copyManifests(gamePackage, selectedSet); err != nil {
		return &OperationResult{Success: false, Message: fmt.Sprintf("复制清单文件失败: %v", err)}, nil
	}

	// 步骤 2：修改 config.vdf
	if err := a.patchConfigVDF(gamePackage, selectedSet); err != nil {
		return &OperationResult{Success: false, Message: fmt.Sprintf("修改 config.vdf 失败: %v", err)}, nil
	}

	// 步骤 3：修改 Steamtools.lua
	if err := a.patchSteamtoolsLua(gamePackage, selectedSet); err != nil {
		return &OperationResult{Success: false, Message: fmt.Sprintf("修改 Steamtools.lua 失败: %v", err)}, nil
	}

	return &OperationResult{
		Success: true,
		Message: fmt.Sprintf("成功安装 %d 个 DLC！请重启 Steam。", len(selectedAppIDs)),
	}, nil
}

// RemoveAllDLCs 清除指定游戏的所有伪入库 DLC。
//
// 完整流程：
//   1. 关闭 Steam 进程
//   2. 删除 depotcache 中对应的 manifest 文件
//   3. 从 config.vdf 中移除解密密钥块
//   4. 从 Steamtools.lua 中移除 addappid() 调用行
//
// 参数：
//   - gamePackage: 解析后的游戏数据包（用于确定需要清理的 AppID 范围）
//
// 返回值：
//   - *OperationResult: 操作结果
//   - error:            前置条件不满足时返回错误
func (a *App) RemoveAllDLCs(gamePackage *GamePackage) (*OperationResult, error) {
	if a.steamPath == "" {
		return nil, fmt.Errorf("Steam 路径未初始化")
	}

	// 关闭 Steam 进程
	a.killSteam()
	time.Sleep(time.Duration(KillSteamWaitDuration) * time.Second)

	// 收集所有相关的 AppID
	allAppIDs := a.collectAllAppIDs(gamePackage)

	// 步骤 1：删除 depotcache 中的 manifest 文件
	a.removeManifests(allAppIDs)

	// 步骤 2：从 config.vdf 中移除密钥
	if err := a.unpatchConfigVDF(gamePackage); err != nil {
		return &OperationResult{Success: false, Message: fmt.Sprintf("恢复 config.vdf 失败: %v", err)}, nil
	}

	// 步骤 3：从 Steamtools.lua 中移除 addappid
	if err := a.unpatchSteamtoolsLua(gamePackage); err != nil {
		return &OperationResult{Success: false, Message: fmt.Sprintf("清理 Steamtools.lua 失败: %v", err)}, nil
	}

	return &OperationResult{
		Success: true,
		Message: "已成功清除所有伪入库 DLC！请重启 Steam。",
	}, nil
}

// ProcessDroppedFile 处理通过拖拽方式上传的文件。
//
// 与 ProcessZipFile 的区别在于：拖拽文件以二进制数据形式传入，
// 需要先写入临时文件再委托 processZipFromPath 完成后续处理。
//
// 参数：
//   - fileName: 拖拽文件的原始文件名（用于格式校验和临时文件命名）
//   - fileData: 文件的完整二进制内容
//
// 返回值：
//   - *GamePackage: 解析后的完整游戏数据包
//   - error:       格式不支持、写入失败或解析失败时返回错误
func (a *App) ProcessDroppedFile(fileName string, fileData []byte) (*GamePackage, error) {
	if fileName == "" || len(fileData) == 0 {
		return nil, fmt.Errorf("文件数据为空")
	}

	if !strings.HasSuffix(fileName, ".zip") {
		return nil, fmt.Errorf("只支持 .zip 格式文件")
	}

	// 创建临时目录，将二进制数据落盘为 zip 文件
	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	tempZipPath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(tempZipPath, fileData, 0644); err != nil {
		os.RemoveAll(tempDir)
		return nil, fmt.Errorf("保存临时文件失败: %w", err)
	}

	// 委托通用处理流程
	return a.processZipFromPath(tempZipPath)
}

// ============================================================
// 内部方法
// ============================================================

// processZipFromPath 是 ProcessZipFile 和 ProcessDroppedFile 的通用实现。
//
// 统一处理流程：
//   1. 确保 Steam 路径已初始化
//   2. 创建临时目录并解压 zip 文件
//   3. 解析 Lua 文件，构建 GamePackage
//   4. 检测已安装的 DLC 状态
//
// 临时目录生命周期说明：
//   成功时临时目录不会被立即清理，因为 ManifestFiles 中的路径
//   在后续 InstallDLCs 步骤中仍需使用（复制到 depotcache）。
//   临时目录会在下次启动工具或系统清理时被回收。
//
// 参数：
//   - zipPath: zip 文件的完整路径（可以是用户选择的原始文件，也可以是临时落盘的文件）
//
// 返回值：
//   - *GamePackage: 解析后的完整游戏数据包
//   - error:       任何步骤失败时返回错误（失败时会清理临时目录）
func (a *App) processZipFromPath(zipPath string) (*GamePackage, error) {
	// 确保 Steam 路径已获取
	if a.steamPath == "" {
		if _, err := a.GetSteamPath(); err != nil {
			return nil, err
		}
	}

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}

	// 解压 zip 文件
	luaPath, manifestFiles, err := a.unzipFile(zipPath, tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	// 解析 Lua 文件
	gamePackage, err := a.parseLuaFile(luaPath)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, err
	}

	// 保存 manifest 文件路径供后续安装使用
	gamePackage.ManifestFiles = manifestFiles

	// 检测已安装的 DLC
	a.detectInstalledDLCs(gamePackage)

	return gamePackage, nil
}
