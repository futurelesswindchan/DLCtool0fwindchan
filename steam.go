package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ============================================================
// Zip 文件解压
// ============================================================

// unzipFile 解压 zip 文件到指定目录，返回 Lua 文件路径和 Manifest 文件路径列表。
// M 站的 zip 格式：所有文件在根目录，包含一个 .lua 和若干 .manifest 文件。
func (a *App) unzipFile(zipPath string, destDir string) (string, []string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("无法打开压缩包: %w", err)
	}
	defer r.Close()

	var luaPath string
	var manifestFiles []string

	for _, f := range r.File {
		// 跳过目录
		if f.FileInfo().IsDir() {
			continue
		}

		fileName := filepath.Base(f.Name)
		destPath := filepath.Join(destDir, fileName)

		// 创建目标文件
		outFile, err := os.Create(destPath)
		if err != nil {
			return "", nil, fmt.Errorf("创建文件 %s 失败: %w", fileName, err)
		}

		// 打开 zip 中的文件
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", nil, fmt.Errorf("读取压缩包中的 %s 失败: %w", fileName, err)
		}

		// 复制内容
		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return "", nil, fmt.Errorf("解压 %s 失败: %w", fileName, err)
		}

		// 分类文件
		lowerName := strings.ToLower(fileName)
		if strings.HasSuffix(lowerName, ".lua") {
			luaPath = destPath
		} else if strings.HasSuffix(lowerName, ".manifest") {
			manifestFiles = append(manifestFiles, destPath)
		}
	}

	if luaPath == "" {
		return "", nil, fmt.Errorf("压缩包中未找到 .lua 文件，请确认压缩包格式正确")
	}

	return luaPath, manifestFiles, nil
}

// ============================================================
// Lua 文件解析
// ============================================================

// parseLuaFile 解析 M 站格式的 Lua 文件，提取游戏信息、Depot 和 DLC 数据。
//
// Lua 文件格式示例：
//
//	-- 548430's Lua and Manifest Created by Morrenus
//	-- Deep Rock Galactic
//	addappid(548430, 1, "密钥")           → 主应用
//	addappid(548431, 1, "密钥")           → Depot（带密钥）
//	setManifestid(548431, "ManifestID", FileSize)
//	addappid(801860)                       → DLC（无密钥）
//	addappid(801860) -- DLC Name           → DLC（带注释名称）
func (a *App) parseLuaFile(luaPath string) (*GamePackage, error) {
	contentBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return nil, fmt.Errorf("读取 Lua 文件失败: %w", err)
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	gp := &GamePackage{
		LuaContent: content,
	}

	// 正则表达式
	reGameName := regexp.MustCompile(`^--\s+(.+)$`)
	reAddAppIDWithKey := regexp.MustCompile(`addappid\((\d+),\s*1,\s*"([a-f0-9]+)"\)`)
	reAddAppIDNoKey := regexp.MustCompile(`addappid\((\d+)\)\s*(?:--\s*(.*))?$`)
	reSetManifest := regexp.MustCompile(`setManifestid\((\d+),\s*"(\d+)",\s*(\d+)\)`)

	// 用于追踪哪些 AppID 有 setManifestid（即是 Depot 而非 DLC）
	depotIDs := make(map[string]bool)
	// 用于追踪已处理的 AppID（避免重复）
	processedAppIDs := make(map[string]bool)

	// 第一遍：收集所有有 setManifestid 的 DepotID
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := reSetManifest.FindStringSubmatch(line); matches != nil {
			depotIDs[matches[1]] = true
		}
	}

	// 提取游戏名称（Lua 文件的第二行注释通常是游戏名称）
	commentLineCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--") {
			break
		}
		commentLineCount++
		if commentLineCount == 2 {
			if matches := reGameName.FindStringSubmatch(line); matches != nil {
				gp.GameName = strings.TrimSpace(matches[1])
			}
		}
	}

	// 第二遍：解析所有 addappid 和 setManifestid
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过注释行和空行
		if line == "" || (strings.HasPrefix(line, "--") && !strings.Contains(line, "addappid")) {
			continue
		}

		// 匹配 addappid(AppID, 1, "密钥")
		if matches := reAddAppIDWithKey.FindStringSubmatch(line); matches != nil {
			appID := matches[1]
			key := matches[2]

			if processedAppIDs[appID] {
				continue
			}
			processedAppIDs[appID] = true

			// 提取行尾注释作为名称
			name := extractCommentName(line)

			if gp.MainAppID == "" {
				// 第一个带密钥的 addappid 是主应用
				gp.MainAppID = appID
				continue
			}

			if depotIDs[appID] {
				// 这是一个 Depot
				gp.Depots = append(gp.Depots, DepotInfo{
					DepotID:       appID,
					DecryptionKey: key,
				})
			} else {
				// 这是一个带密钥的 DLC
				gp.DLCs = append(gp.DLCs, DLCInfo{
					AppID:         appID,
					Name:          name,
					HasKey:        true,
					DecryptionKey: key,
				})
			}
			continue
		}

		// 匹配 addappid(AppID) -- 无密钥
		if matches := reAddAppIDNoKey.FindStringSubmatch(line); matches != nil {
			appID := matches[1]

			if processedAppIDs[appID] {
				continue
			}
			processedAppIDs[appID] = true

			// 跳过主应用的无密钥重复注册
			if appID == gp.MainAppID {
				continue
			}

			name := ""
			if len(matches) > 2 {
				name = strings.TrimSpace(matches[2])
			}
			if name == "" {
				name = "DLC " + appID
			}

			// 检查是否是 Depot（有些 Depot 也有无密钥的 addappid）
			if depotIDs[appID] {
				continue
			}

			gp.DLCs = append(gp.DLCs, DLCInfo{
				AppID:  appID,
				Name:   name,
				HasKey: false,
			})
			continue
		}

		// 匹配 setManifestid(DepotID, "ManifestID", FileSize)
		if matches := reSetManifest.FindStringSubmatch(line); matches != nil {
			depotID := matches[1]
			manifestID := matches[2]

			// 更新对应 Depot 的 ManifestID
			for i := range gp.Depots {
				if gp.Depots[i].DepotID == depotID {
					gp.Depots[i].ManifestID = manifestID
					break
				}
			}
			continue
		}
	}

	// 如果没有解析到游戏名称，使用主 AppID
	if gp.GameName == "" {
		gp.GameName = "游戏 " + gp.MainAppID
	}

	return gp, nil
}

// extractCommentName 从 Lua 行中提取行尾注释作为名称。
// 例如：addappid(801860) -- Deep Rock Galactic - Supporter Upgrade
// 返回："Deep Rock Galactic - Supporter Upgrade"
func extractCommentName(line string) string {
	idx := strings.Index(line, "-- ")
	if idx == -1 {
		return ""
	}
	name := strings.TrimSpace(line[idx+3:])
	// 去掉 "Depot XXXXX" 这种格式
	if strings.HasPrefix(name, "Depot ") {
		return ""
	}
	return name
}

// ============================================================
// DLC 检测
// ============================================================

// detectInstalledDLCs 扫描 Steam 配置文件，检测哪些 DLC 已经安装。
func (a *App) detectInstalledDLCs(gp *GamePackage) {
	// 读取 Steamtools.lua
	luaPath := filepath.Join(a.steamPath, "config", "stplug-in", "Steamtools.lua")
	luaBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return // 文件不存在，说明没有安装任何 DLC
	}
	luaContent := string(luaBytes)

	// 读取 config.vdf
	vdfPath := filepath.Join(a.steamPath, "config", "config.vdf")
	vdfBytes, err := os.ReadFile(vdfPath)
	if err != nil {
		return
	}
	vdfContent := string(vdfBytes)

	// 检查每个 DLC 是否已安装
	for i := range gp.DLCs {
		dlc := &gp.DLCs[i]

		// 在 Steamtools.lua 中检查 addappid(AppID)
		if strings.Contains(luaContent, "addappid("+dlc.AppID+")") ||
			strings.Contains(luaContent, "addappid("+dlc.AppID+",") {
			dlc.IsInstalled = true
			continue
		}

		// 在 config.vdf 中检查 "AppID" 条目
		if strings.Contains(vdfContent, `"`+dlc.AppID+`"`) {
			dlc.IsInstalled = true
		}
	}
}

// ============================================================
// Steam 进程管理
// ============================================================

// killSteam 终止 Steam 进程
func (a *App) killSteam() {
	cmd := exec.Command("taskkill", "/F", "/IM", "steam.exe")
	cmd.Run() // 忽略错误；Steam 可能未运行
}

// ============================================================
// 安装操作
// ============================================================

// copyManifests 将 Manifest 文件复制到 Steam 的 depotcache 目录。
// 复制前会清理同 DepotID 的旧版本 manifest。
func (a *App) copyManifests(gp *GamePackage, selectedSet map[string]bool) error {
	destDir := filepath.Join(a.steamPath, "depotcache")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建 depotcache 目录失败: %w", err)
	}

	// 收集所有需要的 DepotID（来自选中的 DLC 和主应用的 Depot）
	neededDepotIDs := make(map[string]bool)
	for _, depot := range gp.Depots {
		neededDepotIDs[depot.DepotID] = true
	}

	for _, manifestPath := range gp.ManifestFiles {
		fileName := filepath.Base(manifestPath)
		parts := strings.Split(fileName, "_")
		if len(parts) < 2 {
			continue
		}
		depotID := parts[0]

		// 清理旧版本
		destEntries, err := os.ReadDir(destDir)
		if err == nil {
			for _, de := range destEntries {
				if de.IsDir() {
					continue
				}
				destFileName := de.Name()
				if strings.HasPrefix(destFileName, depotID+"_") &&
					strings.HasSuffix(strings.ToLower(destFileName), ".manifest") {
					oldPath := filepath.Join(destDir, destFileName)
					os.Remove(oldPath)
				}
			}
		}

		// 复制新文件
		srcFile, err := os.Open(manifestPath)
		if err != nil {
			continue
		}

		destPath := filepath.Join(destDir, fileName)
		destFile, err := os.Create(destPath)
		if err != nil {
			srcFile.Close()
			continue
		}

		io.Copy(destFile, srcFile)
		destFile.Close()
		srcFile.Close()
	}

	return nil
}

// patchConfigVDF 修改 config.vdf，在 depots 部分添加解密密钥。
func (a *App) patchConfigVDF(gp *GamePackage, selectedSet map[string]bool) error {
	vdfPath := filepath.Join(a.steamPath, "config", "config.vdf")

	contentBytes, err := os.ReadFile(vdfPath)
	if err != nil {
		return fmt.Errorf("读取 config.vdf 失败: %w", err)
	}
	content := string(contentBytes)

	// 备份原始文件
	backupPath := vdfPath + ".bak"
	os.WriteFile(backupPath, contentBytes, 0644)

	modified := false

	// 添加所有 Depot 的解密密钥
	for _, depot := range gp.Depots {
		keyCheck := fmt.Sprintf(`"%s"`, depot.DepotID)

		// 幂等性检查：如果已存在则跳过
		if strings.Contains(content, keyCheck) {
			continue
		}

		// 定位 "depots" 部分
		depotsIndex := strings.Index(content, `"depots"`)
		if depotsIndex == -1 {
			return fmt.Errorf("在 config.vdf 中找不到 \"depots\" 节点")
		}

		// 找到 "depots" 之后的开括号
		openBraceIndex := strings.Index(content[depotsIndex:], "{")
		if openBraceIndex == -1 {
			return fmt.Errorf("找不到 depots 的起始括号")
		}

		// 构建 VDF 块
		vdfBlock := fmt.Sprintf(`
					"%s"
					{
						"DecryptionKey"		"%s"
					}`, depot.DepotID, depot.DecryptionKey)

		// 在开括号之后插入
		insertIndex := depotsIndex + openBraceIndex + 1
		content = content[:insertIndex] + vdfBlock + content[insertIndex:]
		modified = true
	}

	// 添加带密钥的 DLC 的解密密钥
	for _, dlc := range gp.DLCs {
		if !dlc.HasKey || !selectedSet[dlc.AppID] {
			continue
		}

		keyCheck := fmt.Sprintf(`"%s"`, dlc.AppID)
		if strings.Contains(content, keyCheck) {
			continue
		}

		depotsIndex := strings.Index(content, `"depots"`)
		if depotsIndex == -1 {
			return fmt.Errorf("在 config.vdf 中找不到 \"depots\" 节点")
		}

		openBraceIndex := strings.Index(content[depotsIndex:], "{")
		if openBraceIndex == -1 {
			return fmt.Errorf("找不到 depots 的起始括号")
		}

		vdfBlock := fmt.Sprintf(`
					"%s"
					{
						"DecryptionKey"		"%s"
					}`, dlc.AppID, dlc.DecryptionKey)

		insertIndex := depotsIndex + openBraceIndex + 1
		content = content[:insertIndex] + vdfBlock + content[insertIndex:]
		modified = true
	}

	if modified {
		return os.WriteFile(vdfPath, []byte(content), 0644)
	}
	return nil
}

// patchSteamtoolsLua 修改 Steamtools.lua，添加 addappid() 调用。
func (a *App) patchSteamtoolsLua(gp *GamePackage, selectedSet map[string]bool) error {
	luaPath := filepath.Join(a.steamPath, "config", "stplug-in", "Steamtools.lua")

	// 确保目录存在
	dir := filepath.Dir(luaPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 读取现有内容（如果文件不存在则创建空内容）
	var content string
	if contentBytes, err := os.ReadFile(luaPath); err == nil {
		content = string(contentBytes)
	}

	// 备份
	if content != "" {
		os.WriteFile(luaPath+".bak", []byte(content), 0644)
	}

	var linesToAdd []string

	// 添加主应用
	mainLine := fmt.Sprintf("addappid(%s)", gp.MainAppID)
	if !strings.Contains(content, "addappid("+gp.MainAppID+")") &&
		!strings.Contains(content, "addappid("+gp.MainAppID+",") {
		linesToAdd = append(linesToAdd, mainLine)
	}

	// 添加所有 Depot（带密钥）
	for _, depot := range gp.Depots {
		line := fmt.Sprintf(`addappid(%s, 1, "%s")`, depot.DepotID, depot.DecryptionKey)
		if !strings.Contains(content, "addappid("+depot.DepotID+",") &&
			!strings.Contains(content, "addappid("+depot.DepotID+")") {
			linesToAdd = append(linesToAdd, line)
		}
	}

	// 添加选中的 DLC
	for _, dlc := range gp.DLCs {
		if !selectedSet[dlc.AppID] {
			continue
		}

		var line string
		if dlc.HasKey {
			line = fmt.Sprintf(`addappid(%s, 1, "%s")`, dlc.AppID, dlc.DecryptionKey)
		} else {
			line = fmt.Sprintf("addappid(%s)", dlc.AppID)
		}

		if !strings.Contains(content, "addappid("+dlc.AppID+",") &&
			!strings.Contains(content, "addappid("+dlc.AppID+")") {
			linesToAdd = append(linesToAdd, line)
		}
	}

	if len(linesToAdd) == 0 {
		return nil // 所有条目已存在
	}

	// 确保文件以换行符结尾
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// 追加新行
	content += strings.Join(linesToAdd, "\n") + "\n"

	return os.WriteFile(luaPath, []byte(content), 0644)
}

// ============================================================
// 卸载操作
// ============================================================

// collectAllAppIDs 收集游戏包中所有相关的 AppID
func (a *App) collectAllAppIDs(gp *GamePackage) []string {
	idSet := make(map[string]bool)

	// 主应用
	idSet[gp.MainAppID] = true

	// 所有 Depot
	for _, depot := range gp.Depots {
		idSet[depot.DepotID] = true
	}

	// 所有 DLC
	for _, dlc := range gp.DLCs {
		idSet[dlc.AppID] = true
	}

	var ids []string
	for id := range idSet {
		ids = append(ids, id)
	}
	return ids
}

// removeManifests 从 depotcache 中删除指定 AppID 的 manifest 文件
func (a *App) removeManifests(appIDs []string) {
	depotcachePath := filepath.Join(a.steamPath, "depotcache")

	entries, err := os.ReadDir(depotcachePath)
	if err != nil {
		return
	}

	idSet := make(map[string]bool)
	for _, id := range appIDs {
		idSet[id] = true
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fileName := e.Name()
		if !strings.HasSuffix(strings.ToLower(fileName), ".manifest") {
			continue
		}

		parts := strings.Split(fileName, "_")
		if len(parts) < 2 {
			continue
		}

		if idSet[parts[0]] {
			filePath := filepath.Join(depotcachePath, fileName)
			os.Remove(filePath)
		}
	}
}

// unpatchConfigVDF 从 config.vdf 中移除指定游戏的所有密钥条目
func (a *App) unpatchConfigVDF(gp *GamePackage) error {
	vdfPath := filepath.Join(a.steamPath, "config", "config.vdf")

	contentBytes, err := os.ReadFile(vdfPath)
	if err != nil {
		return fmt.Errorf("读取 config.vdf 失败: %w", err)
	}
	content := string(contentBytes)

	// 备份
	os.WriteFile(vdfPath+".bak.remove", contentBytes, 0644)

	modified := false

	// 移除所有 Depot 的密钥块
	for _, depot := range gp.Depots {
		block := fmt.Sprintf(`
					"%s"
					{
						"DecryptionKey"		"%s"
					}`, depot.DepotID, depot.DecryptionKey)

		if strings.Contains(content, block) {
			content = strings.ReplaceAll(content, block, "")
			modified = true
		}
	}

	// 移除所有带密钥的 DLC 的密钥块
	for _, dlc := range gp.DLCs {
		if !dlc.HasKey {
			continue
		}
		block := fmt.Sprintf(`
					"%s"
					{
						"DecryptionKey"		"%s"
					}`, dlc.AppID, dlc.DecryptionKey)

		if strings.Contains(content, block) {
			content = strings.ReplaceAll(content, block, "")
			modified = true
		}
	}

	if modified {
		return os.WriteFile(vdfPath, []byte(content), 0644)
	}
	return nil
}

// unpatchSteamtoolsLua 从 Steamtools.lua 中移除指定游戏的所有 addappid 调用
func (a *App) unpatchSteamtoolsLua(gp *GamePackage) error {
	luaPath := filepath.Join(a.steamPath, "config", "stplug-in", "Steamtools.lua")

	contentBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return nil // 文件不存在，无需清理
	}
	content := string(contentBytes)

	// 备份
	os.WriteFile(luaPath+".bak.remove", contentBytes, 0644)

	// 收集所有要移除的行
	var removePatterns []string

	// 主应用
	removePatterns = append(removePatterns, fmt.Sprintf("addappid(%s)", gp.MainAppID))

	// 所有 Depot
	for _, depot := range gp.Depots {
		removePatterns = append(removePatterns,
			fmt.Sprintf(`addappid(%s, 1, "%s")`, depot.DepotID, depot.DecryptionKey))
	}

	// 所有 DLC
	for _, dlc := range gp.DLCs {
		if dlc.HasKey {
			removePatterns = append(removePatterns,
				fmt.Sprintf(`addappid(%s, 1, "%s")`, dlc.AppID, dlc.DecryptionKey))
		}
		removePatterns = append(removePatterns,
			fmt.Sprintf("addappid(%s)", dlc.AppID))
	}

	// 逐行过滤
	lines := strings.Split(content, "\n")
	var newLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		shouldRemove := false
		for _, pattern := range removePatterns {
			if strings.Contains(trimmed, pattern) {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			newLines = append(newLines, line)
		}
	}

	// 清理多余的空行
	newContent := strings.Join(newLines, "\n")
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	return os.WriteFile(luaPath, []byte(newContent), 0644)
}
