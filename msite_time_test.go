package main

import (
	"testing"
	"time"
)

// TestParseMSiteTime_EasternOffset 验证不带时区的时间戳按美东时间解析，
// 且夏令时与冬令时各自取到正确的偏移。
//
// 该站返回的时间戳无时区标识，若按 UTC 解析会使算出的到期时刻偏早
// 4~5 小时。本测试锁定偏移量，避免将来有人「顺手」改回 UTC。
func TestParseMSiteTime_EasternOffset(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantOffset int // 期望的时区偏移（秒）
	}{
		{
			// 7 月属夏令时，EDT = UTC-4
			name:       "夏令时 EDT",
			input:      "2026-07-30T15:50:50.528769",
			wantOffset: -4 * 3600,
		},
		{
			// 1 月属冬令时，EST = UTC-5
			name:       "冬令时 EST",
			input:      "2026-01-15T15:50:50.528769",
			wantOffset: -5 * 3600,
		},
		{
			// 不带小数秒的形式
			name:       "无小数秒",
			input:      "2026-07-30T15:50:50",
			wantOffset: -4 * 3600,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseMSiteTime(c.input)
			if err != nil {
				t.Fatalf("parseMSiteTime(%q) 返回错误: %v", c.input, err)
			}
			_, offset := got.Zone()
			if offset != c.wantOffset {
				t.Errorf("时区偏移 = %d 秒, want %d 秒（解析结果 %s）",
					offset, c.wantOffset, got.Format(time.RFC3339))
			}
		})
	}
}

// TestParseMSiteTime_WithExplicitZone 验证输入自带时区标识时以其为准，
// 不被 msiteLocation 覆盖。ParseInLocation 的既有语义如此，此处做回归锁定。
func TestParseMSiteTime_WithExplicitZone(t *testing.T) {
	got, err := parseMSiteTime("2026-07-30T15:50:50Z")
	if err != nil {
		t.Fatalf("解析带 Z 后缀的时间失败: %v", err)
	}
	if _, offset := got.Zone(); offset != 0 {
		t.Errorf("带 Z 后缀应解析为 UTC，实际偏移 %d 秒", offset)
	}
}

// TestParseMSiteTime_Invalid 验证空串与无法识别的格式均返回错误。
func TestParseMSiteTime_Invalid(t *testing.T) {
	for _, s := range []string{"", "   ", "not-a-time", "2026/07/30 15:50"} {
		if _, err := parseMSiteTime(s); err == nil {
			t.Errorf("parseMSiteTime(%q) 应返回错误", s)
		}
	}
}
