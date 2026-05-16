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
// 安全校验：
//   - 跳过目录条目
//   - 拒绝包含路径遍历字符（..）的文件名
//   - 拒绝空文件名或以点开头的隐藏文件
//   - 仅提取 .lua 和 .manifest 文件，忽略其他类型
//   - 检测文件名冲突（同名文件会覆盖前一个）
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

		// 安全校验：拒绝空文件名
		if fileName == "" || fileName == "." {
			continue
		}

		// 安全校验：拒绝包含路径遍历字符的文件名
		if strings.Contains(f.Name, "..") {
			continue
		}

		// 安全校验：跳过隐藏文件（以点开头）
		if strings.HasPrefix(fileName, ".") {
			continue
		}

		// 仅处理 .lua 和 .manifest 文件，忽略其他类型
		lowerName := strings.ToLower(fileName)
		isLua := strings.HasSuffix(lowerName, ".lua")
		isManifest := strings.HasSuffix(lowerName, ".manifest")
		if !isLua && !isManifest {
			continue
		}

		destPath := filepath.Join(destDir, fileName)

		// 安全校验：确保最终路径仍在目标目录内（防止符号链接等绕过）
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
		if isLua {
			luaPath = destPath
		} else if isManifest {
			manifestFiles = append(manifestFiles, destPath)
		}
	}

	if luaPath == "" {
		return "", nil, fmt.Errorf("压缩包中未找到 .lua 文件，请确认压缩包格式正确")
	}

	return luaPath, manifestFiles, nil
}

// ============================================================
// DLC 安装状态检测
// ============================================================

// detectInstalledDLCs 扫描 Steam 配置文件，检测哪些 DLC 已经安装。
//
// 检测逻辑（双重来源）：
//   1. 在 Steamtools.lua 中查找 addappid(AppID) 调用（正则容错匹配）
//   2. 在 config.vdf 的 depots 节点中查找 AppID 子键（精确树状解析）
//
// 任一条件命中即标记 DLC 为已安装状态。
// 若配置文件不存在（如首次使用），则所有 DLC 均标记为未安装。
//
// VDF 检测策略：
//   优先使用 VDF 解析器精确提取 depots 节点的 key 集合（零误判）。
//   若解析失败，fallback 到 findDepotsSection 范围限定搜索（仍优于全文搜索）。
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

	// 尝试用 VDF 解析器精确提取 depots 节点的 key 集合
	depotsKeys := parseDepotsKeys(vdfContent)

	// 若解析失败，fallback 到范围限定搜索
	var depotsRange string
	if depotsKeys == nil {
		_, openIdx, closeIdx, findErr := findDepotsSection(vdfContent)
		if findErr == nil {
			depotsRange = vdfContent[openIdx : closeIdx+1]
		}
	}

	// 逐个检查每个 DLC 的安装状态
	for i := range gp.DLCs {
		dlc := &gp.DLCs[i]

		// 优先在 Steamtools.lua 中检查（正则容错匹配）
		if luaContainsAppID(luaContent, dlc.AppID) {
			dlc.IsInstalled = true
			continue
		}

		// 在 config.vdf 的 depots 节点中检查
		if depotsKeys != nil {
			// 精确模式：直接查 map key
			if depotsKeys[dlc.AppID] {
				dlc.IsInstalled = true
			}
		} else if depotsRange != "" {
			// Fallback 模式：范围内带引号搜索
			if strings.Contains(depotsRange, `"`+dlc.AppID+`"`) {
				dlc.IsInstalled = true
			}
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

// copyManifests 将解压后的 manifest 文件复制到 Steam 的 depotcache 目录。
//
// 复制前会清理目标目录中同 DepotID 的旧版本 manifest，确保不会残留过期文件。
// manifest 文件命名格式为 <DepotID>_<ManifestID>.manifest。
//
// 错误处理策略：
//   单个文件复制失败不会阻断整体流程，而是收集到错误列表中返回，
//   由调用方决定是否告知用户。仅 depotcache 目录创建失败视为致命错误。
//
// 参数：
//   - gp:          游戏数据包（包含 ManifestFiles 路径列表）
//   - selectedSet: 用户选中的 DLC AppID 集合（当前未用于过滤，预留扩展）
//
// 返回值：
//   - []error: 复制过程中遇到的所有错误；全部成功时返回 nil
func (a *App) copyManifests(gp *GamePackage, selectedSet map[string]bool) []error {
	destDir := a.depotcachePath()
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return []error{fmt.Errorf("创建 depotcache 目录失败: %w", err)}
	}

	// 收集所有需要的 DepotID（来自选中的 DLC 和主应用的 Depot）
	neededDepotIDs := make(map[string]bool)
	for _, depot := range gp.Depots {
		neededDepotIDs[depot.DepotID] = true
	}

	var errs []error

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
			errs = append(errs, fmt.Errorf("打开源文件 %s 失败: %w", fileName, err))
			continue
		}

		destPath := filepath.Join(destDir, fileName)
		destFile, err := os.Create(destPath)
		if err != nil {
			srcFile.Close()
			errs = append(errs, fmt.Errorf("创建目标文件 %s 失败: %w", fileName, err))
			continue
		}

		_, copyErr := io.Copy(destFile, srcFile)
		destFile.Close()
		srcFile.Close()
		if copyErr != nil {
			errs = append(errs, fmt.Errorf("复制 %s 内容失败: %w", fileName, copyErr))
		}
	}

	return errs
}

// ============================================================
// config.vdf 写入与回滚
// ============================================================

// patchConfigVDF 修改 config.vdf，在 depots 节点中添加解密密钥。
//
// 写入策略：
//   1. 用 findDepotsSection 精确定位 depots 节点的 { } 范围
//   2. 用 inferIndent 从现有内容推断缩进风格（自适应格式）
//   3. 用 buildDepotBlock 按推断缩进生成 VDF 块
//   4. 在 depots 开括号之后插入新块
//   5. 幂等性检查限定在 depots 范围内（避免其他节点的同名 key 干扰）
//
// 写入前会创建 .bak 备份文件。
//
// 参数：
//   - gp:          游戏数据包
//   - selectedSet: 用户选中的 DLC AppID 集合
//
// 返回值：
//   - error: 文件读写失败或 depots 节点定位失败时返回错误
func (a *App) patchConfigVDF(gp *GamePackage, selectedSet map[string]bool) error {
	vdfPath := a.configVDFPath()

	contentBytes, err := os.ReadFile(vdfPath)
	if err != nil {
		return fmt.Errorf("读取 config.vdf 失败: %w", err)
	}
	content := string(contentBytes)

	// 备份原始文件
	os.WriteFile(vdfPath+BackupSuffix, contentBytes, 0644)

	// 精确定位 depots 节点
	_, openIdx, closeIdx, err := findDepotsSection(content)
	if err != nil {
		return err
	}

	// 推断缩进风格
	entryIndent, innerIndent := inferIndent(content, openIdx, closeIdx)

	// 提取 depots 范围内的文本用于幂等性检查
	depotsContent := content[openIdx : closeIdx+1]

	modified := false

	// 添加所有 Depot 的解密密钥
	for _, depot := range gp.Depots {
		// 幂等性检查：仅在 depots 范围内查找
		if strings.Contains(depotsContent, `"`+depot.DepotID+`"`) {
			continue
		}

		block := buildDepotBlock(depot.DepotID, depot.DecryptionKey, entryIndent, innerIndent)

		// 在开括号之后插入
		insertIdx := openIdx + 1
		content = content[:insertIdx] + block + content[insertIdx:]

		// 插入后需要重新定位（内容已变化）
		_, openIdx, closeIdx, err = findDepotsSection(content)
		if err != nil {
			return fmt.Errorf("插入 Depot %s 后重新定位失败: %w", depot.DepotID, err)
		}
		depotsContent = content[openIdx : closeIdx+1]
		modified = true
	}

	// 添加带密钥的 DLC 的解密密钥
	for _, dlc := range gp.DLCs {
		if !dlc.HasKey || !selectedSet[dlc.AppID] {
			continue
		}

		if strings.Contains(depotsContent, `"`+dlc.AppID+`"`) {
			continue
		}

		block := buildDepotBlock(dlc.AppID, dlc.DecryptionKey, entryIndent, innerIndent)

		insertIdx := openIdx + 1
		content = content[:insertIdx] + block + content[insertIdx:]

		_, openIdx, closeIdx, err = findDepotsSection(content)
		if err != nil {
			return fmt.Errorf("插入 DLC %s 后重新定位失败: %w", dlc.AppID, err)
		}
		depotsContent = content[openIdx : closeIdx+1]
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
//   3. 通过正则匹配检查实现幂等性（容忍空格、注释等格式变体）
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
	if !luaContainsAppID(content, gp.MainAppID) {
		linesToAdd = append(linesToAdd, fmt.Sprintf("addappid(%s)", gp.MainAppID))
	}

	// 添加所有 Depot（带密钥）
	for _, depot := range gp.Depots {
		if !luaContainsAppID(content, depot.DepotID) {
			linesToAdd = append(linesToAdd,
				fmt.Sprintf(`addappid(%s, 1, "%s")`, depot.DepotID, depot.DecryptionKey))
		}
	}

	// 添加选中的 DLC
	for _, dlc := range gp.DLCs {
		if !selectedSet[dlc.AppID] {
			continue
		}

		if luaContainsAppID(content, dlc.AppID) {
			continue
		}

		if dlc.HasKey {
			linesToAdd = append(linesToAdd,
				fmt.Sprintf(`addappid(%s, 1, "%s")`, dlc.AppID, dlc.DecryptionKey))
		} else {
			linesToAdd = append(linesToAdd,
				fmt.Sprintf("addappid(%s)", dlc.AppID))
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
// 返回的切片按以下顺序排列：主应用 AppID 在最前，
// 随后是 Depot ID（按解析顺序），最后是 DLC AppID（按解析顺序）。
// 使用 map 去重确保不会出现重复 ID。
//
// 返回值：
//   - []string: 去重且有序的 AppID 列表
func (a *App) collectAllAppIDs(gp *GamePackage) []string {
	idSet := make(map[string]bool)
	var ids []string

	// 主应用排在最前
	if gp.MainAppID != "" {
		idSet[gp.MainAppID] = true
		ids = append(ids, gp.MainAppID)
	}

	// 所有 Depot（按解析顺序）
	for _, depot := range gp.Depots {
		if !idSet[depot.DepotID] {
			idSet[depot.DepotID] = true
			ids = append(ids, depot.DepotID)
		}
	}

	// 所有 DLC（按解析顺序）
	for _, dlc := range gp.DLCs {
		if !idSet[dlc.AppID] {
			idSet[dlc.AppID] = true
			ids = append(ids, dlc.AppID)
		}
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
//   使用正则匹配删除，容忍缩进和空白格式变体。
//   无论 config.vdf 被 Steam 客户端重新格式化过，还是被手动编辑过，
//   只要 "DepotID" + { + "DecryptionKey" + } 的结构存在即可匹配。
//
// 写入前会创建 .bak.remove 备份文件。
//
// 参数：
//   - gp: 游戏数据包（用于确定需要移除的 Depot 和 DLC 密钥块）
//
// 返回值：
//   - error: 文件读写失败时返回错误
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
		result, changed := removeDepotBlock(content, depot.DepotID)
		if changed {
			content = result
			modified = true
		}
	}

	// 移除所有带密钥的 DLC 的密钥块
	for _, dlc := range gp.DLCs {
		if !dlc.HasKey {
			continue
		}
		result, changed := removeDepotBlock(content, dlc.AppID)
		if changed {
			content = result
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
//   1. 收集所有需要移除的 AppID 列表
//   2. 逐行扫描文件，使用正则匹配判断是否为目标行（容忍空格和格式变体）
//   3. 跳过被注释的行（以 -- 开头），避免误删 EXCLUDED DLCS 区域
//   4. 清理移除后产生的多余空行（连续三个以上空行压缩为两个）
//
// 写入前会创建 .bak.remove 备份文件。
//
// 参数：
//   - gp: 游戏数据包
//
// 返回值：
//   - error: 文件写入失败时返回错误；文件不存在时返回 nil（无需清理）
func (a *App) unpatchSteamtoolsLua(gp *GamePackage) error {
	luaPath := a.steamtoolsLuaPath()

	contentBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return nil // 文件不存在，无需清理
	}
	content := string(contentBytes)

	// 备份
	os.WriteFile(luaPath+BackupRemoveSuffix, contentBytes, 0644)

	// 收集所有要移除的 AppID
	var removeIDs []string

	// 主应用
	removeIDs = append(removeIDs, gp.MainAppID)

	// 所有 Depot
	for _, depot := range gp.Depots {
		removeIDs = append(removeIDs, depot.DepotID)
	}

	// 所有 DLC
	for _, dlc := range gp.DLCs {
		removeIDs = append(removeIDs, dlc.AppID)
	}

	// 逐行过滤：使用正则匹配判断是否需要移除
	lines := strings.Split(content, "\n")
	var newLines []string
	for _, line := range lines {
		shouldRemove := false
		for _, id := range removeIDs {
			if luaLineMatchesAppID(line, id) {
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