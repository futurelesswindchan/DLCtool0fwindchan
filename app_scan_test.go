// app_scan_test.go
//
// 覆盖部署目录对账的纯逻辑部分。ScanDeployed 本身需要文件系统与
// App 实例，故此处只测可独立验证的归属判定与文件名解析。

package main

import (
	"reflect"
	"sort"
	"testing"
)

func TestMainAppIDFromFileName(t *testing.T) {
	cases := []struct {
		fileName string
		want     string
	}{
		{"ARK_Survival_Ascended_2399830.lua", "2399830"},
		{"2399830.lua", "2399830"},
		{"Street_Fighter_6_1364780.LUA", "1364780"},
		// 下划线后非数字：无从判定，交由内容推断
		{"my_notes.lua", ""},
		{"random.lua", ""},
	}

	for _, c := range cases {
		t.Run(c.fileName, func(t *testing.T) {
			if got := mainAppIDFromFileName(c.fileName); got != c.want {
				t.Errorf("mainAppIDFromFileName(%q) = %q, 期望 %q",
					c.fileName, got, c.want)
			}
		})
	}
}

func TestBuildDeployedEntry(t *testing.T) {
	const script = `addappid(2399830, 1, "key")
addappid(2881150)
`
	known := map[string]struct{}{"2399830": {}}

	t.Run("本工具产物且有历史记录", func(t *testing.T) {
		got := buildDeployedEntry("ARK_2399830.lua", script, known)

		if got.MainAppID != "2399830" {
			t.Errorf("MainAppID = %q, 期望 2399830", got.MainAppID)
		}
		if got.IsExternal {
			t.Error("IsExternal = true，符合命名格式的文件不应判为外部")
		}
		if !got.InHistory {
			t.Error("InHistory = false，该 AppID 存在于历史中")
		}

		ids := append([]string{}, got.AppIDs...)
		sort.Strings(ids)
		if want := []string{"2399830", "2881150"}; !reflect.DeepEqual(ids, want) {
			t.Errorf("AppIDs = %v, 期望 %v", ids, want)
		}
	})

	// 纯数字命名是外部清单的典型形态。此前按 `_<AppID>.lua` 后缀定位的
	// 实现会完全漏掉这类文件，导致界面显示未安装而 Steam 中实际已装。
	t.Run("纯数字命名判为外部", func(t *testing.T) {
		got := buildDeployedEntry("2399830.lua", script, known)

		if got.MainAppID != "2399830" {
			t.Errorf("MainAppID = %q, 期望 2399830", got.MainAppID)
		}
		if !got.IsExternal {
			t.Error("IsExternal = false，纯数字命名不属本工具格式")
		}
		if !got.InHistory {
			t.Error("InHistory = false，该 AppID 存在于历史中")
		}
	})

	t.Run("文件名无法解析时退用内容首个声明", func(t *testing.T) {
		got := buildDeployedEntry("whatever.lua", script, known)

		if got.MainAppID != "2399830" {
			t.Errorf("MainAppID = %q, 期望回退为内容首个 AppID 2399830", got.MainAppID)
		}
		if !got.IsExternal {
			t.Error("IsExternal = false，该命名不属本工具格式")
		}
	})
}
