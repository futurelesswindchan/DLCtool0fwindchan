package main

import "testing"

// TestNormalizeTheme 验证主题取值的归一化规则。
//
// 这组用例源自一个实机缺陷（2026-08-01）：normalize 原先只认 "dark" 与
// "light"，于是前端传来的 "system" 被静默改回 "dark"。
//
// 症状与根因隔了一层，很值得记下来：用户在设置页点「跟随系统」后，界面先
// 闪成浅色（前端已按系统偏好把 data-theme 落好），随即跳回深色（save 之后
// refresh 拉回了被后端纠正的值）。整个过程没有任何报错，看起来完全像是
// 前端的主题切换逻辑写坏了，而真正的原因在这个函数里。
//
// 故此测试的作用不只是防回归，更是把「三档」这个契约钉在后端——
// 前端的 Theme 类型是三档，后端就必须如实存取三档。
func TestNormalizeTheme(t *testing.T) {
	cm := &ConfigManager{}

	t.Run("三个合法档位均原样保留", func(t *testing.T) {
		for _, want := range []string{"dark", "light", "system"} {
			cfg := &AppConfig{Theme: want}
			cm.normalize(cfg)

			if cfg.Theme != want {
				t.Errorf("主题 %q 被改写为 %q，合法取值不应被纠正", want, cfg.Theme)
			}
		}
	})

	t.Run("非法取值回退到默认主题", func(t *testing.T) {
		// 空字符串来自旧版配置或人工编辑 config.json 后的缺失字段
		for _, bad := range []string{"", "Dark", "auto", "blue"} {
			cfg := &AppConfig{Theme: bad}
			cm.normalize(cfg)

			if cfg.Theme != defaultTheme {
				t.Errorf("非法主题 %q 归一化为 %q，期望 %q", bad, cfg.Theme, defaultTheme)
			}
		}
	})

	t.Run("默认配置的主题合法且为浅色", func(t *testing.T) {
		cfg := defaultConfig()

		// 默认取浅色而非深色：生产力工具默认深色几乎是行业惯例，但本工具的
		// 目标用户含完全不懂 Steam 目录结构的人，深色对这类用户有额外的
		// 压迫感——它暗示「这是给专业人士的东西」。
		if cfg.Theme != "light" {
			t.Errorf("默认主题为 %q，期望 light", cfg.Theme)
		}

		// 前端 stores/config.ts 的兜底值与 styles/tokens/color.css 的裸 :root
		// 都按此值对齐，三者不一致会导致首屏跳色。
		if cfg.Theme != defaultTheme {
			t.Errorf("defaultConfig 的主题 %q 与 defaultTheme %q 不一致", cfg.Theme, defaultTheme)
		}
	})
}
