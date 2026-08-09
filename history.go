// history.go
//
// 本文件负责安装历史的管理：记录用户装过哪些游戏、装了哪些 DLC。
//
// 历史记录的三个用途：
//   1. 让用户回看「我都装过什么」
//   2. 卸载时提供当初部署的文件名与 DLC 清单
//   3. 重装同一游戏时带出上次的勾选状态，免得重新一个个点
//
// 存储位置：~/.kazeusa/history.json
// 写入策略：与配置一致，走 atomicWriteFile 原子提交。
//
// 去重规则：以 mainAppID 为唯一键，重复部署同一游戏是「更新记录」
// 而非追加新条目——否则用户装五次同一个游戏，历史里就躺着五条
// 几乎一样的条目。

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// GameRecord 表示一条安装历史记录。
//
// 字段说明：
//   - MainAppID:    主游戏 AppID，同时作为记录的唯一键
//   - GameName:     游戏名称
//   - DLCCount:     清单包中可选 DLC 的总数，用于展示「已装 3 / 共 21」
//   - InstalledIDs: 实际部署的 DLC AppID 列表
//   - InstalledAt:  最近一次部署的时间，RFC 3339 格式字符串
//   - LuaFileName:  部署产生的清单文件名（不含目录），供卸载时定位
//   - Source:       清单包的来源，取值为 RepoSource.Name 或 SourceLocalImport
//
// InstalledAt 用字符串而非 time.Time 的原因：
//
//	Wails 的 TypeScript 类型生成器无法处理 time.Time，会输出
//	「Not found: time.Time」并让前端拿到无类型的字段。改用 RFC 3339
//	字符串后，前端可直接 new Date(record.installedAt) 解析，
//	JSON 中的表现形式也与 time.Time 序列化后完全一致。
//
// Source 的价值在于三个来源的数据完整度差异悬殊（见 DECISIONS.md 的
// 「三源并非同构」条）：同一游戏经 MAU 入库只有 4 个 DLC，经 M 站则有 19 个。
// 用户回看历史时若不知当初的来源，便无从判断是否值得换源重装。
//
// NOTE: 旧版 history.json 中无此字段，反序列化后为空字符串。
// 读取方须容忍空值，不得据此判定记录损坏。
type GameRecord struct {
	MainAppID    string   `json:"mainAppID"`
	GameName     string   `json:"gameName"`
	DLCCount     int      `json:"dlcCount"`
	InstalledIDs []string `json:"installedIDs"`
	InstalledAt  string   `json:"installedAt"`
	LuaFileName  string   `json:"luaFileName"`
	Source       string   `json:"source"`
}

// SourceLocalImport 是本地导入的来源标识，用于 GameRecord.Source。
//
// 在线源一律使用 RepoSource.Name 作为取值，故此处只需为「非在线」这唯一
// 情形定义常量。取中文字面量是因为该值会直接呈现于界面，无需再做映射。
const SourceLocalImport = "本地导入"

// HistoryManager 管理安装历史的读取、更新与持久化。
//
// 并发安全：Wails 为每个前端调用启用独立 goroutine，
// 用户快速连点安装按钮时可能出现并发写入。
type HistoryManager struct {
	mu      sync.RWMutex
	records []GameRecord
	path    string
	logger  *Logger
}

// NewHistoryManager 创建历史管理器并立即加载磁盘上的记录。
//
// 参数：
//   - logger: 日志记录器，可为 nil
//
// 返回值：
//   - *HistoryManager: 已完成加载的管理器，任何情况下均非 nil
//   - error:           仅当数据目录不可用时返回。历史文件本身的
//     缺失或损坏会降级为「从空列表开始」而不报错
//
// NOTE: 即使返回 error 也不应视为致命——历史记录是辅助功能，
// 读不出来不该阻断用户安装游戏。
func NewHistoryManager(logger *Logger) (*HistoryManager, error) {
	hm := &HistoryManager{
		records: []GameRecord{},
		logger:  logger,
	}

	dir, err := appDataDir()
	if err != nil {
		hm.warnf("数据目录不可用，安装历史将无法保存: %v", err)
		return hm, err
	}
	hm.path = filepath.Join(dir, HistoryFileName)

	hm.load()
	return hm, nil
}

// load 从磁盘读取历史记录到内存。
//
// 出错时保留空列表并记录日志，不向上传播。
func (hm *HistoryManager) load() {
	data, err := os.ReadFile(hm.path)
	if err != nil {
		if !os.IsNotExist(err) {
			hm.warnf("读取安装历史失败，从空列表开始: %v", err)
		}
		return
	}

	var records []GameRecord
	if err := json.Unmarshal(data, &records); err != nil {
		// 保留损坏文件供排查，不覆盖。
		hm.warnf("安装历史解析失败，从空列表开始（原文件保留待查）: %v", err)
		return
	}

	hm.records = records
	hm.logf("安装历史加载成功，共 %d 条记录", len(records))
}

// logf 在 logger 可用时记录信息级日志。
func (hm *HistoryManager) logf(format string, args ...any) {
	if hm.logger != nil {
		hm.logger.Info(format, args...)
	}
}

// warnf 在 logger 可用时记录警告级日志。
func (hm *HistoryManager) warnf(format string, args ...any) {
	if hm.logger != nil {
		hm.logger.Warn(format, args...)
	}
}

// ============================================================
// 公开 API
// ============================================================

// List 返回全部历史记录，按最近安装时间倒序排列。
//
// 返回深拷贝，调用方可安全地修改结果而不影响内部状态。
// 倒序是因为用户最关心刚装的东西，前端直接顺序渲染即可。
func (hm *HistoryManager) List() []GameRecord {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	out := make([]GameRecord, len(hm.records))
	copy(out, hm.records)

	// RFC 3339 是字典序可比的格式：年-月-日在前且位宽固定，
	// 因此直接比较字符串即可得到正确的时间顺序，无需解析为 time.Time。
	sort.Slice(out, func(i, j int) bool {
		return out[i].InstalledAt > out[j].InstalledAt
	})
	return out
}

// Find 按主游戏 AppID 查找历史记录。
//
// 返回值：
//   - *GameRecord: 找到时返回副本指针，未找到返回 nil
//
// 典型用途是用户再次导入同一游戏的清单包时，带出上次勾选的
// DLC 列表作为默认选中项。
func (hm *HistoryManager) Find(mainAppID string) *GameRecord {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	for i := range hm.records {
		if hm.records[i].MainAppID == mainAppID {
			record := hm.records[i]
			return &record
		}
	}
	return nil
}

// Record 写入或更新一条安装记录。
//
// 参数：
//   - gp:          已部署的清单包
//   - selectedIDs: 实际部署的 DLC AppID 列表
//   - luaFileName: 部署产生的文件名（不含目录）
//   - source:      清单来源，取 RepoSource.Name 或 SourceLocalImport；
//     留空时回填 SourceLocalImport，因唯一的无源路径即本地导入
//
// 返回值：
//   - error: 清单包无效或落盘失败时返回
//
// 同一 mainAppID 已存在时执行覆盖而非追加，InstalledAt 刷新为当前时间。
// 落盘失败时内存中的记录仍会保留，保证本次运行期间界面显示正确。
func (hm *HistoryManager) Record(gp *GamePackage, selectedIDs []string, luaFileName, source string) error {
	if gp == nil || gp.MainAppID == "" {
		return ErrEmptyPackage
	}

	if source == "" {
		source = SourceLocalImport
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	// 复制一份 selectedIDs：调用方的切片可能在之后被复用或修改，
	// 直接持有引用会让历史记录被悄悄篡改。
	ids := make([]string, len(selectedIDs))
	copy(ids, selectedIDs)

	record := GameRecord{
		MainAppID:    gp.MainAppID,
		GameName:     gp.GameName,
		DLCCount:     len(gp.DLCs),
		InstalledIDs: ids,
		InstalledAt:  time.Now().Format(time.RFC3339),
		LuaFileName:  luaFileName,
		Source:       source,
	}

	if idx := hm.indexOf(gp.MainAppID); idx >= 0 {
		hm.records[idx] = record
		hm.logf("安装记录已更新: %s (%s)，DLC %d 项",
			gp.GameName, gp.MainAppID, len(ids))
	} else {
		hm.records = append(hm.records, record)
		hm.logf("安装记录已新增: %s (%s)，DLC %d 项",
			gp.GameName, gp.MainAppID, len(ids))
	}

	return hm.persist()
}

// Delete 移除指定游戏的历史记录。
//
// 参数：
//   - mainAppID: 主游戏 AppID
//
// 返回值：
//   - error: 落盘失败时返回。记录本就不存在时返回 nil（幂等）
//
// NOTE: 本方法只动历史记录，不删除已部署的清单文件——
// 那是 Deployer.Remove 的职责。调用方需自行保证两者的调用顺序，
// 建议先删文件再删记录，这样中途失败时用户还能从历史里重试。
func (hm *HistoryManager) Delete(mainAppID string) error {
	if mainAppID == "" {
		return fmt.Errorf("AppID 不能为空")
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	idx := hm.indexOf(mainAppID)
	if idx < 0 {
		hm.logf("历史中无 AppID %s 的记录，无需移除", mainAppID)
		return nil
	}

	name := hm.records[idx].GameName
	hm.records = append(hm.records[:idx], hm.records[idx+1:]...)
	hm.logf("安装记录已移除: %s (%s)", name, mainAppID)

	return hm.persist()
}

// Clear 清空全部历史记录。
//
// 供前端设置面板的「清空历史」功能使用。
// 同样不触碰已部署的清单文件。
func (hm *HistoryManager) Clear() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	count := len(hm.records)
	hm.records = []GameRecord{}
	hm.logf("已清空全部安装历史（%d 条）", count)

	return hm.persist()
}

// indexOf 返回指定 AppID 在记录切片中的下标，未找到返回 -1。
//
// 调用方必须已持有锁。
func (hm *HistoryManager) indexOf(mainAppID string) int {
	for i := range hm.records {
		if hm.records[i].MainAppID == mainAppID {
			return i
		}
	}
	return -1
}

// persist 将内存中的历史记录序列化写入磁盘。
//
// 调用方必须已持有写锁。
func (hm *HistoryManager) persist() error {
	if hm.path == "" {
		return fmt.Errorf("数据目录不可用，安装历史无法保存")
	}

	data, err := json.MarshalIndent(hm.records, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化安装历史失败: %w", err)
	}

	if err := atomicWriteFile(hm.path, data); err != nil {
		if hm.logger != nil {
			hm.logger.Error("安装历史写入失败: %v", err)
		}
		return err
	}
	return nil
}
