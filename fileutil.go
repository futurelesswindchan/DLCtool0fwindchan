// fileutil.go
//
// 本文件提供跨模块复用的文件操作工具函数。
//
// 存在意义：配置持久化（config.go）、历史记录（history.go）和
// 清单部署（deployer_ost.go）都需要「保证文件要么是旧内容、要么是
// 完整新内容，绝不出现半截状态」的写入语义，故抽取到此处统一实现。

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// webviewDataDir 返回 WebView2 用户数据的存放目录。
//
// 统一收拢到 ~/.kazeusa/webview2/，保证本工具产生的全部文件
// 都在同一棵目录树下，卸载时删一个文件夹即可彻底清理。
//
// 返回值：
//   - string: 目录完整路径；数据目录不可用时返回空字符串，
//     此时 Wails 会退回 WebView2 的默认行为
//
// NOTE: 此函数在 wails.Run 之前调用，此时 logger 尚未就绪，
// 故失败时静默返回空字符串而不记录日志。
func webviewDataDir() string {
	dir, err := appDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "webview2")
}

// cleanStaleTempDirs 清理 %TEMP% 下遗留的解压临时目录。
//
// 正常流程中临时目录由 defer os.RemoveAll 回收，但进程被强制
// 结束（任务管理器结束进程、崩溃、断电）时来不及执行，会留下残留。
// 本函数在启动时做一次兜底扫描。
//
// 参数：
//   - maxAge: 目录的最小存留时长，只清理修改时间早于此阈值的目录。
//     设置门槛是为了避免误删同时运行的另一个实例正在使用的目录
//
// 返回值：
//   - int: 成功清理的目录数量
//
// 安全性：仅匹配 TempDirPrefix 前缀且位于系统临时目录下的项，
// 不会触碰其他程序的文件。删除失败（如被占用）只跳过，不报错。
func cleanStaleTempDirs(maxAge time.Duration) int {
	tempRoot := os.TempDir()
	entries, err := os.ReadDir(tempRoot)
	if err != nil {
		return 0
	}

	deadline := time.Now().Add(-maxAge)
	cleaned := 0

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), TempDirPrefix) {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(deadline) {
			continue
		}

		if err := os.RemoveAll(filepath.Join(tempRoot, entry.Name())); err == nil {
			cleaned++
		}
	}

	return cleaned
}

// atomicWriteFile 以原子方式将数据写入指定路径。
//
// 实现策略：先写入同目录下的临时文件，fsync 落盘后再 rename 覆盖目标。
// 临时文件与目标文件必须同处一个目录——跨卷 rename 在 Windows 上
// 会退化为「复制 + 删除」，丧失原子性。
//
// 参数：
//   - path: 目标文件完整路径，其所在目录会被自动创写入的完整内容
//
// 返回值：
//   - error: 目录创建、写入、同步或重命名任一环节失败时返回
//
// 对 OST 热重载的意义：
//
//	OST 的 LuaFileWatcher 基于 ReadDirectoryChangesW 事件驱动并带
//	500ms 防抖。若直接以 O_TRUNC 打开目标文件再逐步写入，OST 可能
//	在内容写完前就被触发，读到不完整的 Lua 而解析失败。
//	经由 rename 提交后，OST 只会收到一次 RenamedNewName 事件，
//	且此刻文件内容已完整。
//
// XXX: Windows 上若目标文件正被其他进程以独占方式打开，rename 会失败。
// 正常情况下 OST 只做短暂的读取，不会长期持有句柄。
func atomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败 %s: %w", dir, err)
	}

	tmpPath := path + TempFileSuffix

	// 清理可能残留的上次失败产物，否则 OpenFile 会因已存在而困扰后续判断。
	_ = os.Remove(tmpPath)

	if err := writeAndSync(tmpPath, data); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("提交文件失败 %s: %w", path, err)
	}

	return nil
}

// writeAndSync 将数据写入文件并强制刷入磁盘。
//
// Sync 调用不可省略：仅 Close 只保证数据交给了操作系统缓存，
// 若此刻断电或系统崩溃，rename 后的文件可能是空的。
func writeAndSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("创建临时文件失败 %s: %w", path, err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("写入临时文件失败 %s: %w", path, err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("同步临时文件到磁盘失败 %s: %w", path, err)
	}

	return nil
}

// fileExists 判断指定路径是否存在且为普通文件。
//
// 目录会返回 false——环境检测需要确认的是 DLL 文件本身，
// 若用户误建了同名目录不应被判定为「已安装」。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}