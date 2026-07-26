// lua_match.go
//
// 本文件提供对 Lua 清单脚本做文本匹配的工具函数。
//
// 与 lua_parser.go 的分工：
//   - lua_parser.go 用嵌入式 Lua VM 执行脚本，提取完整的结构化数据，
//     用于解析用户导入的清单包
//   - 本文件只做轻量的文本判断，用于「某个 AppID 是否已在脚本中」
//     这类不值得启动 VM 的场景
//
// 之所以不统一用 VM：检测安装状态时可能要对几十个 AppID 逐个判断，
// 为此反复执行脚本代价过高；而正则在这个场景下足够可靠。

package main

import (
	"fmt"
	"regexp"
)

// luaContainsAppID 检查 Lua 脚本中是否存在指定 AppID 的 addappid 调用。
//
// 相比 strings.Contains，本函数能容忍以下格式变体：
//   - addappid 与左括号之间的空格
//   - AppID 前后的空格
//   - 参数后紧跟逗号（三参数形式）或右括号（单参数形式）
//
// 同时会跳过被注释掉的调用——清单包常在文件尾部用注释列出
// 「已排除的 DLC」，若不区分注释就会把它们误判为已安装。
//
// 参数：
//   - content: Lua 脚本的完整文本
//   - appID:   待检查的 AppID
//
// 返回值：
//   - bool: 是否存在有效（未被注释）的匹配调用
//
// XXX: 行首至 addappid 之间不允许出现连字符，这是排除注释行的手段。
// 代价是形如 `x = a-b; addappid(123)` 这样的行会被漏判，
// 但清单脚本中不会出现这种写法，可以接受。
func luaContainsAppID(content, appID string) bool {
	pattern := fmt.Sprintf(`(?m)^[^-]*addappid\(\s*%s\s*[,)]`, regexp.QuoteMeta(appID))
	return regexp.MustCompile(pattern).MatchString(content)
}
