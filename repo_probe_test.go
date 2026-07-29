package main

import "testing"

// TestNarrowByProbeResults 验证候选源的排除规则：只排除确定未收录者，
// 探测未得出结论的源必须保留候选资格。
//
// 该规则的失效方式是静默的——被误排除的源不会报错，只是永远不被尝试，
// 表现为「明明有清单却提示需要本地导入」，故必须有测试锁定。
func TestNarrowByProbeResults(t *testing.T) {
	sources := []RepoSource{
		{Name: "A", Kind: KindGitHubBranch, Repo: "a/a", Enabled: true},
		{Name: "B", Kind: KindGitHubBranch, Repo: "b/b", Enabled: true},
		{Name: "C", Kind: KindGitHubBranch, Repo: "c/c", Enabled: true},
	}

	cases := []struct {
		name    string
		results []probeResult
		want    []string
	}{
		{
			name:    "全部命中则全部保留",
			results: []probeResult{probeHit, probeHit, probeHit},
			want:    []string{"A", "B", "C"},
		},
		{
			name:    "明确未收录者被排除",
			results: []probeResult{probeHit, probeMiss, probeHit},
			want:    []string{"A", "C"},
		},
		{
			// 核心用例：B 超时（探测不明）不得被排除，
			// 它仍有机会在下载阶段经镜像链取到清单。
			name:    "探测不明者保留",
			results: []probeResult{probeHit, probeUnknown, probeMiss},
			want:    []string{"A", "B"},
		},
		{
			name:    "全部探测不明则全部保留",
			results: []probeResult{probeUnknown, probeUnknown, probeUnknown},
			want:    []string{"A", "B", "C"},
		},
		{
			// 全部明确未收录时不做排除，交由下载阶段给出失败结论。
			// 检测只是优化手段，不该拥有否决下载的权力。
			name:    "全部未收录则原样返回",
			results: []probeResult{probeMiss, probeMiss, probeMiss},
			want:    []string{"A", "B", "C"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := narrowByProbeResults(sources, sources, c.results)
			if len(got) != len(c.want) {
				t.Fatalf("保留 %d 个源，期望 %d 个（实际 %v）",
					len(got), len(c.want), sourceNames(got))
			}
			for i := range got {
				if got[i].Name != c.want[i] {
					t.Errorf("第 %d 个源为 %q，期望 %q（实际顺序 %v）",
						i, got[i].Name, c.want[i], sourceNames(got))
				}
			}
		})
	}
}

// sourceNames 提取源名称列表，仅供测试断言的错误信息使用。
func sourceNames(sources []RepoSource) []string {
	names := make([]string, 0, len(sources))
	for _, s := range sources {
		names = append(names, s.Name)
	}
	return names
}
