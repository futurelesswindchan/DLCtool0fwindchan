package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseLuaFile_TwoArgSetManifestid 验证解析器能正确处理两参数形态的
// setManifestid（GitHub 快照源的实际格式），不因缺少第三参数而报错，
// 且主游戏识别不受「主游戏 addappid 无 key」影响。
func TestParseLuaFile_TwoArgSetManifestid(t *testing.T) {
	// 这段 lua 来自 SSMGAlt/ManifestHub2 的 ARK(2399830) 实测输出。
	// 特征：setManifestid 只有两个参数，无 fileSize。
	const luaContent = `addappid(2399830)
setManifestid(228989,"550968249685141759")
setManifestid(228990,"1829726630299308803")
addappid(2399831,0,"320e0bcc46f1e7d88e18488c73990878eedbc06c3284015e1d0af97abe143e23")
setManifestid(2399831,"4735635872933062882")
`

	dir := t.TempDir()
	luaPath := filepath.Join(dir, "2399830.lua")
	if err := os.WriteFile(luaPath, []byte(luaContent), 0644); err != nil {
		t.Fatalf("写入临时 lua 文件失败: %v", err)
	}

	app := &App{}
	gp, err := app.parseLuaFile(luaPath)
	if err != nil {
		t.Fatalf("parseLuaFile 返回错误（两参数 setManifestid 应不报错）: %v", err)
	}

	// 主游戏取第一个 addappid，即便它不带密钥。
	if gp.MainAppID != "2399830" {
		t.Errorf("MainAppID = %q, want %q", gp.MainAppID, "2399830")
	}
	// 快照源的主游戏行无密钥，MainKey 应为空。部署器会据此输出单行
	// addappid 并记警告，与源文件原样一致。
	if gp.MainKey != "" {
		t.Errorf("MainKey = %q, want 空（快照源主游戏行不带密钥）", gp.MainKey)
	}

	// 2399831 带 key 且有两参数 setManifestid，应成为有效 Depot，
	// 其 FileSize 因源未提供而为 0。
	var depot *DepotInfo
	for i := range gp.Depots {
		if gp.Depots[i].DepotID == "2399831" {
			depot = &gp.Depots[i]
			break
		}
	}
	if depot == nil {
		t.Fatalf("未找到 DepotID 2399831，实际 Depots: %+v", gp.Depots)
	}
	if depot.ManifestID != "4735635872933062882" {
		t.Errorf("ManifestID = %q, want %q", depot.ManifestID, "4735635872933062882")
	}
	if depot.FileSize != 0 {
		t.Errorf("FileSize = %d, want 0（两参数形态无此值）", depot.FileSize)
	}
}

// TestParseLuaFile_MixedSetManifestidArgs 验证同一脚本中两参数与三参数
// setManifestid 混合出现时均正确解析，fileSize 缺失时为 0。
func TestParseLuaFile_MixedSetManifestidArgs(t *testing.T) {
	const luaContent = `-- test
-- Mixed Args Game
addappid(100000, 1, "mainkey123")
addappid(100001, 1, "depotkey456")
setManifestid(100001, "9999999999", 12345678)
addappid(100002, 1, "depotkey789")
setManifestid(100002, "8888888888")
addappid(200001)
`

	dir := t.TempDir()
	luaPath := filepath.Join(dir, "100000.lua")
	if err := os.WriteFile(luaPath, []byte(luaContent), 0644); err != nil {
		t.Fatalf("写入临时 lua 文件失败: %v", err)
	}

	app := &App{}
	gp, err := app.parseLuaFile(luaPath)
	if err != nil {
		t.Fatalf("parseLuaFile 返回错误: %v", err)
	}

	if gp.MainAppID != "100000" {
		t.Fatalf("MainAppID = %q, want %q", gp.MainAppID, "100000")
	}

	// 期望两个 Depot
	if len(gp.Depots) != 2 {
		t.Fatalf("len(Depots) = %d, want 2", len(gp.Depots))
	}

	for _, d := range gp.Depots {
		switch d.DepotID {
		case "100001":
			if d.ManifestID != "9999999999" {
				t.Errorf("Depot 100001 ManifestID = %q, want %q", d.ManifestID, "9999999999")
			}
			if d.FileSize != 12345678 {
				t.Errorf("Depot 100001 FileSize = %d, want %d", d.FileSize, 12345678)
			}
		case "100002":
			if d.ManifestID != "8888888888" {
				t.Errorf("Depot 100002 ManifestID = %q, want %q", d.ManifestID, "8888888888")
			}
			if d.FileSize != 0 {
				t.Errorf("Depot 100002 FileSize = %d, want 0（两参数形态）", d.FileSize)
			}
		default:
			t.Errorf("意外的 DepotID: %q", d.DepotID)
		}
	}

	// DLC 注册
	if len(gp.DLCs) != 1 || gp.DLCs[0].AppID != "200001" {
		t.Errorf("DLCs 不符合预期: %+v", gp.DLCs)
	}
}

// TestParseLuaFile_ThreeArgSetManifestid 验证三参数形态（Hubcap 源）仍正常工作，
// 确保兼容改动没有破坏既有路径。
func TestParseLuaFile_ThreeArgSetManifestid(t *testing.T) {
	const luaContent = `-- test
-- Test Game
addappid(1364780, 1, "ab1ae48f")
addappid(1364781, 1, "cfec3971")
setManifestid(1364781, "4741141599989541719", 86803973402)
addappid(1792750)
`

	dir := t.TempDir()
	luaPath := filepath.Join(dir, "1364780.lua")
	if err := os.WriteFile(luaPath, []byte(luaContent), 0644); err != nil {
		t.Fatalf("写入临时 lua 文件失败: %v", err)
	}

	app := &App{}
	gp, err := app.parseLuaFile(luaPath)
	if err != nil {
		t.Fatalf("parseLuaFile 返回错误: %v", err)
	}

	if gp.MainAppID != "1364780" {
		t.Errorf("MainAppID = %q, want %q", gp.MainAppID, "1364780")
	}

	found := false
	for _, d := range gp.Depots {
		if d.DepotID == "1364781" {
			found = true
			if d.ManifestID != "4741141599989541719" {
				t.Errorf("ManifestID = %q, want %q", d.ManifestID, "4741141599989541719")
			}
			if d.FileSize != 86803973402 {
				t.Errorf("FileSize = %d, want %d", d.FileSize, 86803973402)
			}
			break
		}
	}
	if !found {
		t.Error("未找到 DepotID 1364781")
	}

	// DLC 注册
	if len(gp.DLCs) == 0 {
		t.Fatal("DLCs 为空，期望至少 1 个")
	}
	if gp.DLCs[0].AppID != "1792750" {
		t.Errorf("DLCs[0].AppID = %q, want %q", gp.DLCs[0].AppID, "1792750")
	}
}
