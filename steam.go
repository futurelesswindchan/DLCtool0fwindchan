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
// 重点改进：
// - 跳过注释掉的 addappid 调用（特别是 EXCLUDED DLCS 块）
// - 过滤无 manifest 和密钥的空 Depot
// parseLuaFile 解析 M 站格式的 Lua 文件
func (a *App) parseLuaFile(luaPath string) (*GamePackage, error) {
	contentBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return nil, fmt.Errorf("读取 Lua 文件失败: %w", err)
	}
	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	gp := &GamePackage{
		LuaContent: content,
		Depots:     []DepotInfo{},
		DLCs:       []DLCInfo{},
	}

	// 正则表达式
	reAddAppIDWithKey := regexp.MustCompile(`addappid\((\d+),\s*1,\s*"([a-f0-9]+)"\)`)
	reAddAppIDNoKey := regexp.MustCompile(`addappid\((\d+)\)\s*(?:--\s*(.*))?$`)
	reSetManifest := regexp.MustCompile(`setManifestid\((\d+),\s*"(\d+)",\s*(\d+)\)`)

	// 临时存储，用于合并多次出现的数据
	tempDLCs := make(map[string]*DLCInfo)
	tempDepots := make(map[string]*DepotInfo)
	appIDOrder := []string{} // 保持解析顺序

	inExcludedBlock := false

	// 单遍扫描处理所有逻辑
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 1. 处理块注释和排除块
		if strings.HasPrefix(line, "--") {
			if strings.Contains(line, "EXCLUDED DLCS") || strings.Contains(line, "EMPTY DEPOTS") {
				inExcludedBlock = true
			}
			// 提取游戏名称（通常是第二行）
			if gp.GameName == "" && strings.Contains(line, "-- ") {
				// 简单的 heuristic：寻找不含日期和网址的前几行注释
				if !strings.Contains(line, "Created") && !strings.Contains(line, "Website") && !strings.Contains(line, "Total") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "-- "))
					if name != "" && !strings.Contains(name, "'s Lua") {
						gp.GameName = name
					}
				}
			}
			continue
		} else if line == "" {
			inExcludedBlock = false // 空行重置排除块状态
			continue
		}

		if inExcludedBlock {
			continue
		}

		// 2. 解析带密钥的 addappid
		if matches := reAddAppIDWithKey.FindStringSubmatch(line); matches != nil {
			id := matches[1]
			key := matches[2]
			name := extractCommentName(line)

			// 记录主 AppID
			if gp.MainAppID == "" {
				gp.MainAppID = id
				continue
			}

			// 更新或创建 Depot 信息
			if _, ok := tempDepots[id]; !ok {
				tempDepots[id] = &DepotInfo{DepotID: id}
				appIDOrder = append(appIDOrder, id)
			}
			tempDepots[id].DecryptionKey = key

			// 如果不是主应用，也可能是带密钥的 DLC
			if _, ok := tempDLCs[id]; !ok {
				tempDLCs[id] = &DLCInfo{AppID: id, Name: name}
			}
			tempDLCs[id].HasKey = true
			tempDLCs[id].DecryptionKey = key
			if name != "" && (tempDLCs[id].Name == "" || strings.HasPrefix(tempDLCs[id].Name, "DLC ")) {
				tempDLCs[id].Name = name
			}
		}

		// 3. 解析无密钥的 addappid (通常是 DLC 注册)
		if matches := reAddAppIDNoKey.FindStringSubmatch(line); matches != nil {
			id := matches[1]
			name := strings.TrimSpace(matches[2])

			if id == gp.MainAppID {
				continue
			}

			if _, ok := tempDLCs[id]; !ok {
				tempDLCs[id] = &DLCInfo{AppID: id, Name: name}
				if !contains(appIDOrder, id) {
					appIDOrder = append(appIDOrder, id)
				}
			}
			if name != "" {
				tempDLCs[id].Name = name
			}
		}

		// 4. 解析 setManifestid
		if matches := reSetManifest.FindStringSubmatch(line); matches != nil {
			id := matches[1]
			mid := matches[2]

			if _, ok := tempDepots[id]; !ok {
				tempDepots[id] = &DepotInfo{DepotID: id}
				if !contains(appIDOrder, id) {
					appIDOrder = append(appIDOrder, id)
				}
			}
			tempDepots[id].ManifestID = mid
		}
	}

	// 5. 组装结果并进行最终过滤
	processedDLCs := make(map[string]bool)
	for _, id := range appIDOrder {
		// 添加有效的 Depot (必须有 Key 和 Manifest)
		if d, ok := tempDepots[id]; ok && d.DecryptionKey != "" && d.ManifestID != "" {
			gp.Depots = append(gp.Depots, *d)
		}

		// 添加 DLC (排除主 App，排除已处理)
		if dlc, ok := tempDLCs[id]; ok && id != gp.MainAppID && !processedDLCs[id] {
			if dlc.Name == "" {
				dlc.Name = "DLC " + id
			}
			gp.DLCs = append(gp.DLCs, *dlc)
			processedDLCs[id] = true
		}
	}

	if gp.GameName == "" {
		gp.GameName = "游戏 " + gp.MainAppID
	}

	// 检查解析是否彻底失败
	if gp.MainAppID == "" || (len(gp.DLCs) == 0 && len(gp.Depots) == 0) {
		return nil, fmt.Errorf("解析结果为空或格式不正确，请检查 LUA 文件内容")
	}

	return gp, nil
}

// 辅助函数：判断切片是否包含字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// extractCommentName 提取行尾名
func extractCommentName(line string) string {
	idx := strings.Index(line, "-- ")
	if idx == -1 {
		return ""
	}
	name := strings.TrimSpace(line[idx+3:])
	// 过滤掉 Morrenus 自动生成的 "Depot XXXXX" 占位符
	if strings.HasPrefix(name, "Depot ") && len(name) < 15 {
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
