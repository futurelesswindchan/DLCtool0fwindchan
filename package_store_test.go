package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempDataDir 把数据目录临时指向 t.TempDir()。
//
// appDataDir 在开发构建下取 ~/.kazeusa，直接跑测试会污染真实数据目录。
// 此处经 USERPROFILE 改写 home，测试结束自动还原。
func withTempDataDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)
}

func samplePackage() *GamePackage {
	return &GamePackage{
		MainAppID: "780310",
		MainKey:   "deadbeef",
		GameName:  "The Riftbreaker",
		Depots: []DepotInfo{
			{DepotID: "780311", DecryptionKey: "aabb", ManifestID: "123", FileSize: 4096},
		},
		DLCs: []DLCInfo{
			{AppID: "2506610", Name: "Heart of the Swamp", ManifestID: "456"},
			{AppID: "1554430", Name: "Soundtrack"},
		},
	}
}

// TestPackageStoreRoundTrip 验证留存的写入与读回保持一致。
//
// 重点是 DLC 与 Depot 的完整往返——MainKey 与 ManifestID 缺失时不会
// 报错，只会静默产出无效脚本，故必须逐字段确认。
func TestPackageStoreRoundTrip(t *testing.T) {
	withTempDataDir(t)
	ps := NewPackageStore(nil)
	gp := samplePackage()

	if err := ps.Save(gp, "Hubcap Manifest"); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}

	stored, err := ps.Load("780310")
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}
	if stored == nil {
		t.Fatal("Load 返回 nil，期望读到留存")
	}

	if stored.Source != "Hubcap Manifest" {
		t.Errorf("Source = %q，期望 %q", stored.Source, "Hubcap Manifest")
	}
	if stored.SavedAt == "" {
		t.Error("SavedAt 为空，界面无从展示获取时间")
	}

	got := stored.Package
	if got.MainKey != gp.MainKey {
		t.Errorf("MainKey = %q，期望 %q", got.MainKey, gp.MainKey)
	}
	if len(got.DLCs) != len(gp.DLCs) {
		t.Fatalf("DLC 数 = %d，期望 %d", len(got.DLCs), len(gp.DLCs))
	}
	if got.DLCs[0].ManifestID != "456" {
		t.Errorf("DLC ManifestID = %q，期望 456", got.DLCs[0].ManifestID)
	}
	if len(got.Depots) != 1 || got.Depots[0].DecryptionKey != "aabb" {
		t.Errorf("Depot 未完整往返: %+v", got.Depots)
	}
}

// TestPackageStoreLoadMissing 验证「没有留存」不被当作错误。
//
// 该区分对界面很重要：返回 nil 且无错误时应引导用户获取清单，
// 而真出错时应提示重试。混为一谈会让用户看到莫名的错误提示。
func TestPackageStoreLoadMissing(t *testing.T) {
	withTempDataDir(t)
	ps := NewPackageStore(nil)

	stored, err := ps.Load("999999")
	if err != nil {
		t.Errorf("文件不存在时不应返回错误，得到: %v", err)
	}
	if stored != nil {
		t.Errorf("期望 nil，得到 %+v", stored)
	}
}

// TestPackageStoreLoadCorrupted 验证损坏的留存被识别为错误而非静默放行。
//
// 若返回一个字段残缺的 GamePackage，部署时会静默产出无效脚本——
// 症状要到 Steam 下载失败时才显现，排查成本极高。
func TestPackageStoreLoadCorrupted(t *testing.T) {
	withTempDataDir(t)
	ps := NewPackageStore(nil)

	path := packagePath("111111")
	if path == "" {
		t.Fatal("数据目录不可用")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	if _, err := ps.Load("111111"); err == nil {
		t.Error("损坏内容应返回错误")
	}
}

// TestPackageStoreDeleteIdempotent 验证删除不存在的留存不报错。
//
// 调用方的意图是「让它不存在」，已经不存在时该意图已然达成。
func TestPackageStoreDeleteIdempotent(t *testing.T) {
	withTempDataDir(t)
	ps := NewPackageStore(nil)

	if err := ps.Delete("404404"); err != nil {
		t.Errorf("删除不存在的留存不应报错，得到: %v", err)
	}

	if err := ps.Save(samplePackage(), ""); err != nil {
		t.Fatalf("Save 失败: %v", err)
	}
	if err := ps.Delete("780310"); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	stored, _ := ps.Load("780310")
	if stored != nil {
		t.Error("删除后仍能读到留存")
	}
}

// TestPackageStoreSaveInvalid 验证无效清单包被拒绝。
func TestPackageStoreSaveInvalid(t *testing.T) {
	withTempDataDir(t)
	ps := NewPackageStore(nil)

	if err := ps.Save(nil, ""); err == nil {
		t.Error("nil 清单包应被拒绝")
	}
	if err := ps.Save(&GamePackage{}, ""); err == nil {
		t.Error("缺少 MainAppID 的清单包应被拒绝")
	}
}
