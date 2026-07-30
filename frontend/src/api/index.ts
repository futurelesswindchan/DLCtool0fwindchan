/**
 * 后端 API 封装层
 *
 * 存在意义有二：
 *   1. wailsjs 自动生成的绑定参数名为 arg1/arg2，可读性差
 *   2. 部分调用需前置数据转换（如二进制文件转字节数组）
 *
 * 组件不得直接 import wailsjs，一律经本层。Wails 升级改变生成规则时
 * 只需改动一处。
 *
 * NOTE: 返回 OperationResult 的后端方法在此统一归一化为异常语义
 * （见 unwrap），使调用方只写一套 try/catch，不必同时判断 success
 * 字段与捕获运行时异常两条分支。
 */

import * as App from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'

export type AppConfig = main.AppConfig
export type RepoSource = main.RepoSource
export type DetectorResult = main.DetectorResult
export type GamePackage = main.GamePackage
export type GameRecord = main.GameRecord
export type OperationResult = main.OperationResult
export type DLCInfo = main.DLCInfo
export type DepotInfo = main.DepotInfo
export type DeployedEntry = main.DeployedEntry
export type GameSearchResult = main.GameSearchResult
export type GameDetail = main.GameDetail
export type MSiteStats = main.MSiteStats
export type StoredPackage = main.StoredPackage
export type UpdateInfo = main.UpdateInfo
export type DiagnosticsResult = main.DiagnosticsResult
export type BuildInfo = main.BuildInfo
export type SourceTrial = main.SourceTrial
export type TrialReport = main.TrialReport

/**
 * 后端业务失败所抛出的异常。
 *
 * 与运行时异常区分开来，便于调用方在需要时判定失败来源——业务失败的
 * message 是后端给出的面向用户文案，可直接展示；运行时异常则不宜直出。
 */
export class ApiError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

/**
 * 将 OperationResult 归一化为异常语义。
 *
 * 返回后端携带的 message，成功路径上仍可用于展示提示文案。
 */
async function unwrap(p: Promise<OperationResult>): Promise<string> {
  const r = await p
  if (!r.success) throw new ApiError(r.message)
  return r.message
}

/* ─── 环境与配置 ─── */

/** 检测注入器环境。返回值永不为空，状态分 ready / missing / unknown 三态。 */
export const detectEnvironment = (): Promise<DetectorResult> =>
  App.DetectEnvironment()

/** 获取当前配置。 */
export const getConfig = (): Promise<AppConfig> => App.GetConfig()

/** 保存配置。路径变更时后端会重建部署器。 */
export const saveConfig = (cfg: AppConfig): Promise<string> =>
  unwrap(App.SaveConfig(cfg))

/** 从注册表自动识别 Steam 路径并写入配置。 */
export const autoDetectSteamPath = (): Promise<string> => App.GetSteamPath()

/** 打开文件夹选择对话框，返回用户选择的路径；取消时返回空字符串。 */
export const selectDirectory = (): Promise<string> => App.SelectDirectory()

/** 手动指定 Steam 路径，内部会校验目录下是否存在 config 子目录。 */
export const setSteamPath = (path: string): Promise<string> =>
  unwrap(App.SetSteamPath(path))

/** 返回清单文件将被写入的目录，用于向用户展示工具的实际行为。 */
export const getDeployDir = (): Promise<string> => App.GetDeployDir()

/* ─── 本地清单包 ─── */

/** 打开清单包选择对话框，返回文件路径；取消时返回空字符串。 */
export const selectZipFile = (): Promise<string> => App.SelectZipFile()

/** 解析指定路径的清单包。 */
export const processZipFile = (path: string): Promise<GamePackage> =>
  App.ProcessZipFile(path)

/**
 * 解析拖拽进窗口的清单包。
 *
 * Wails 生成的绑定要求第二参数为 number[]，而浏览器给出的是 File 对象，
 * 此处完成 File → ArrayBuffer → Array<number> 的转换。
 *
 * XXX: 转换会在内存中产生一份与文件等大的数组。清单包通常几十 KB 到
 * 几 MB，尚可接受；若将来支持超大文件，应改为让后端直接读取路径。
 */
export async function processDroppedFile(file: File): Promise<GamePackage> {
  const buffer = await file.arrayBuffer()
  return App.ProcessDroppedFile(file.name, Array.from(new Uint8Array(buffer)))
}

/* ─── 部署与卸载 ─── */

/**
 * 部署清单并记录历史。selectedIDs 为用户勾选的 DLC AppID 列表。
 *
 * 返回后端文案：检出外部声明时文案会附带提示，但操作仍算成功。
 */
export const installDLCs = (
  pkg: GamePackage,
  selectedIDs: string[],
): Promise<string> => unwrap(App.InstallDLCs(pkg, selectedIDs))

/**
 * 按主游戏 AppID 移除清单文件与历史记录。
 *
 * NOTE: 检出外部 lua 也声明了同一 AppID 时，后端返回的是**失败**。
 * 这不是异常，而是如实告知「已删除本工具的文件，但游戏可能仍在库中」。
 * 调用方应把 ApiError.message 原样呈现，其中已列出需手动处理的文件名。
 */
export const removeDLCs = (mainAppID: string): Promise<string> =>
  unwrap(App.RemoveDLCs(mainAppID))

/* ─── 历史与对账 ─── */

/** 获取全部安装历史，已按安装时间倒序。 */
export const getHistory = (): Promise<GameRecord[]> => App.GetHistory()

/** 按主游戏 AppID 查单条历史，用于带出上次的勾选状态。 */
export const findHistory = (mainAppID: string): Promise<GameRecord> =>
  App.FindHistory(mainAppID)

/** 仅清空历史记录，不动已部署的文件。 */
export const clearHistory = (): Promise<string> => unwrap(App.ClearHistory())

/**
 * 扫描部署目录，对账实际文件与历史记录。
 *
 * 返回条目含 isExternal / inHistory 两个判定位，界面据此区分常态、
 * 历史丢失、外部清单与双处声明四种情形。
 */
export const scanDeployed = (): Promise<DeployedEntry[]> => App.ScanDeployed()

/**
 * 读取已入库游戏的留存清单。
 *
 * 使用户重启应用后仍能调整 DLC 勾选，无需重新联网下载。
 *
 * NOTE: 「没有留存」不是错误——返回 null 时应引导用户获取清单，而抛错
 * 时才提示重试。二者混为一谈会让用户看到莫名的错误提示。
 *
 * 后端不做过期判定。界面应按 savedAt 表述为「获取于 X 天前」，
 * 而非「已过期」——清单旧不等于无效。
 */
export const getPackage = (mainAppID: string): Promise<StoredPackage | null> =>
  App.GetPackage(mainAppID)

/* ─── 在线获取 ─── */

/** 搜索游戏。纯数字输入按 AppID 直查。 */
export const searchGames = (term: string): Promise<GameSearchResult[]> =>
  App.SearchGames(term)

/** 获取游戏详情。后端在接口失败时返回降级结果而非报错。 */
export const getGameDetail = (appID: string): Promise<GameDetail> =>
  App.GetGameDetail(appID)

/** 并发查询各源的收录情况，返回收录了该游戏的源名称列表。 */
/**
 * 对各源做试下载，返回实得 DLC 数的对比。
 *
 * 存在意义：`lookupRepos` 只能回答「该源存在这个游戏的文件」，而用户关心
 * 的是「这个源能给我多少 DLC」。实测同一个游戏 7 个源全报确认收录，实得
 * 从 200 个到 0 个——用户看到「收录 7/7」却拿到 0 个 DLC，必然认为是本
 * 工具坏了。
 *
 * NOTE: 耗时较长（最坏为两批超时之和），调用期间必须展示进度。
 * 认证型源不参与自动试下载，需用 `trialOneSource` 由用户主动触发。
 *
 * @param refresh 为真时忽略缓存强制重查
 */
export const trialSources = (
  appID: string,
  refresh = false,
): Promise<TrialReport> => App.TrialSources(appID, refresh)

/**
 * 对单个源做试下载。供认证型源由用户主动触发。
 *
 * 与 `trialSources` 分开而非加开关：开关式设计下前端传错参数就会静默消耗
 * 用户的 API 额度，而额度不可退还。
 */
export const trialOneSource = (
  appID: string,
  source: string,
): Promise<SourceTrial> => App.TrialOneSource(appID, source)

/**
 * 用试下载得到的清单包入库，缓存缺失时自动回退为重新下载。
 *
 * 这是试下载那一轮等待的收益端——缓存命中时零网络请求。
 */
export const installFromTrial = (
  appID: string,
  source: string,
): Promise<GamePackage> => App.InstallFromTrial(appID, source)

export const lookupRepos = (appID: string): Promise<string[]> =>
  App.LookupRepos(appID)

/** 从指定源下载并解析清单包，解析路径由后端按包内形态自动分派。 */
export const downloadFromRepo = (
  appID: string,
  sourceName: string,
): Promise<GamePackage> => App.DownloadFromRepo(appID, sourceName)

/** 设置认证型源的凭据，传空字符串即清除。 */
export const setRepoToken = (
  sourceName: string,
  token: string,
): Promise<string> => unwrap(App.SetRepoToken(sourceName, token))

/** 获取 M 站额度与凭据到期状态。未配置凭据时后端返回 null。 */
export const getMSiteStats = (): Promise<MSiteStats | null> =>
  App.GetMSiteStats()

/* ─── 杂项 ─── */

/** 在系统文件管理器中打开本工具的数据目录。 */
export const openDataDir = (): Promise<string> => unwrap(App.OpenDataDir())

/** 返回当前日志文件路径。 */
export const getLogPath = (): Promise<string> => App.GetLogPath()

/**
 * 导出脱敏诊断包到数据目录，并自动打开所在文件夹。
 *
 * 存在意义是替代危险行为：数据目录里的 `config.json` 明文存着用户自己
 * 申请的 API 凭据，而报障场合最常见的一句话是「把日志发上来」。若不给
 * 一个更省事的脱敏出口，用户就会直接分享原始配置文件。
 *
 * NOTE: 后端返回 error 而非 OperationResult，调用方需自行 catch。
 */
export const exportDiagnostics = (): Promise<DiagnosticsResult> =>
  App.ExportDiagnostics()

/* ─── 应用自身 ─── */

/** 返回当前构建的版本号。未经 ldflags 注入的本地构建返回 `dev`。 */
export const getAppVersion = (): Promise<string> => App.GetAppVersion()

/**
 * 返回构建身份（版本 + 提交哈希 + 构建时刻）。
 *
 * 封测期间同一版本号会被反复重新构建，此时版本号无法区分用户手里的包
 * 是哪一次构建，而报障最需要确定的恰恰是这一点。`label` 字段是供界面
 * 直接展示、供用户抄写的单行标识，格式与诊断包内的完全一致。
 */
export const getBuildInfo = (): Promise<BuildInfo> => App.GetBuildInfo()

/** 返回项目发布页地址，供检查更新失败时的手动跳转。 */
export const getReleasePageURL = (): Promise<string> => App.GetReleasePageURL()

/** 在系统默认浏览器中打开链接。后端只放行 http 与 https。 */
export const openURL = (url: string): Promise<string> =>
  unwrap(App.OpenURL(url))

/**
 * 查询最新版本并与当前版本比对。
 *
 * NOTE: 后端在此处返回 error 而非 OperationResult，故调用方需自行 catch。
 * 检查更新失败是常态（国内访问 api.github.com 常直接超时），应按「暂时
 * 查不到」呈现，配合 getReleasePageURL 引导手动查看，而非报为操作失败。
 */
export const checkUpdate = (): Promise<UpdateInfo> => App.CheckUpdate()
