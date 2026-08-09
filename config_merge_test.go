package main

import "testing"

// TestMergeNewBuiltinSources 验证内置源的合并规则：缺失者补入末尾，
// 已存在者原样保留。
//
// 最关键的一条是不得覆盖 Token 与 Enabled——前者是用户自行申请的凭据，
// 后者是用户明确表达的意愿，二者若因版本升级被重置，表现为「设置莫名
// 恢复默认」，而用户很难把这个现象与升级联系起来。
func TestMergeNewBuiltinSources(t *testing.T) {
	t.Run("补入缺失的内置源", func(t *testing.T) {
		// 模拟旧版配置：只有早期的四个源
		existing := []RepoSource{
			{Name: msiteSourceName, Kind: KindAPIZip, Repo: msiteBaseURL, Enabled: true},
			{Name: "ManifestHub", Kind: KindGitHubBranch, Repo: "SteamAutoCracks/ManifestHub", Enabled: false},
			{Name: "MAU", Kind: KindGitHubBranch, Repo: "Auiowu/ManifestAutoUpdate", Enabled: true},
			{Name: "MAU 镜像", Kind: KindGitHubBranch, Repo: "Satisl/MAU", Enabled: true},
		}

		merged := mergeNewBuiltinSources(existing)

		if len(merged) <= len(existing) {
			t.Fatalf("合并后源数为 %d，应多于原有的 %d", len(merged), len(existing))
		}

		// 原有条目须保持在前且顺序不变
		for i := range existing {
			if merged[i].Name != existing[i].Name {
				t.Errorf("第 %d 个源为 %q，期望保持 %q", i, merged[i].Name, existing[i].Name)
			}
		}

		// 内置源应全部出现在合并结果中
		got := make(map[string]struct{}, len(merged))
		for _, s := range merged {
			got[s.Name] = struct{}{}
		}
		for _, b := range defaultRepoSources() {
			if _, ok := got[b.Name]; !ok {
				t.Errorf("内置源 %q 未被补入", b.Name)
			}
		}
	})

	t.Run("不覆盖已有的 Token 与 Enabled", func(t *testing.T) {
		const userToken = "smm_user_own_credential"

		existing := []RepoSource{
			// 用户已填凭据
			{Name: msiteSourceName, Kind: KindAPIZip, Repo: msiteBaseURL, Token: userToken, Enabled: true},
			// 用户手动停用了 MAU
			{Name: "MAU", Kind: KindGitHubBranch, Repo: "Auiowu/ManifestAutoUpdate", Enabled: false},
		}

		merged := mergeNewBuiltinSources(existing)

		for _, s := range merged {
			switch s.Name {
			case msiteSourceName:
				if s.Token != userToken {
					t.Errorf("Token 被改写为 %q，应保持 %q", s.Token, userToken)
				}
			case "MAU":
				if s.Enabled {
					t.Error("用户已停用的 MAU 被重新启用")
				}
			}
		}
	})

	t.Run("已是最新时不产生重复条目", func(t *testing.T) {
		merged := mergeNewBuiltinSources(defaultRepoSources())

		if len(merged) != len(defaultRepoSources()) {
			t.Errorf("源数为 %d，期望 %d（不应重复追加）",
				len(merged), len(defaultRepoSources()))
		}

		seen := make(map[string]int, len(merged))
		for _, s := range merged {
			seen[s.Name]++
			if seen[s.Name] > 1 {
				t.Errorf("源 %q 出现 %d 次", s.Name, seen[s.Name])
			}
		}
	})

	t.Run("不修改传入的切片", func(t *testing.T) {
		existing := []RepoSource{
			{Name: "MAU", Kind: KindGitHubBranch, Repo: "Auiowu/ManifestAutoUpdate", Enabled: true},
		}
		_ = mergeNewBuiltinSources(existing)

		if len(existing) != 1 {
			t.Errorf("传入切片被修改，长度变为 %d", len(existing))
		}
	})
}
