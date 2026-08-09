package main

import "testing"

// TestCompareVersions 覆盖版本比较的常规与边界形态。
//
// 重点在段数不等与非数字后缀两类——它们是实际 tag 书写中真正会出现的
// 偏差，而纯粹的 2.0.0 与 2.0.1 比较不可能出错。
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int
	}{
		{"完全相同", "2.0.0", "2.0.0", 0},
		{"补丁号更高", "2.0.1", "2.0.0", 1},
		{"次版本号更高", "2.1.0", "2.0.9", 1},
		{"主版本号更高", "3.0.0", "2.99.99", 1},
		{"更低版本", "1.9.9", "2.0.0", -1},
		{"段数不等视缺失为零", "2.0", "2.0.0", 0},
		{"短版本号仍能比出高低", "2.1", "2.0.5", 1},
		{"多出的零段不影响相等", "2.0.0.0", "2.0", 0},
		{"两位数段不按字典序", "2.10.0", "2.9.0", 1},
		{"非数字后缀取前导数字", "2.1.0-beta", "2.0.0", 1},
		{"预发布与正式版判为相同", "2.0.0-rc1", "2.0.0", 0},
		{"整段非数字退化为零", "2.x.0", "2.0.0", 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compareVersions(c.a, c.b); got != c.want {
				t.Errorf("compareVersions(%q, %q) = %d, 期望 %d", c.a, c.b, got, c.want)
			}
		})
	}
}

// TestNormalizeVersion 确认 v 前缀与空白被剥离。
//
// 这一步若失效，"v2.0.0" 与 "2.0.0" 会因首字符不同而在比较中把 v 解析为
// 0，从而把同一个版本判成有更新——是最容易漏、也最容易误导用户的一处。
func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v2.0.0":    "2.0.0",
		"2.0.0":     "2.0.0",
		" v2.0.0 ":  "2.0.0",
		"\t2.1.3\n": "2.1.3",
		"":          "",
	}

	for in, want := range cases {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestFormatReleaseDate 确认时间戳只保留日期，且失败时返回空串。
//
// 返回空串而非原文是有意为之：把 "2026-07-30T12:00:00Z" 直接甩在界面上
// 比留空更糟。
func TestFormatReleaseDate(t *testing.T) {
	cases := map[string]string{
		"2026-07-30T12:34:56Z":      "2026-07-30",
		"2026-01-02T00:00:00+08:00": "2026-01-02",
		"":                          "",
		"2026-07-30":                "",
		"not-a-date":                "",
	}

	for in, want := range cases {
		if got := formatReleaseDate(in); got != want {
			t.Errorf("formatReleaseDate(%q) = %q, 期望 %q", in, got, want)
		}
	}
}
