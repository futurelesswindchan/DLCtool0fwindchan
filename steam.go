// steam.go
//
// 本文件包含清单包的解压与 Steam 路径识别相关的底层操作。
//
// v2.0 相较 v1.4 大幅精简：ST 时代的 config.vdf 读写、Steamtools.lua
// 追加、manifest 复制与删除、关闭 Steam 进程等逻辑全部移除。
// 理由见 docs/DECISIONS.md：本工具已转型为清单包管理盒子，
// 只负责把 .lua 放到注入器的监控目录，其余交由 OpenSteamTool 处理。
//
// 移除的能力及其去向：
//   - 写 config.vdf         → 不再需要，OST 自行管理 depot 密钥
//   - 复制 manifest         → 不再需要，OST 自动下载至 depotcache
//   - 关闭 Steam 进程       → 不再需要，OST 热重载 500ms 内生效
//   - 追加 Steamtools.lua   → 改由 deployer_ost.go 写独立文件
//   - DLC 安装状态检测      → 改为读取 <Steam>/config/lua/ 下的部署产物

package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ============================================================
// Zip 文件解压
// ============================================================

// unzipFile 解压清单包，返回其中的 Lua 文件路径与 manifest 文件路径列表。
//
// 预期的包格式：所有文件位于压缩包根目录，含一个 .lua 与若干 .manifest。
// 解压时以 filepath.Base 提取文件名，忽略包内的目录结构。
//
// manifest 文件仍会被解压出来，但 v2.0 不再将其复制到 depotcache——
// 保留提取逻辑是为了让前端能展示「这个包附带了几个 manifest」，
// 以及在 OST 上游 API 全部失效时用户可手动取用。
//
// 安全校验（清单包来源不可控，以下检查不可省略）：
//   - 跳过目录条目
//   - 拒绝文件名含路径遍历字符（..）
//   - 拒绝空文件名与以点开头的隐藏文件
//   - 仅提取 .lua 与 .manifest，忽略其他类型
//   - 校验最终路径仍位于目标目录内，防止符号链接绕过
//
// 参数：
//   - zipPath: 压缩包完整路径
//   - destDir: 解压目标目录，应为临时目录
//
// 返回值：
//   - string:   Lua 文件的完整路径
//   - []string: 所有 manifest 文件的完整路径
//   - error:    压缩包格式异常或解压失败时返回
func (a *App) unzipFile(zipPath string, destDir string) (string, []string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("无法打开压缩包: %w", err)
	}
	defer r.Close()

	var luaPath string
	var manifestFiles []string

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		fileName := filepath.Base(f.Name)

		if fileName == "" || fileName == "." {
			continue
		}

		// 路径遍历防护：即使后续用了 filepath.Base，仍显式拒绝可疑条目。
		if strings.Contains(f.Name, "..") {
			continue
		}

		if strings.HasPrefix(fileName, ".") {
			continue
		}

		lowerName := strings.ToLower(fileName)
		isLua := strings.HasSuffix(lowerName, LuaFileExt)
		isManifest := strings.HasSuffix(lowerName, ".manifest")
		if !isLua && !isManifest {
			continue
		}

		destPath := filepath.Join(destDir, fileName)

		// 兜底校验：确认解析后的绝对路径仍在目标目录内。
		absDestPath, err := filepath.Abs(destPath)
		if err != nil {
			continue
		}
		absDestDir, err := filepath.Abs(destDir)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(absDestPath, absDestDir+string(filepath.Separator)) {
			continue
		}

		if err := extractZipEntry(f, destPath); err != nil {
			return "", nil, err
		}

		if isLua {
			luaPath = destPath
		} else {
			manifestFiles = append(manifestFiles, destPath)
		}
	}

	if luaPath == "" {
		return "", nil, fmt.Errorf("压缩包中未找到 .lua 文件，请确认这是有效的清单包")
	}

	return luaPath, manifestFiles, nil
}

// extractZipEntry 将压缩包中的单个条目写出到指定路径。
//
// 抽取为独立函数是为了让 defer 的作用域收敛到单个条目——
// 若写在循环内，所有文件句柄要等整个循环结束才释放，
// 处理含大量 manifest 的包时可能耗尽句柄。
func extractZipEntry(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("读取压缩包中的 %s 失败: %w", f.Name, err)
	}
	defer rc.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件 %s 失败: %w", filepath.Base(destPath), err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return fmt.Errorf("解压 %s 失败: %w", filepath.Base(destPath), err)
	}
	return nil
}

// ============================================================
// DLC 安装状态检测
// ============================================================

// detectInstalledDLCs 扫描注入器的部署目录，标记哪些 DLC 已经安装。
//
// v2.0 的检测方式远比 v1.4 简单：既然每个游戏对应一个独立的 .lua 文件，
// 只需读取该文件的内容，看其中出现了哪些 AppID 即可。
// 不再需要解析 config.vdf，也不必处理多个游戏共用一份 Lua 的情况。
//
// 参数：
//   - gp: 待标记的清单包，其 DLCs 字段的 IsInstalled 会被就地更新
//
// 若部署目录或对应文件不存在（首次使用该游戏），所有 DLC 保持未安装状态。
func (a *App) detectInstalledDLCs(gp *GamePackage) {
	if gp == nil || gp.MainAppID == "" {
		return
	}

	content, err := a.readDeployedLua(gp.MainAppID)
	if err != nil {
		// 未部署过此游戏，全部保持未安装。这是正常情况，不记录警告。
		return
	}

	for i := range gp.DLCs {
		if luaContainsAppID(content, gp.DLCs[i].AppID) {
			gp.DLCs[i].IsInstalled = true
		}
	}
}

// readDeployedLua 读取指定游戏已部署的清单脚本内容。
//
// 由于文件名含游戏名而调用方只握有 AppID，此处扫描部署目录
// 匹配 `_<AppID>.lua` 后缀来定位，与 OSTDeployer.Remove 的策略一致。
//
// 返回值：
//   - string: 文件内容
//   - error:  目录不存在、无匹配文件或读取失败时返回
func (a *App) readDeployedLua(mainAppID string) (string, error) {
	dir := a.deployer.DeployDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	suffix := "_" + mainAppID + LuaFileExt
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	return "", fmt.Errorf("未找到 AppID %s 的部署文件", mainAppID)
}
