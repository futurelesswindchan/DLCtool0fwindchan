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

// DepotInfo 表示一个 Depot 的信息（带解密密钥和 Manifest）
type DepotInfo struct {
	DepotID       string `json:"depotID"`
	DecryptionKey string `json:"decryptionKey"`
	ManifestID    string `json:"manifestID"`
	FileSize      int64  `json:"fileSize"`
}

// DLCInfo 表示一个 DLC 的信息
type DLCInfo struct {
	AppID         string `json:"appID"`
	Name          string `json:"name"`
	HasKey        bool   `json:"hasKey"`
	DecryptionKey string `json:"decryptionKey"`
	IsInstalled   bool   `json:"isInstalled"`
}

// GamePackage 表示从 Lua 文件解析出的完整游戏数据
type GamePackage struct {
	MainAppID     string      `json:"mainAppID"`
	GameName      string      `json:"gameName"`
	Depots        []DepotInfo `json:"depots"`
	DLCs          []DLCInfo   `json:"dlcs"`
	LuaContent    string      `json:"luaContent"`
	ManifestFiles []string    `json:"manifestFiles"`
}

// OperationResult 表示操作结果
type OperationResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// App struct
type App struct {
	ctx       context.Context
	steamPath string
}

// NewApp 创建一个新的 App 实例
func NewApp() *App {
	return &App{}
}

// startup 在应用启动时调用，保存 context
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ============================================================
// 公开方法（供前端调用）
// ============================================================

// GetSteamPath 获取 Steam 安装路径
func (a *App) GetSteamPath() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("无法打开注册表: %w", err)
	}
	defer k.Close()

	path, _, err := k.GetStringValue("SteamPath")
	if err != nil {
		return "", fmt.Errorf("无法读取 Steam 路径: %w", err)
	}

	a.steamPath = filepath.FromSlash(path)
	return a.steamPath, nil
}

// SelectZipFile 打开文件选择对话框，让用户选择 zip 文件
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

// ProcessZipFile 处理用户选择的 zip 文件，解析其中的 Lua 和 Manifest
func (a *App) ProcessZipFile(zipPath string) (*GamePackage, error) {
	if zipPath == "" {
		return nil, fmt.Errorf("未选择文件")
	}

	// 确保 Steam 路径已获取
	if a.steamPath == "" {
		if _, err := a.GetSteamPath(); err != nil {
			return nil, err
		}
	}

	// 创建临时目录用于解压
	tempDir, err := os.MkdirTemp("", "dlctool_")
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

	// 将临时目录中的 manifest 文件路径保存到 GamePackage
	gamePackage.ManifestFiles = manifestFiles

	// 检测已安装的 DLC
	a.detectInstalledDLCs(gamePackage)

	return gamePackage, nil
}

// InstallDLCs 安装用户选中的 DLC
func (a *App) InstallDLCs(gamePackage *GamePackage, selectedAppIDs []string) (*OperationResult, error) {
	if a.steamPath == "" {
		return nil, fmt.Errorf("Steam 路径未初始化")
	}

	// 关闭 Steam 进程
	a.killSteam()
	time.Sleep(2 * time.Second)

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

// RemoveAllDLCs 清除指定游戏的所有伪入库 DLC
func (a *App) RemoveAllDLCs(gamePackage *GamePackage) (*OperationResult, error) {
	if a.steamPath == "" {
		return nil, fmt.Errorf("Steam 路径未初始化")
	}

	// 关闭 Steam 进程
	a.killSteam()
	time.Sleep(2 * time.Second)

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

// ProcessDroppedFile 处理拖拽上传的文件（二进制数据）
func (a *App) ProcessDroppedFile(fileName string, fileData []byte) (*GamePackage, error) {
	if fileName == "" || len(fileData) == 0 {
		return nil, fmt.Errorf("文件数据为空")
	}

	if !strings.HasSuffix(fileName, ".zip") {
		return nil, fmt.Errorf("只支持 .zip 格式文件")
	}

	// 确保 Steam 路径已获取
	if a.steamPath == "" {
		if _, err := a.GetSteamPath(); err != nil {
			return nil, err
		}
	}

	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "dlctool_")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// 将二进制数据写入临时 zip 文件
	tempZipPath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(tempZipPath, fileData, 0644); err != nil {
		return nil, fmt.Errorf("保存临时文件失败: %w", err)
	}

	// 解压 zip 文件
	luaPath, manifestFiles, err := a.unzipFile(tempZipPath, tempDir)
	if err != nil {
		return nil, err
	}

	// 解析 Lua 文件
	gamePackage, err := a.parseLuaFile(luaPath)
	if err != nil {
		return nil, err
	}

	// 保存 manifest 文件路径
	gamePackage.ManifestFiles = manifestFiles

	// 检测已安装的 DLC
	a.detectInstalledDLCs(gamePackage)

	return gamePackage, nil
}
