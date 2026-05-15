// lua_parser.go
//
// 本文件实现基于 Lua VM 的脚本解析器，用于替代原先基于正则表达式的解析逻辑。
//
// 核心思路：
//   M 站生成的 .lua 文件本质上是可执行的 Lua 脚本，包含 addappid() 和
//   setManifestid() 两个函数调用。与其用正则去"猜"数据结构，不如直接用
//   Lua 解释器执行脚本，通过注册回调函数让脚本自己告诉我们数据是什么。
//
// 优势：
//   - 注释行天然被 Lua 解释器跳过，无需手动处理 EXCLUDED DLCS 块
//   - 不依赖任何正则、启发式规则，对格式变化完全免疫
//   - 调用顺序由 Lua 执行顺序决定，天然有序
//
// 架构分为三个阶段：
//   1. 注释头解析器：从文件头部注释中提取游戏名称等元信息
//   2. DLC 名称映射：从注释中建立 AppID → 名称的映射表
//   3. Lua VM 执行器：注册回调函数，执行脚本收集核心数据

package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// ============================================================
// 注释头解析器
// ============================================================

// CommentMetadata 存储从 Lua 文件头部注释中提取的元信息。
//
// 这些信息无法通过 Lua VM 执行获取（因为它们是注释），
// 需要单独用简单的字符串匹配来提取。
type CommentMetadata struct {
	// GameName 是游戏的显示名称（通常位于文件第二行注释）。
	GameName string
	// TotalDepots 是注释中声明的 Depot 总数（用于校验）。
	TotalDepots int
	// TotalDLCs 是注释中声明的 DLC 总数（用于校验）。
	TotalDLCs int
	// CreatedDate 是文件生成日期。
	CreatedDate string
}

// parseCommentHeader 从 Lua 文件内容的头部注释中提取元信息。
//
// 扫描策略：仅处理文件前 15 行，遇到非注释行立即停止。
// 这确保了即使文件很大，元信息提取也是 O(1) 的。
//
// 参数：
//   - content: Lua 文件的完整文本内容
//
// 返回值：
//   - *CommentMetadata: 提取到的元信息（字段可能为空，表示未找到）
func parseCommentHeader(content string) *CommentMetadata {
	meta := &CommentMetadata{}
	lines := strings.Split(content, "\n")

	maxLines := 15
	if len(lines) < maxLines {
		maxLines = len(lines)
	}

	gameNameFound := false

	for _, line := range lines[:maxLines] {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "--") {
			break // 遇到非注释行就停止
		}

		comment := strings.TrimSpace(strings.TrimPrefix(line, "--"))

		// 跳过空注释行
		if comment == "" {
			continue
		}

		// 游戏名称：第一个不含关键词的注释行（跳过第一行的 "XXXXX's Lua..."）
		if !gameNameFound {
			if strings.Contains(comment, "'s Lua") || strings.Contains(comment, "Created") {
				continue
			}
			if !strings.Contains(comment, "Website") && !strings.Contains(comment, "Total") &&
				!strings.Contains(comment, "Shared") && !strings.Contains(comment, "Depot") {
				meta.GameName = comment
				gameNameFound = true
				continue
			}
		}

		// 提取 Total Depots 数量
		if strings.HasPrefix(comment, "Total Depots:") {
			numStr := strings.TrimSpace(strings.TrimPrefix(comment, "Total Depots:"))
			if n, err := strconv.Atoi(numStr); err == nil {
				meta.TotalDepots = n
			}
		}

		// 提取 Total DLCs 数量
		if strings.HasPrefix(comment, "Total DLCs:") {
			// 格式可能是 "Total DLCs: 21 (3 excluded)"
			numStr := strings.TrimSpace(strings.TrimPrefix(comment, "Total DLCs:"))
			// 取第一个空格前的数字
			if spaceIdx := strings.Index(numStr, " "); spaceIdx != -1 {
				numStr = numStr[:spaceIdx]
			}
			if n, err := strconv.Atoi(numStr); err == nil {
				meta.TotalDLCs = n
			}
		}

		// 提取创建日期
		if strings.HasPrefix(comment, "Created:") {
			meta.CreatedDate = strings.TrimSpace(strings.TrimPrefix(comment, "Created:"))
		}
	}

	return meta
}

// ============================================================
// DLC 名称映射
// ============================================================

// buildDLCNameMap 从 Lua 文件注释中建立 AppID → DLC 名称的映射表。
//
// M 站生成的 Lua 文件中，每个 DLC 块前都有格式化的注释行：
//   -- Monster Hunter Stories 3 ... (AppID: 3581270)
//
// 本函数通过正则匹配这些注释行，提取 AppID 和对应的名称。
// 这是唯一仍需要正则的地方，但它只处理注释格式，不涉及代码逻辑。
//
// 参数：
//   - content: Lua 文件的完整文本内容
//
// 返回值：
//   - map[string]string: AppID（字符串）→ DLC 名称的映射
func buildDLCNameMap(content string) map[string]string {
	nameMap := make(map[string]string)

	// 匹配格式：-- DLC名称 (AppID: XXXXX)
	re := regexp.MustCompile(`--\s*(.+?)\s*\(AppID:\s*(\d+)\)`)

	for _, match := range re.FindAllStringSubmatch(content, -1) {
		name := strings.TrimSpace(match[1])
		appID := match[2]
		nameMap[appID] = name
	}

	return nameMap
}

// ============================================================
// 数据收集器
// ============================================================

// AppCall 记录一次 addappid() 函数调用的完整信息。
type AppCall struct {
	// AppID 是调用的第一个参数（字符串形式的数字 ID）。
	AppID string
	// HasKey 表示此调用是否携带解密密钥（三参数形式）。
	HasKey bool
	// Key 是解密密钥（仅当 HasKey=true 时有效）。
	Key string
	// Order 是此调用在脚本中的执行顺序（从 0 开始）。
	Order int
}

// ManifestInfo 记录一次 setManifestid() 函数调用的信息。
type ManifestInfo struct {
	ManifestID string
	FileSize   int64
}

// DataCollector 在 Lua 脚本执行过程中收集所有函数调用数据。
//
// 它作为 Lua VM 回调函数的闭包上下文，按调用顺序记录所有
// addappid() 和 setManifestid() 的参数信息。
type DataCollector struct {
	// calls 按执行顺序记录所有 addappid 调用。
	calls []AppCall
	// manifests 存储 depotID → manifest 信息的映射。
	manifests map[string]ManifestInfo
	// callCounter 用于记录调用顺序。
	callCounter int
}

// NewDataCollector 创建一个新的数据收集器实例。
func NewDataCollector() *DataCollector {
	return &DataCollector{
		calls:     []AppCall{},
		manifests: make(map[string]ManifestInfo),
	}
}

// AddAppWithKey 记录一次带密钥的 addappid(id, 1, "key") 调用。
func (c *DataCollector) AddAppWithKey(appID string, key string) {
	c.calls = append(c.calls, AppCall{
		AppID:  appID,
		HasKey: true,
		Key:    key,
		Order:  c.callCounter,
	})
	c.callCounter++
}

// AddApp 记录一次无密钥的 addappid(id) 调用。
func (c *DataCollector) AddApp(appID string) {
	c.calls = append(c.calls, AppCall{
		AppID:  appID,
		HasKey: false,
		Order:  c.callCounter,
	})
	c.callCounter++
}

// SetManifest 记录一次 setManifestid(depotID, "manifestID", fileSize) 调用。
func (c *DataCollector) SetManifest(depotID string, manifestID string, fileSize int64) {
	c.manifests[depotID] = ManifestInfo{
		ManifestID: manifestID,
		FileSize:   fileSize,
	}
}

// BuildGamePackage 将收集到的原始数据组装为最终的 GamePackage 结构。
//
// 组装规则：
//   - 第一个带 key 的 addappid 调用 → MainAppID（主游戏）
//   - 后续带 key 且有对应 manifest 的 → 有效 Depot
//   - 无 key 的 addappid 调用 → DLC 注册（排除主应用 ID）
//   - DLC 名称从 nameMap 中查找，找不到则使用 "DLC <AppID>" 作为默认值
//
// 参数：
//   - meta:    从注释头提取的元信息
//   - nameMap: AppID → DLC 名称的映射表
//
// 返回值：
//   - *GamePackage: 组装完成的游戏数据包
func (c *DataCollector) BuildGamePackage(meta *CommentMetadata, nameMap map[string]string, luaContent string) *GamePackage {
	gp := &GamePackage{
		GameName:   meta.GameName,
		LuaContent: luaContent,
		Depots:     []DepotInfo{},
		DLCs:       []DLCInfo{},
	}

	// 用于去重：同一个 AppID 可能同时出现 addappid(id) 和 addappid(id, 1, "key")
	processedDLCs := make(map[string]bool)
	mainAppFound := false

	for _, call := range c.calls {
		if call.HasKey && !mainAppFound {
			// 第一个带 key 的调用 → 主应用
			gp.MainAppID = call.AppID
			mainAppFound = true
			continue
		}

		if call.HasKey {
			// 带 key 的后续调用 → Depot
			depot := DepotInfo{
				DepotID:       call.AppID,
				DecryptionKey: call.Key,
			}
			if m, ok := c.manifests[call.AppID]; ok {
				depot.ManifestID = m.ManifestID
				depot.FileSize = m.FileSize
			}
			// 仅保留同时具备 key 和 manifest 的有效 Depot
			if depot.ManifestID != "" {
				gp.Depots = append(gp.Depots, depot)
			}
		}

		if !call.HasKey && call.AppID != gp.MainAppID && !processedDLCs[call.AppID] {
			// 无 key 的调用 → DLC 注册
			name := nameMap[call.AppID]
			if name == "" {
				name = "DLC " + call.AppID
			}

			// 检查此 DLC 是否有对应的带 key 调用
			hasKey := false
			decryptionKey := ""
			for _, other := range c.calls {
				if other.AppID == call.AppID && other.HasKey {
					hasKey = true
					decryptionKey = other.Key
					break
				}
			}

			gp.DLCs = append(gp.DLCs, DLCInfo{
				AppID:         call.AppID,
				Name:          name,
				HasKey:        hasKey,
				DecryptionKey: decryptionKey,
			})
			processedDLCs[call.AppID] = true
		}
	}

	// 游戏名称兜底
	if gp.GameName == "" && gp.MainAppID != "" {
		gp.GameName = "游戏 " + gp.MainAppID
	}

	return gp
}

// ============================================================
// Lua VM 执行器
// ============================================================

// parseLuaFile 使用 Lua VM 解析 M 站格式的 Lua 文件。
//
// 完整流程：
//   1. 读取文件内容
//   2. 从注释头提取元信息（游戏名称等）
//   3. 从注释中建立 DLC 名称映射表
//   4. 创建 Lua VM，注册 addappid/setManifestid 回调
//   5. 执行 Lua 脚本，回调自动收集数据
//   6. 将收集到的数据组装为 GamePackage
//
// 相比旧的正则解析方案，本实现：
//   - 天然跳过所有注释行（包括 EXCLUDED DLCS 块）
//   - 不依赖缩进、空行、注释风格等格式细节
//   - 调用顺序由 Lua 执行顺序决定，天然有序且稳定
//
// 参数：
//   - luaPath: Lua 文件的完整路径
//
// 返回值：
//   - *GamePackage: 解析后的游戏数据包
//   - error:       文件读取失败或 Lua 执行出错时返回错误
func (a *App) parseLuaFile(luaPath string) (*GamePackage, error) {
	// 读取文件内容
	contentBytes, err := os.ReadFile(luaPath)
	if err != nil {
		return nil, fmt.Errorf("读取 Lua 文件失败: %w", err)
	}
	content := string(contentBytes)

	// 阶段 1：从注释头提取元信息
	metadata := parseCommentHeader(content)

	// 阶段 2：建立 DLC 名称映射表
	nameMap := buildDLCNameMap(content)

	// 阶段 3：创建 Lua VM 并注册回调函数
	collector := NewDataCollector()
	L := lua.NewState()
	defer L.Close()

	// 注册 addappid 回调
	// 支持两种调用形式：
	//   addappid(appID)              → 无密钥的 DLC 注册
	//   addappid(appID, 1, "key")    → 带密钥的 Depot/DLC 注册
	L.SetGlobal("addappid", L.NewFunction(func(L *lua.LState) int {
		appID := L.CheckNumber(1)
		appIDStr := strconv.FormatInt(int64(appID), 10)

		if L.GetTop() >= 3 {
			// 三参数形式：addappid(id, 1, "key")
			key := L.CheckString(3)
			collector.AddAppWithKey(appIDStr, key)
		} else {
			// 单参数形式：addappid(id)
			collector.AddApp(appIDStr)
		}
		return 0
	}))

	// 注册 setManifestid 回调
	// 调用形式：setManifestid(depotID, "manifestID", fileSize)
	L.SetGlobal("setManifestid", L.NewFunction(func(L *lua.LState) int {
		depotID := L.CheckNumber(1)
		depotIDStr := strconv.FormatInt(int64(depotID), 10)
		manifestID := L.CheckString(2)
		fileSize := int64(L.CheckNumber(3))

		collector.SetManifest(depotIDStr, manifestID, fileSize)
		return 0
	}))

	// 阶段 4：执行 Lua 脚本
	if err := L.DoString(content); err != nil {
		return nil, fmt.Errorf("Lua 脚本执行失败: %w", err)
	}

	// 阶段 5：组装最终结果
	gp := collector.BuildGamePackage(metadata, nameMap, content)

	// 最终校验：确保解析结果不为空
	if gp.MainAppID == "" || (len(gp.DLCs) == 0 && len(gp.Depots) == 0) {
		return nil, fmt.Errorf("解析结果为空或格式不正确，请检查 Lua 文件内容")
	}

	return gp, nil
}
