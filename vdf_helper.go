// vdf_helper.go
//
// 本文件提供 Valve Data Format (VDF) 文件的局部操作工具函数。
//
// 设计思路：
//   config.vdf 是 Steam 客户端自动生成的配置文件，格式为 VDF（KeyValues）。
//   本工具仅需操作其中的 "depots" 子节点（增删解密密钥），无需解析整个文件。
//   因此采用"局部定位 + 格式自适应"策略：
//     - 通过括号配对精确定位 depots 节点的字节范围
//     - 从现有内容推断缩进风格，确保写入格式与原文一致
//     - 删除时使用正则匹配，容忍缩进和空白变体
//
// 外部依赖：
//   - github.com/andygrunwald/vdf: 用于 detectInstalledDLCs 的精确树状解析

package main

import (
	"fmt"
	"regexp"
	"strings"

	vdf "github.com/andygrunwald/vdf"
)

// findDepotsSection 在 config.vdf 内容中精确定位 "depots" 节点的字节范围。
//
// 定位策略：
//   1. 搜索 `"depots"` 关键字（带引号匹配，避免命中值中的子串）
//   2. 从关键字位置向后找到第一个 `{`
//   3. 从 `{` 开始做局部括号计数，找到配对的 `}`
//
// 返回值：
//   - keyIdx:   "depots" 关键字的起始字节索引
//   - openIdx:  depots 节点开括号 `{` 的字节索引
//   - closeIdx: depots 节点闭括号 `}` 的字节索引
//   - err:      定位失败时返回错误（关键字不存在、括号不配对等）
func findDepotsSection(content string) (keyIdx, openIdx, closeIdx int, err error) {
	keyIdx = strings.Index(content, `"depots"`)
	if keyIdx == -1 {
		return 0, 0, 0, fmt.Errorf("在 config.vdf 中找不到 \"depots\" 节点")
	}

	// 从关键字末尾开始搜索开括号
	searchStart := keyIdx + len(`"depots"`)
	relativeOpen := strings.Index(content[searchStart:], "{")
	if relativeOpen == -1 {
		return 0, 0, 0, fmt.Errorf("找不到 depots 节点的开括号")
	}
	openIdx = searchStart + relativeOpen

	// 局部括号配对
	depth := 0
	for i := openIdx; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return keyIdx, openIdx, i, nil
			}
		}
	}

	return 0, 0, 0, fmt.Errorf("depots 节点括号不配对（缺少闭括号）")
}

// inferIndent 从 depots 节点内部的现有内容推断缩进风格。
//
// 推断策略：
//   扫描 depots 内部第一个包含引号的非空行，提取其前导空白作为"条目缩进"，
//   在此基础上追加一个 tab 作为"内部缩进"（DecryptionKey 行的缩进）。
//
// 若 depots 节点为空（首次写入），使用默认缩进（5 tab / 6 tab），
// 对应样本中 depots 在 VDF 树第 5 层的标准位置。
//
// 参数：
//   - content:  config.vdf 的完整文本
//   - openIdx:  depots 开括号的字节索引
//   - closeIdx: depots 闭括号的字节索引
//
// 返回值：
//   - entryIndent: DepotID 行和子节点括号行的缩进字符串
//   - innerIndent: DecryptionKey 等键值对行的缩进字符串
func inferIndent(content string, openIdx, closeIdx int) (entryIndent, innerIndent string) {
	defaultEntry := "\t\t\t\t\t"
	defaultInner := "\t\t\t\t\t\t"

	inner := content[openIdx+1 : closeIdx]
	lines := strings.Split(inner, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 跳过空行和纯括号行，找到第一个带引号的内容行（即 "DepotID"）
		if trimmed == "" || trimmed == "{" || trimmed == "}" {
			continue
		}
		if !strings.Contains(trimmed, `"`) {
			continue
		}
		// 提取前导空白
		entryIndent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		innerIndent = entryIndent + "\t"
		return entryIndent, innerIndent
	}

	return defaultEntry, defaultInner
}

// buildDepotBlock 按指定缩进生成一个 VDF depot 条目块。
//
// 生成格式与 Steam 客户端写入的格式一致：
//
//	<entry>"DepotID"
//	<entry>{
//	<inner>"DecryptionKey"		"hex..."
//	<entry>}
//
// 参数：
//   - depotID:       Depot 或 DLC 的 AppID
//   - decryptionKey: 解密密钥的十六进制字符串
//   - entryIndent:   条目行的缩进前缀
//   - innerIndent:   内部键值对行的缩进前缀
//
// 返回值：
//   - string: 格式化后的 VDF 块文本（以换行符开头，不以换行符结尾）
func buildDepotBlock(depotID, decryptionKey, entryIndent, innerIndent string) string {
	return fmt.Sprintf("\n%s\"%s\"\n%s{\n%s\"DecryptionKey\"\t\t\"%s\"\n%s}",
		entryIndent, depotID,
		entryIndent,
		innerIndent, decryptionKey,
		entryIndent)
}

// removeDepotBlock 使用正则从 config.vdf 内容中移除指定 ID 的 depot 条目块。
//
// 匹配模式容忍以下格式变体：
//   - 任意数量的前导空白（tab 或空格混合）
//   - 换行符差异（\r\n 或 \n）
//   - DecryptionKey 与值之间的任意空白分隔符
//
// 参数：
//   - content: config.vdf 的完整文本
//   - depotID: 要移除的 Depot/DLC AppID
//
// 返回值：
//   - string: 移除后的文本
//   - bool:   是否实际发生了移除（用于判断是否需要写回文件）
func removeDepotBlock(content, depotID string) (string, bool) {
	// 匹配完整的 depot 块：换行 + 缩进 + "ID" + 换行 + 缩进 + { + 换行 + 缩进 + "DecryptionKey" + 值 + 换行 + 缩进 + }
	pattern := fmt.Sprintf(
		`\r?\n[ \t]*"%s"[ \t]*\r?\n[ \t]*\{[ \t]*\r?\n[ \t]*"DecryptionKey"[ \t]+"[^"]*"[ \t]*\r?\n[ \t]*\}`,
		regexp.QuoteMeta(depotID))
	re := regexp.MustCompile(pattern)

	result := re.ReplaceAllString(content, "")
	return result, result != content
}

// luaContainsAppID 使用正则检查 Lua 文件中是否存在指定 AppID 的 addappid 调用。
//
// 相比简单的 strings.Contains，本函数能容忍：
//   - addappid 与括号之间的空格
//   - AppID 前后的空格
//   - 参数后紧跟逗号或右括号的两种形式
//
// 参数：
//   - content: Lua 文件的完整文本
//   - appID:   要检查的 AppID 字符串
//
// 返回值：
//   - bool: 是否存在匹配的 addappid 调用（未被注释的有效调用）
func luaContainsAppID(content, appID string) bool {
	// 匹配未被注释的 addappid 调用
	pattern := fmt.Sprintf(`(?m)^[^-]*addappid\(\s*%s\s*[,)]`, regexp.QuoteMeta(appID))
	re := regexp.MustCompile(pattern)
	return re.MatchString(content)
}

// luaLineMatchesAppID 检查单行文本是否包含指定 AppID 的 addappid 调用。
//
// 用于 unpatchSteamtoolsLua 的逐行过滤，容忍空格和参数格式变体。
// 注意：被注释掉的行（以 -- 开头）不会被匹配，避免误删 EXCLUDED DLCS 区域的注释。
//
// 参数：
//   - line:  单行文本（原始格式，未 TrimSpace）
//   - appID: 要匹配的 AppID
//
// 返回值：
//   - bool: 该行是否为需要移除的 addappid 调用
func luaLineMatchesAppID(line, appID string) bool {
	trimmed := strings.TrimSpace(line)
	// 跳过注释行
	if strings.HasPrefix(trimmed, "--") {
		return false
	}
	pattern := fmt.Sprintf(`addappid\(\s*%s\s*[,)]`, regexp.QuoteMeta(appID))
	re := regexp.MustCompile(pattern)
	return re.MatchString(trimmed)
}

// parseDepotsKeys 使用 VDF 解析器精确提取 depots 节点中所有子键名。
//
// 策略：先用 findDepotsSection 提取 depots 区域文本，
// 包装为最小 VDF 文档后交给 andygrunwald/vdf 库解析。
// 这样即使整个 config.vdf 有其他区域的格式异常，也不影响 depots 的解析。
//
// 参数：
//   - content: config.vdf 的完整文本
//
// 返回值：
//   - map[string]bool: depots 节点下所有子键名（即 DepotID）的集合；解析失败时返回 nil
func parseDepotsKeys(content string) map[string]bool {
	_, openIdx, closeIdx, err := findDepotsSection(content)
	if err != nil {
		return nil
	}

	// 将 depots 区域包装为独立的 VDF 文档，避免解析整个文件
	depotsDoc := "\"depots\"\n" + content[openIdx:closeIdx+1]

	p := vdf.NewParser(strings.NewReader(depotsDoc))
	m, err := p.Parse()
	if err != nil {
		return nil
	}

	depots, ok := m["depots"]
	if !ok {
		return nil
	}

	depotsMap, ok := depots.(map[string]interface{})
	if !ok {
		return nil
	}

	keys := make(map[string]bool)
	for k := range depotsMap {
		keys[k] = true
	}
	return keys
}