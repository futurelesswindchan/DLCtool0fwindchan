// app_diag.go
//
// 本文件承载「排障与诊断」类前端方法：打开数据目录、日志路径、
// 导出诊断包。它们与 DLC 清单管理无关，只服务于用户报障链路，
// 故与业务方法分开存放（同一 package main，Wails 感知不到差异）。
//
// 存在意义源于一条封测期的现实约束：群聊里最常见的一句话是「把日志
// 发上来」，而数据目录中的 config.json 明文存着用户自己申请的 API
// 凭据。若不提供一个「自动脱敏打包」的按钮，用户就会手动翻文件夹、
// 直接把原始 config.json 发到群里——那等于把自己的额度送人。
//
// 因此 ExportDiagnostics 不是锦上添花的便利功能，而是替代危险行为的
// 安全出口：它必须比手动翻目录更省事，用户才会选它。

package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

const (
	// diagLogCount 是诊断包中收录的日志文件数量（按修改时间取最新）。
	//
	// 取 3 而非全部：日志按天轮转，报障通常发生在当天或前一天，
	// 而把半年的日志全打进 zip 只会让群里传不动文件。
	diagLogCount = 3

	// diagMaskedTokenPlaceholder 是脱敏后写入配置副本的占位值。
	//
	// 不留空字符串：空值会让排障者无法区分「用户没填凭据」与
	// 「凭据已被脱敏」，而这两种情形的处置方式完全不同。
	diagMaskedTokenPlaceholder = "***已脱敏***"
)

// OpenDataDir 在系统文件管理器中打开本工具的数据目录。
//
// 返回值：
//   - *OperationResult: 目录不可用或打开失败时 Success 为 false
//
// 为何不用 runtime.BrowserOpenURL：Wails v2.11 起为该函数加入了 URL
// 校验（internal/frontend/utils/urlValidator.go），两道关卡各自独立地
// 否决了 Windows 本地路径——
//
//	scheme == ""            → Windows 裸路径无 scheme，命中「scheme not allowed」
//	路径含反斜杠            → 命中 shell 危险字符黑名单
//
// 表现为点击按钮毫无反应，控制台仅一行 "Invalid URL shell metacharacters
// not allowed"。这不是本项目的缺陷，而是上游收紧了安全校验，故改为
// 直接调用 explorer。
func (a *App) OpenDataDir() *OperationResult {
	dir, err := appDataDir()
	if err != nil {
		return failure(fmt.Sprintf("数据目录不可用：%v", err))
	}
	return a.openInExplorer(dir)
}

// openInExplorer 用资源管理器打开指定路径。
//
// 参数：
//   - path: 目标目录或文件的完整路径
//
// 返回值：
//   - *OperationResult: 启动失败时 Success 为 false
//
// 两个易踩的点：
//
//  1. **不能据退出码判断成败**。explorer.exe 在成功打开窗口的情况下
//     同样返回退出码 1，故此处只 Start() 不 Wait()——既避免误报失败，
//     也不必为一个即时返回的进程阻塞调用方。
//  2. **路径必须作为独立参数传入**，绝不拼进命令字符串。用户的
//     Steam 路径与主目录都可能含空格，而字符串拼接在含引号或 & 的
//     路径下会变成命令注入面。exec.Command 的变参形式不经 shell 解析，
//     天然免疫。
func (a *App) openInExplorer(path string) *OperationResult {
	if runtime.GOOS != "windows" {
		return failure("当前系统不支持该操作")
	}

	cmd := exec.Command("explorer", path)
	if err := cmd.Start(); err != nil {
		a.logger.Warn("打开资源管理器失败 %s: %v", path, err)
		return failure(fmt.Sprintf("无法打开文件管理器：%v", err))
	}

	// 不 Wait()：explorer 成功时也返回退出码 1，等待只会拿到误导性的
	// 错误。进程本身由系统回收，此处不 Wait 不产生僵尸进程。
	a.logger.Info("已请求资源管理器打开 %s", path)
	return success("已打开文件管理器")
}

// GetLogPath 返回当前日志文件的完整路径。
//
// 供前端「打开日志」功能使用。日志系统降级运行时返回空字符串。
func (a *App) GetLogPath() string {
	return a.logger.Path()
}
