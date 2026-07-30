package main

import (
	"strings"
	"testing"
)

// withBuildVars 临时替换构建信息变量并在测试结束时还原。
//
// 这些是包级变量，直接改会影响同包内其他测试。t.Cleanup 保证还原，
// 即便断言失败提前返回也不会漏。
func withBuildVars(t *testing.T, version, commit, builtAt, dirty string) {
	t.Helper()

	oldV, oldC, oldB, oldD := appVersion, appCommit, appBuiltAt, appDirty
	t.Cleanup(func() {
		appVersion, appCommit, appBuiltAt, appDirty = oldV, oldC, oldB, oldD
	})

	appVersion, appCommit, appBuiltAt, appDirty = version, commit, builtAt, dirty
}

// TestBuildInfoLabel 锁定构建标识的组装规则。
//
// label 会被用户抄进报障消息，也会写入诊断包，两处必须一致——故它由后端
// 单点组装。本测试即是那个单点的回归锁。
func TestBuildInfoLabel(t *testing.T) {
	cases := []struct {
		name    string
		version string
		commit  string
		dirty   string
		want    string
	}{
		{
			name:    "正常发布包",
			version: "2.0.0-rc.1",
			commit:  "d0e73d4",
			dirty:   "false",
			want:    "2.0.0-rc.1 (d0e73d4)",
		},
		{
			name:    "工作树有改动",
			version: "2.0.0-rc.1",
			commit:  "d0e73d4",
			dirty:   "true",
			want:    "2.0.0-rc.1 (d0e73d4) [已修改]",
		},
		{
			// 直接 go build 而未走发布脚本的情形。此时不应拼出
			// "dev (unknown)" 这种既无信息又占位置的串。
			name:    "未注入任何信息",
			version: "dev",
			commit:  "unknown",
			dirty:   "",
			want:    "dev",
		},
		{
			name:    "有版本号但无哈希",
			version: "2.0.0",
			commit:  "",
			dirty:   "false",
			want:    "2.0.0",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withBuildVars(t, c.version, c.commit, "2026-07-30T09:52:05Z", c.dirty)

			got := currentBuildInfo()

			// 开发构建下 label 会追加 [开发构建]，故用前缀比对。
			// 该后缀由构建标签决定，无法在测试中切换。
			if !strings.HasPrefix(got.Label, c.want) {
				t.Errorf("Label = %q，期望以 %q 开头", got.Label, c.want)
			}
		})
	}
}

// TestBuildInfoDirtyParsing 验证 dirty 仅在注入值为 "true" 时成立。
//
// 这一条容易写错成「非空即为真」，而发布脚本在工作树干净时注入的是
// 字符串 "false"——按非空判断会把每个正常包都标成已修改，进而使这个
// 警告彻底失去意义（用户看惯了就不再当回事）。
func TestBuildInfoDirtyParsing(t *testing.T) {
	cases := []struct {
		injected string
		want     bool
	}{
		{"true", true},
		{"false", false},
		{"", false},
		{"TRUE", false}, // 只认小写，与脚本注入值严格对应
	}

	for _, c := range cases {
		t.Run("注入值_"+c.injected, func(t *testing.T) {
			withBuildVars(t, "2.0.0", "abc1234", "2026-07-30T09:52:05Z", c.injected)

			if got := currentBuildInfo().Dirty; got != c.want {
				t.Errorf("Dirty = %v，期望 %v（注入值 %q）", got, c.want, c.injected)
			}
		})
	}
}

// TestBuildInfoFieldsPassThrough 验证各字段原样透传，不被加工。
//
// 诊断包会逐项打印这些字段，若某处做了截断或格式化，排障者拿到的信息
// 就与实际注入值不符。
func TestBuildInfoFieldsPassThrough(t *testing.T) {
	withBuildVars(t, "2.0.0-rc.2", "9f8e7d6", "2026-07-30T12:00:00Z", "false")

	bi := currentBuildInfo()

	if bi.Version != "2.0.0-rc.2" {
		t.Errorf("Version = %q", bi.Version)
	}
	if bi.Commit != "9f8e7d6" {
		t.Errorf("Commit = %q", bi.Commit)
	}
	if bi.BuiltAt != "2026-07-30T12:00:00Z" {
		t.Errorf("BuiltAt = %q", bi.BuiltAt)
	}
	if bi.DevBuild != isDevBuild {
		t.Errorf("DevBuild = %v，应与构建标签一致（%v）", bi.DevBuild, isDevBuild)
	}
}
