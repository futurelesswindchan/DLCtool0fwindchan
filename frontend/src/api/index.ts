/**
 * 后端 API 封装层
 *
 * 存在意义：wailsjs 的自动生成绑定参数名为 arg1/arg2，可读性差，
 * 且部分调用需要前置的数据转换（如二进制文件转字节数组）。
 * 此处收敛这些细节，让组件侧只面对语义清晰的函数。
 *
 * NOTE: 本文件是 L1.5 测试台的一部分，用于验证后端 API 是否可用。
 * v2.0 正式界面开发时可保留此层，但需按最终交互需求调整。
 */

import * as App from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export type AppConfig = main.AppConfig
export type DetectorResult = main.DetectorResult
export type GamePackage = main.GamePackage
export type GameRecord = main.GameRecord
export type OperationResult = main.OperationResult
export type DLCInfo = main.DLCInfo

/** 检测注入器环境。返回值永不为空，状态分 ready / missing / unknown 三态。 */
export const detectEnvironment = (): Promise<DetectorResult> =>
  App.DetectEnvironment()

/** 获取当前配置。 */
export const getConfig = (): Promise<AppConfig> => App.GetConfig()

/** 从注册表自动识别 Steam 路径并写入配置。 */
export const autoDetectSteamPath = (): Promise<string> => App.GetSteamPath()

/** 打开文件夹选择对话框，返回用户选择的路径；取消时返回空字符串。 */
export const selectDirectory = (): Promise<string> => App.SelectDirectory()

/** 手动指定 Steam 路径，内部会校验目录下是否存在 config 子目录。 */
export const setSteamPath = (path: string): Promise<OperationResult> =>
  App.SetSteamPath(path)

/** 返回清单文件将被写入的目录，用于向用户展示工具的实际行为。 */
export const getDeployDir = (): Promise<string> => App.GetDeployDir()

/** 打开清单包选择对话框，返回文件路径；取消时返回空字符串。 */
export const selectZipFile = (): Promise<string> => App.SelectZipFile()

/** 解析指定路径的清单包。 */
export const processZipFile = (path: string): Promise<GamePackage> =>
  App.ProcessZipFile(path)

/**
 * 解析拖拽进窗口的清单包。
 *
 * Wails 生成的绑定要求第二参数为 number[]，而浏览器给出的是 File 对象。
 * 此处完成 File → ArrayBuffer → Array<number> 的转换。
 *
 * XXX: 转换会在内存中产生一份与文件等大的数组，清单包通常只有几十 KB
 * 到几 MB，尚可接受。若将来支持超大文件，应改为让后端直接读取路径。
 */
export async function processDroppedFile(file: File): Promise<GamePackage> {
  const buffer = await file.arrayBuffer()
  return App.ProcessDroppedFile(file.name, Array.from(new Uint8Array(buffer)))
}

/** 部署清单并记录历史。selectedIDs 为用户勾选的 DLC AppID 列表。 */
export const installDLCs = (
  pkg: GamePackage,
  selectedIDs: string[],
): Promise<OperationResult> => App.InstallDLCs(pkg, selectedIDs)

/** 按主游戏 AppID 移除清单文件与历史记录。 */
export const removeDLCs = (mainAppID: string): Promise<OperationResult> =>
  App.RemoveDLCs(mainAppID)

/** 获取全部安装历史，已按安装时间倒序。 */
export const getHistory = (): Promise<GameRecord[]> => App.GetHistory()

/** 在系统文件管理器中打开本工具的数据目录。 */
export const openDataDir = (): Promise<OperationResult> => App.OpenDataDir()

/** 返回当前日志文件路径。 */
export const getLogPath = (): Promise<string> => App.GetLogPath()
