// app_trial_install.go
//
// 从试下载结果直接入库。
//
// 这是「试下载」这笔交易的收益端：用户为对比表付出了一轮等待，此处让他
// 免掉第二次下载。若缺了这一环，试下载就成了纯粹的额外等待，用户会有充分
// 理由抱怨工具变慢了。

package main

import (
	"fmt"
	"strings"
)

// InstallFromTrial 用试下载得到的清单包入库，缓存缺失时回退为重新下载。
//
// 参数：
//   - appID:  游戏的 Steam AppID
//   - source: 用户选定的源名称
//
// 返回值：
//   - *GamePackage: 清单包，与 DownloadFromRepo 的产出完全一致
//   - error:        下载或解析失败的原因
//
// 缓存命中时零网络请求。缺失时静默回退到 DownloadFromRepo——缓存过期
// （30 分钟）或用户在别处重启过应用都会导致缺失，这属正常处境而非错误，
// 不该让用户看到「缓存丢失」这类他无法处置的提示。
//
// NOTE: 回退路径下拿到的是重新下载的包，其 DLC 数可能与对比表所示不同
// （上游刷新了清单）。这一差异不做提示——数字本就只是当时的快照，
// 而多一句「与之前显示的不同」只会让用户困惑该信哪个。
func (a *App) InstallFromTrial(appID string, source string) (*GamePackage, error) {
	appID = strings.TrimSpace(appID)
	if !isNumeric(appID) {
		return nil, fmt.Errorf("AppID 必须为纯数字: %q", appID)
	}

	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("未指定源")
	}

	if e, ok := a.trials.get(appID, source); ok && e.pkg != nil {
		a.logger.Info("复用试下载产物入库：AppID %s 源 %s（%d 个 DLC）",
			appID, source, len(e.pkg.DLCs))
		return e.pkg, nil
	}

	a.logger.Info("试下载产物已失效，重新下载：AppID %s 源 %s", appID, source)
	return a.DownloadFromRepo(appID, source)
}
