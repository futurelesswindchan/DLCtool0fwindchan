// package_store.go
//
// 清单解析结果的本地留存。
//
// 存在意义：压缩包在 %TEMP% 内处理后即删，若不留下 GamePackage，用户
// 重启应用后就无法再调整已入库游戏的 DLC 勾选——只能重新联网下载一次
// 清单，既耗流量也消耗认证型源的每日额度。
//
// 序列化后的 GamePackage 比压缩包小两三个数量级，且已是可直接使用的
// 状态（无需再解析），故以 JSON 逐游戏落盘。
//
// 时效处理：本层只负责存取与记录写入时刻，不做任何过期判定。是否需要
// 重新获取由界面结合 SavedAt 呈现给用户决定——清单「旧」不等于「无效」，
// 多数情况下几个月前的清单依然可用。
//
// NOTE: 有意不在此处引入版本号或来源优先级等概念。清单源的探索仍在进行，
// 未来可能出现携带更多元数据的源，届时扩展 StoredPackage 的字段即可，
// 不必推翻存储结构。

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// packagesDirName 是清单解析结果的子目录名，位于数据目录下。
	packagesDirName = "packages"
)

// StoredPackage 是 GamePackage 的落盘格式。
//
// 外层包一层元信息而非直接序列化 GamePackage，是为了记录「何时、从哪
// 获取」——二者都是界面需要展示的信息，且无法从 GamePackage 本身推得。
//
// 字段说明：
//   - SavedAt: 写入时刻，RFC 3339 字符串
//   - Source:  获取来源的源名称，本地导入时为空
//   - Package: 清单解析结果
//
// NOTE: SavedAt 用字符串而非 time.Time——本结构会跨 Wails 边界暴露给
// 前端，标准库复合类型会让 wails generate module 静默丢字段。
type StoredPackage struct {
	SavedAt string       `json:"savedAt"`
	Source  string       `json:"source"`
	Package *GamePackage `json:"package"`
}

// PackageStore 管理清单解析结果的本地留存。
//
// 无内部可变状态，所有方法可并发调用。文件写入经 atomicWriteFile，
// 因此即使在写入过程中断电，也不会留下半截的 JSON。
type PackageStore struct {
	logger *Logger
}

// NewPackageStore 创建清单留存管理器。
func NewPackageStore(logger *Logger) *PackageStore {
	return &PackageStore{logger: logger}
}

func (p *PackageStore) log(format string, args ...any) {
	if p.logger != nil {
		p.logger.Info(format, args...)
	}
}

// packagePath 返回指定主游戏 AppID 的留存文件路径。
//
// 数据目录不可用时返回空字符串，调用方据此跳过留存——它是便利功能，
// 拿不到目录不该让部署失败。
func packagePath(mainAppID string) string {
	dir, err := appDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, packagesDirName, mainAppID+".json")
}

// Save 留存一份清单解析结果。
//
// 参数：
//   - gp:     清单解析结果，MainAppID 为空时直接返回错误
//   - source: 获取来源的源名称，本地导入时传空
//
// 返回值：
//   - error: 写入失败的原因。调用方应记日志而非中断流程——清单已经
//     部署成功了，没有理由因为留存失败而告知用户部署失败
func (p *PackageStore) Save(gp *GamePackage, source string) error {
	if gp == nil || gp.MainAppID == "" {
		return fmt.Errorf("清单包无效，无法留存")
	}

	path := packagePath(gp.MainAppID)
	if path == "" {
		return fmt.Errorf("数据目录不可用")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建留存目录失败: %w", err)
	}

	data, err := json.MarshalIndent(StoredPackage{
		SavedAt: time.Now().Format(time.RFC3339),
		Source:  source,
		Package: gp,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化清单失败: %w", err)
	}

	if err := atomicWriteFile(path, data); err != nil {
		return fmt.Errorf("写入清单留存失败: %w", err)
	}

	p.log("清单已留存: AppID %s（%d 个 DLC，来源 %s）",
		gp.MainAppID, len(gp.DLCs), sourceLabel(source))
	return nil
}

// Load 读取指定主游戏的留存清单。
//
// 返回值：
//   - *StoredPackage: 留存内容。不存在或已损坏时为 nil
//   - error:          仅在文件存在但无法使用时返回；「没有留存」不是错误
//
// 不做过期判定：清单旧不等于无效，多数情况下几个月前的清单依然可用。
// 是否重新获取交由用户按 SavedAt 自行判断。
func (p *PackageStore) Load(mainAppID string) (*StoredPackage, error) {
	if mainAppID == "" {
		return nil, nil
	}

	path := packagePath(mainAppID)
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取清单留存失败: %w", err)
	}

	var stored StoredPackage
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("清单留存内容已损坏: %w", err)
	}
	if stored.Package == nil || stored.Package.MainAppID == "" {
		return nil, fmt.Errorf("清单留存内容不完整")
	}

	return &stored, nil
}

// Delete 删除指定主游戏的留存清单。
//
// 文件不存在视为成功——调用方的意图是「让它不存在」，已经不存在时
// 该意图已然达成。
func (p *PackageStore) Delete(mainAppID string) error {
	path := packagePath(mainAppID)
	if path == "" {
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除清单留存失败: %w", err)
	}
	return nil
}

// sourceLabel 把可能为空的来源名转成便于阅读的日志文本。
func sourceLabel(source string) string {
	if source == "" {
		return "本地导入"
	}
	return source
}
