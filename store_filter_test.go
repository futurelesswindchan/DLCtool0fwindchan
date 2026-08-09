package main

import "testing"

// TestIsMainGame 验证搜索结果的本体判据。
//
// 该判据直接决定用户能否搜到想要的游戏，误杀比漏放危险得多——搜不到
// 会让用户以为工具不支持该游戏，而多出一条干扰项只是稍显杂乱。
func TestIsMainGame(t *testing.T) {
	cases := []struct {
		name   string
		detail *GameDetail
		want   bool
		why    string
	}{
		{
			name:   "普通付费游戏",
			detail: &GameDetail{Type: "game", Name: "The Riftbreaker"},
			want:   true,
		},
		{
			name:   "DLC",
			detail: &GameDetail{Type: "dlc", Name: "Heart of the Swamp"},
			want:   false,
		},
		{
			name:   "试玩版",
			detail: &GameDetail{Type: "demo", Name: "Some Game Demo"},
			want:   false,
		},
		{
			name:   "原声音轨",
			detail: &GameDetail{Type: "music", Name: "Soundtrack"},
			want:   false,
		},
		{
			name:   "独立上架的免费序章",
			detail: &GameDetail{Type: "game", Name: "The Riftbreaker 银河破裂者：序章", IsFree: true},
			want:   false,
			why:   "Steam 把序章当独立免费游戏上架，type 同样是 game",
		},
		{
			name:   "英文 Prologue",
			detail: &GameDetail{Type: "game", Name: "Chernobylite: Prologue", IsFree: true},
			want:   false,
		},
		{
			name:   "付费游戏名中含序章",
			detail: &GameDetail{Type: "game", Name: "英雄传说：空之轨迹 序章", IsFree: false},
			want:   true,
			why:   "付费作品即便名字带序章也是正片，不可误杀",
		},
		{
			name:   "免费正片",
			detail: &GameDetail{Type: "game", Name: "Warframe", IsFree: true},
			want:   true,
			why:   "免费但名称无衍生品标记，应放行",
		},
		{
			name:   "类型未知",
			detail: &GameDetail{Type: "", Name: "780310"},
			want:   true,
			why:   "降级结果或旧缓存，无从判定时放行",
		},
		{
			name:   "nil",
			detail: nil,
			want:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isMainGame(c.detail); got != c.want {
				t.Errorf("isMainGame() = %v，期望 %v。%s", got, c.want, c.why)
			}
		})
	}
}

// TestHasDerivativeMarker 验证衍生品标记的匹配大小写不敏感。
func TestHasDerivativeMarker(t *testing.T) {
	for _, name := range []string{"Game DEMO", "Game Demo", "game prologue", "试玩版"} {
		if !hasDerivativeMarker(name) {
			t.Errorf("%q 应被识别为衍生品", name)
		}
	}
	for _, name := range []string{"The Riftbreaker", "猎人：荒野的召唤"} {
		if hasDerivativeMarker(name) {
			t.Errorf("%q 不应被识别为衍生品", name)
		}
	}
}
