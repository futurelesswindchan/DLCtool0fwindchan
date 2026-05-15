// steam.go
//
// 本文件包含与 Steam 配置文件交互的所有底层操作函数，包括：
//   - Zip 文件解压（unzipFile）
//   - Lua 文件解析（parseLuaFile）
//   - DLC 安装状态检测（detectInstalledDLCs）
//   - Steam 进程管理（killSteam）
//   - 配置文件的写入与回滚（patch/unpatch 系列函数）
//   - Manifest 文件的复制与删除
//
// 这些函数均为 App 的私有方法，不直接暴露给前端，
// 由 app.go 中的公开方法编排调用。

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
//
// 预期的 zip 格式（M 站标准）：所有文件位于根目录，包含一个 .lua 和若干 .manifest 文件。
// 解压时使用 filepath.Base 提取文件名，忽略 zip 内部的目录结构。
//
// 参数：
//   - zipPath: zip 文件的完整路径
//   - destDir: 解压目标目录（应为临时目录）
//
// 返回值：
//   - string:   解压后 Lua 文件的完整路径
//   - []string: 解压后所有 manifest 文件的完整路径列表
//   - error:    zip 格式异常或解压失败时返回错误
func (a *App) unzipFile(zipPath string, destDir string) (string, []string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", nil, fmt.Errorf("无法打开压缩包: %w", err)
	}
	defer r.Close()

	var luaPath string
	var manifestFiles []string

	for _, f := range r.File {
		// 跳过目录条目
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

		// 打开 zip 中的文件流
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return "", nil, fmt.Errorf("读取压缩包中的 %s 失败: %w", fileName, err)
		}

		// 复制内容到目标文件
		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if err != nil {
			return "", nil, fmt.Errorf("解压 %s 失败: %w", fileName, err)
		}

		// 按扩展名分类文件
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
// 解析策略：单遍扫描，通过正则表达式匹配以下三种关键调用：
//   - addappid(id, 1, "key")  —— 带解密密钥的 Depot/DLC 注册
//   - addappid(id)            —— 无密钥的 DLC 注册
//   - setManifestid(id, "mid", size) —— Manifest 关联
//
// 特殊处理：
//   - 跳过 "EXCLUDED DLCS" 和 "EMPTY DEPOTS" 注释块中的内容
//   - 第一个 addappid 调用的参数被识别为主游戏 AppID
//   - 游戏名称从文件头部注释中启发式提取
//
// 参数：
//   - luaPath: Lua 文件的完整路径
//
// 返回值：
//   - *GamePackage: 解析后的游戏数据包（不含 ManifestFiles，需外部填充）
//   - error:       文件读取失败或解析结果为空时返回错误
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

	// 正则表达式：匹配三种关键函数调用
	reAddAppIDWithKey := regexp.MustCompile(`addappid\((\d+),\s*1,\s*"([a-f0-9]+)"\)`)
	reAddAppIDNoKey := regexp.MustCompile(`addappid\((\d+)\)\s*(?:--\s*(.*))?$`)
	reSetManifest := regexp.MustCompile(`setManifestid\((\d+),\s*"(\d+)",\s*(\d+)\)`)

	// 临时存储，用于合并同一 ID 多次出现的数据
	tempDLCs := make(map[string]*DLCInfo)
	tempDepots := make(map[string]*DepotInfo)
	appIDOrder := []string{} // 保持解析顺序，确保输出稳定

	inExcludedBlock := false

	// 单遍扫描处理所有逻辑
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 1. 处理注释行和排除块标记
		if strings.HasPrefix(line, "--") {
			if strings.Contains(line, "EXCLUDED DLCS") || strings.Contains(line, "EMPTY DEPOTS") {
				inExcludedBlock = true
			}
			// 启发式提取游戏名称（通常位于文件头部注释）
			if gp.GameName == "" && strings.Contains(line, "-- ") {
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

		// 跳过排除块中的内容
		if inExcludedBlock {
			continue
		}

		// 2. 解析带密钥的 addappid(id, 1, "key")
		if matches := reAddAppIDWithKey.FindStringSubmatch(line); matches != nil {
			id := matches[1]
			key := matches[2]
			name := extractCommentName(line)

			// 第一个 addappid 调用的参数为主游戏 AppID
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

			// 同时记录为带密钥的 DLC
			if _, ok := tempDLCs[id]; !ok {
				tempDLCs[id] = &DLCInfo{AppID: id, Name: name}
			}
			tempDLCs[id].HasKey = true
			tempDLCs[id].DecryptionKey = key
			if name != "" && (tempDLCs[id].Name == "" || strings.HasPrefix(tempDLCs[id].Name, "DLC ")) {
				tempDLCs[id].Name = name
			}
		}

		// 3. 解析无密钥的 addappid(id)（通常是纯 DLC 注册）
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

		// 4. 解析 setManifestid(depotID, "manifestID", size)
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

	// 5. 按解析顺序组装最终结果，并过滤无效条目
	processedDLCs := make(map[string]bool)
	for _, id := range appIDOrder {
		// 添加有效的 Depot（必须同时具备解密密钥和 ManifestID）
		if d, ok := tempDepots[id]; ok && d.DecryptionKey != "" && d.ManifestID != "" {
			gp.Depots = append(gp.Depots, *d)
		}

		// 添加 DLC（排除主 App，排除已处理的重复项）
		if dlc, ok := tempDLCs[id]; ok && id != gp.MainAppID && !processedDLCs[id] {
			if dlc.Name == "" {
				dlc.Name = "DLC " + id
			}
			gp.DLCs = append(gp.DLCs, *dlc)
			processedDLCs[id] = true
		}
	}

	// 游戏名称兜底：若启发式提取失败，使用 AppID 作为默认名称
	if gp.GameName == "" {
		gp.GameName = "游戏 " + gp.MainAppID
	}

	// 最终校验：确保解析结果不为空
	if gp.MainAppID == "" || (len(gp.DLCs) == 0 && len(gp.Depots) == 0) {
		return nil, fmt.Errorf("解析结果为空或格式不正确，请检查 LUA 文件内容")
	}

	return gp, nil
}

// contains 判断字符串切片中是否包含指定元素。
//
// 用于 appIDOrder 的去重检查，避免同一 ID 被重复追加到顺序列表中。
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// extractCommentName 从代码行尾部的注释中提取名称信息。
//
// 示例输入：addappid(12345, 1, "abcdef") -- My DLC Name
// 示例输出："My DLC Name"
//
// 过滤规则：自动忽略 M 站生成器产生的 "Depot XXXXX" 占位符注释。
func extractCommentName(line string) string {
	idx := strings.Index(line, "-- ")
	if idx == -1 {
		return ""
	}
	name := strings.TrimSpace(line[idx+3:])
	// 过滤 M 站自动生成的 "Depot XXXXX" 占位符
	if strings.HasPrefix(name, "Depot ") && len(name) < 15 {
		return ""
	}
	return name
}

// ============================================================
// DLC 安装状态检测
// ============================================================

// detectInstalledDLCs 扫描 Steam 配置文件，检测哪些 DLC 已经安装。
//
// 检测逻辑：
//   1. 在 Steamtools.lua 中查找 addappid(AppID) 或 addappid(AppID, ...) 调用
//   2. 在 config.vdf 中查找 "AppID" 字符串（带引号匹配，降低误判）
//
// 任一条件命中即标记 DLC 为已安装状态。
// 若配置文件不存在（如首次使用），则所有 DLC 均标记为未安装。
//
// 已知局限：
//   config.vdf 中的字符串包含检查仍可能产生误判（如 AppID 出现在无关节点中）。
func (a *App) detectInstalledDLCs(gp *GamePackage) {
	// 读取 Steamtools.lua
	luaBytes, err := os.ReadFile(a.steamtoolsLuaPath())
	if err != nil {
		return // 文件不存在，说明没有安装任何 DLC
	}
	luaContent := string(luaBytes)

	// 读取 config.vdf
	vdfBytes, err := os.ReadFile(a.configVDFPath())
	if err != nil {
		return
	}
	vdfContent := string(vdfBytes)

	// 逐个检查每个 DLC 的安装状态
	for i := range gp.DLCs {
		dlc := &gp.DLCs[i]

		// 优先在 Steamtools.lua 中检查（更精确）
		if strings.Contains(luaContent, "addappid("+dlc.AppID+")") ||
			strings.Contains(luaContent, "addappid("+dlc.AppID+",") {
			dlc.IsInstalled = true
			continue
		}

		// 在 config.vdf 中检查（带引号匹配以降低误判概率）
		if strings.Contains(vdfContent, `"`+dlc.AppID+`"`) {
			dlc.IsInstalled = true
		}
	}
}

// ============================================================
// Steam 进程管理
// ============================================================

// KillSteamResult 表示关闭 Steam 操作的结果状态。
type KillSteamResult int

const (
	// SteamKilled 表示 Steam 进程已被成功终止。
	SteamKilled KillSteamResult = iota
	// SteamNotRunning 表示 Steam 进程未在运行，无需关闭。
	SteamNotRunning
	// SteamKillFailed 表示尝试关闭 Steam 失败（可能是权限不足等原因）。
	SteamKillFailed
)

// killSteam 尝试终止 Steam 进程，并返回分级结果。
//
// 通过 tasklist 先检测 Steam 是否在运行，再决定是否执行 taskkill。
// 这样可以区分"本来就没开"和"关闭失败"两种情况，
// 为上层调用方提供更精确的状态反馈。
//
// 返回值：
//   - KillSteamResult: 操作结果枚举
//   - error:           仅在 taskkill 执行出错时返回具体错误信息
func (a *App) killSteam() (KillSteamResult, error) {
	// 先检测 Steam 是否在运行
	checkCmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+SteamProcessName)
	output, err := checkCmd.Output()
	if err != nil {
		// tasklist 本身执行失败，无法判断状态，尝试直接 kill
		killCmd := exec.Command("taskkill", "/F", "/IM", SteamProcessName)
		if killErr := killCmd.Run(); killErr != nil {
			return SteamKillFailed, fmt.Errorf("无法确认 Steam 状态且关闭失败: %w", killErr)
		}
		return SteamKilled, nil
	}

	// 检查 tasklist 输出中是否包含 steam.exe
	if !strings.Contains(strings.ToLower(string(output)), strings.ToLower(SteamProcessName)) {
		// Steam 未运行，无需关闭
		return SteamNotRunning, nil
	}

	// Steam 正在运行，执行强制关闭
	killCmd := exec.Command("taskkill", "/F", "/IM", SteamProcessName)
	if err := killCmd.Run(); err != nil {
		return SteamKillFailed, fmt.Errorf("关闭 Steam 失败（可能权限不足）: %w", err)
	}

	return SteamKilled, nil
}

// ============================================================
// Manifest 文件操作
// ============================================================

// copyManifests 将解压后的 manifest 文件复制到 Steam 的 depotcache 目录。
//
// 复制前会清理目标目录中同 DepotID 的旧版本 manifest，确保不会残留过期文件。
// manifest 文件命名格式为 <DepotID>_<ManifestID>.manifest。
//
// 参数：
//   - gp:          游戏数据包（包含 ManifestFiles 路径列表）
//   - selectedSet: 用户选中的 DLC AppID 集合（当前未用于过滤，预留扩展）
//
// 返回值：
//   - error: depotcache 目录创建失败时返回错误；单个文件复制失败会被静默跳过
//
// 已知局限：
//   单个文件复制失败时使用 continue 跳过，外层无法精确知道哪些文件成功/失败。
func (a *App) copyManifests(gp *GamePackage, selectedSet map[string]bool) error {
	destDir := a.depotcachePath()
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

		// 清理同 DepotID 的旧版本 manifest
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

		// 复制新的 manifest 文件
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

// ============================================================
// config.vdf 写入与回滚
// ============================================================

// patchConfigVDF 修改 config.vdf，在 depots 节点中添加解密密钥。
//
// 写入策略：
//   1. 定位 "depots" 节点的起始大括号
//   2. 在大括号之后插入新的 VDF 键值块
//   3. 通过字符串包含检查实现幂等性（已存在的 ID 不会重复写入）
//
// 写入前会创建 .bak 备份文件。
//
// 参数：
//   - gp:          游戏数据包
//   - selectedSet: 用户选中的 DLC AppID 集合
//
// 返回值：
//   - error: 文件读写失败或 depots 节点定位失败时返回错误
//
// 已知局限：
//   对 config.vdf 的原始排版和节点结构高度敏感，
//   若文件格式与预期差异较大，插入位置可能不准确。
func (a *App) patchConfigVDF(gp *GamePackage, selectedSet map[string]bool) error {
	vdfPath := a.configVDFPath()

	contentBytes, err := os.ReadFile(vdfPath)
	if err != nil {
		return fmt.Errorf("读取 config.vdf 失败: %w", err)
	}
	content := string(contentBytes)

	// 备份原始文件
	backupPath := vdfPath + BackupSuffix
	os.WriteFile(backupPath, contentBytes, 0644)

	modified := false

	// 添加所有 Depot 的解密密钥
	for _, depot := range gp.Depots {
		keyCheck := fmt.Sprintf(`"%s"`, depot.DepotID)

		// 幂等性检查：如果已存在则跳过
		if strings.Contains(content, keyCheck) {
			continue
		}

		// 定位 "depots" 节点
		depotsIndex := strings.Index(content, `"depots"`)
		if depotsIndex == -1 {
			return fmt.Errorf("在 config.vdf 中找不到 \"depots\" 节点")
		}

		// 找到 "depots" 之后的开括号
		openBraceIndex := strings.Index(content[depotsIndex:], "{")
		if openBraceIndex == -1 {
			return fmt.Errorf("找不到 depots 的起始括号")
		}

		// 构建 VDF 格式的密钥块
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

// ============================================================
// Steamtools.lua 写入与回滚
// ============================================================

// patchSteamtoolsLua 修改 Steamtools.lua，追加 addappid() 调用。
//
// 写入策略：
//   1. 确保 stplug-in 目录存在
//   2. 读取现有文件内容（不存在则视为空文件）
//   3. 通过 strings.Contains 检查实现幂等性
//   4. 将新增行追加到文件末尾
//
// 写入前会创建 .bak 备份文件。
//
// 参数：
//   - gp:          游戏数据包
//   - selectedSet: 用户选中的 DLC AppID 集合
//
// 返回值：
//   - error: 目录创建或文件写入失败时返回错误
//
// 已知局限：
//   幂等性检查基于简单字符串包含，对空格、注释等格式变体不够容错。
func (a *App) patchSteamtoolsLua(gp *GamePackage, selectedSet map[string]bool) error {
	luaPath := a.steamtoolsLuaPath()

	// 确保目录存在
	if err := os.MkdirAll(a.steamtoolsLuaDir(), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 读取现有内容（如果文件不存在则创建空内容）
	var content string
	if contentBytes, err := os.ReadFile(luaPath); err == nil {
		content = string(contentBytes)
	}

	// 备份现有文件
	if content != "" {
		os.WriteFile(luaPath+BackupSuffix, []byte(content), 0644)
	}

	var linesToAdd []string

	// 添加主应用的 addappid 调用
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
		return nil // 所有条目已存在，无需修改
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

// collectAllAppIDs 收集游戏包中所有相关的 AppID（主应用 + Depot + DLC）。
//
// 返回的切片用于 removeManifests 等批量清理操作。
// 使用 map 去重后转为切片返回。
//
// 已知局限：
//   返回顺序不确定（map 遍历无序），若需要稳定输出应额外排序。
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

// removeManifests 从 depotcache 目录中删除指定 AppID 的 manifest 文件。
//
// 遍历 depotcache 目录，将文件名前缀匹配到 appIDs 集合中的文件全部删除。
// 与之前静默忽略不同，现在会收集所有删除失败的错误并返回，
// 让调用方可以决定是否告知用户。
//
// 参数：
//   - appIDs: 需要清理的 AppID 列表
//
// 返回值：
//   - []error: 删除失败的错误列表；全部成功时返回 nil
func (a *App) removeManifests(appIDs []string) []error {
	depotcachePath := a.depotcachePath()

	entries, err := os.ReadDir(depotcachePath)
	if err != nil {
		return []error{fmt.Errorf("读取 depotcache 目录失败: %w", err)}
	}

	idSet := make(map[string]bool)
	for _, id := range appIDs {
		idSet[id] = true
	}

	var errs []error
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
			if err := os.Remove(filePath); err != nil {
				errs = append(errs, fmt.Errorf("删除 %s 失败: %w", fileName, err))
			}
		}
	}

	return errs
}

// unpatchConfigVDF 从 config.vdf 中移除指定游戏的所有密钥条目。
//
// 移除策略：
//   构建与写入时完全相同格式的 VDF 块文本，通过 strings.ReplaceAll 精确删除。
//   写入前会创建 .bak.remove 备份文件。
//
// 参数：
//   - gp: 游戏数据包（用于确定需要移除的 Depot 和 DLC 密钥块）
//
// 返回值：
//   - error: 文件读写失败时返回错误
//
// 已知局限：
//   依赖与写入时完全一致的文本格式进行匹配删除。
//   若 config.vdf 被手动编辑导致缩进/空格变化，可能无法匹配成功。
func (a *App) unpatchConfigVDF(gp *GamePackage) error {
	vdfPath := a.configVDFPath()

	contentBytes, err := os.ReadFile(vdfPath)
	if err != nil {
		return fmt.Errorf("读取 config.vdf 失败: %w", err)
	}
	content := string(contentBytes)

	// 备份
	os.WriteFile(vdfPath+BackupRemoveSuffix, contentBytes, 0644)

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

// unpatchSteamtoolsLua 从 Steamtools.lua 中移除指定游戏的所有 addappid 调用。
//
// 移除策略：
//   1. 构建所有需要移除的 addappid 模式列表
//   2. 逐行扫描文件，跳过匹配到模式的行
//   3. 清理移除后产生的多余空行（连续三个以上空行压缩为两个）
//
// 写入前会创建 .bak.remove 备份文件。
//
// 参数：
//   - gp: 游戏数据包
//
// 返回值：
//   - error: 文件写入失败时返回错误；文件不存在时返回 nil（无需清理）
//
// 已知局限：
//   逐行字符串匹配对格式变体（空格、注释、参数顺序）不够容错。
func (a *App) unpatchSteamtoolsLua(gp *GamePackage) error {
	luaPath := a.steamtoolsLuaPath()

	contentBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return nil // 文件不存在，无需清理
	}
	content := string(contentBytes)

	// 备份
	os.WriteFile(luaPath+BackupRemoveSuffix, contentBytes, 0644)

	// 收集所有要移除的模式字符串
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

	// 逐行过滤：跳过匹配到移除模式的行
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

	// 清理多余的连续空行
	newContent := strings.Join(newLines, "\n")
	for strings.Contains(newContent, "\n\n\n") {
		newContent = strings.ReplaceAll(newContent, "\n\n\n", "\n\n")
	}

	return os.WriteFile(luaPath, []byte(newContent), 0644)
}
