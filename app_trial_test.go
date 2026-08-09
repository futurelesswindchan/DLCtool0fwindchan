package main

import (
	"errors"
	"testing"
	"time"
)

// TestClassifyTrialError 锁定错误分类。
//
// 分类的目的不是精确，而是让用户知道下一步该做什么：miss / unsupported
// 该换源，failed 该检查网络。当前把这几类全塌成一句「获取清单失败」，
// 正是用户把源的贫瘠误判为工具故障的直接原因，故本测试防的是回退到那种
// 一刀切的实现。
func TestClassifyTrialError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus string
	}{
		{
			// 实测 MAU 镜像与 bingyu50 在 MHW 上即为此种
			name:       "缺 config.json",
			err:        errors.New("MAU 形态解析失败：包内未找到 config.json"),
			wantStatus: TrialUnsupported,
		},
		{
			name:       "源不存在",
			err:        errors.New(`源 "Foo" 不存在或未启用`),
			wantStatus: TrialMiss,
		},
		{
			name:       "404",
			err:        errors.New("下载失败: unexpected status 404"),
			wantStatus: TrialMiss,
		},
		{
			// 实测高频出现的网络故障
			name:       "连接被强制关闭",
			err:        errors.New("read tcp: wsarecv: An existing connection was forcibly closed"),
			wantStatus: TrialFailed,
		},
		{
			name:       "超时",
			err:        errors.New("Get https://example.com: context deadline exceeded"),
			wantStatus: TrialFailed,
		},
		{
			name:       "凭据失效",
			err:        errors.New("请求被拒绝: 401 unauthorized"),
			wantStatus: TrialFailed,
		},
		{
			name:       "未知错误兜底",
			err:        errors.New("something entirely unexpected"),
			wantStatus: TrialFailed,
		},
		{
			name:       "nil 错误",
			err:        nil,
			wantStatus: TrialFailed,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, message := classifyTrialError(c.err)

			if status != c.wantStatus {
				t.Errorf("status = %q，期望 %q", status, c.wantStatus)
			}
			if message == "" {
				t.Error("message 不应为空——界面需要一句可展示的说明")
			}
		})
	}
}

// TestSummarizeTrials 锁定汇总计算。
//
// 该函数的失效是静默的：算错 BestSource 不会报错，只是把用户引向一个更差
// 的源，而用户没有任何办法察觉这一点。
func TestSummarizeTrials(t *testing.T) {
	t.Run("正常挑出最多的源", func(t *testing.T) {
		r := &TrialReport{Trials: []SourceTrial{
			{Source: "A", Status: TrialOK, DLCCount: 64},
			{Source: "B", Status: TrialOK, DLCCount: 200},
			{Source: "C", Status: TrialEmpty, DLCCount: 0},
			{Source: "D", Status: TrialFailed},
		}}
		summarizeTrials(r)

		if r.BestSource != "B" {
			t.Errorf("BestSource = %q，期望 B", r.BestSource)
		}
		if r.MaxDLC != 200 {
			t.Errorf("MaxDLC = %d，期望 200", r.MaxDLC)
		}
		// ok 与 empty 都算可用：empty 源能部署本体，只是没有 DLC
		if r.UsableCount != 3 {
			t.Errorf("UsableCount = %d，期望 3", r.UsableCount)
		}
	})

	t.Run("并列时保留配置顺序在前者", func(t *testing.T) {
		r := &TrialReport{Trials: []SourceTrial{
			{Source: "先", Status: TrialOK, DLCCount: 64},
			{Source: "后", Status: TrialOK, DLCCount: 64},
		}}
		summarizeTrials(r)

		// 用严格大于比较，使结果稳定可复现——否则同样的数据两次刷新
		// 可能给出不同的推荐，用户会怀疑数据不可信
		if r.BestSource != "先" {
			t.Errorf("BestSource = %q，并列时应保留在前者", r.BestSource)
		}
	})

	t.Run("empty 源不作为最佳", func(t *testing.T) {
		r := &TrialReport{Trials: []SourceTrial{
			{Source: "空", Status: TrialEmpty, DLCCount: 0},
		}}
		summarizeTrials(r)

		if r.BestSource != "" {
			t.Errorf("BestSource = %q，无 DLC 的源不应被推荐", r.BestSource)
		}
		if r.UsableCount != 1 {
			t.Errorf("UsableCount = %d，期望 1（可部署本体）", r.UsableCount)
		}
	})

	t.Run("全部失败", func(t *testing.T) {
		r := &TrialReport{Trials: []SourceTrial{
			{Source: "A", Status: TrialFailed},
			{Source: "B", Status: TrialMiss},
			{Source: "C", Status: TrialUnsupported},
		}}
		summarizeTrials(r)

		if r.BestSource != "" || r.MaxDLC != 0 || r.UsableCount != 0 {
			t.Errorf("全失败时应为空汇总，得到 best=%q max=%d usable=%d",
				r.BestSource, r.MaxDLC, r.UsableCount)
		}
	})

	t.Run("空列表", func(t *testing.T) {
		r := &TrialReport{Trials: []SourceTrial{}}
		summarizeTrials(r)

		if r.BestSource != "" || r.MaxDLC != 0 || r.UsableCount != 0 {
			t.Error("空列表应产出空汇总")
		}
	})
}

// TestTrialCache 覆盖缓存的存取、过期与按游戏清除。
func TestTrialCache(t *testing.T) {
	t.Run("存入后可取出", func(t *testing.T) {
		c := newTrialCache()
		want := SourceTrial{Source: "A", Status: TrialOK, DLCCount: 64}
		c.put("582010", "A", want, &GamePackage{GameName: "MHW"})

		e, ok := c.get("582010", "A")
		if !ok {
			t.Fatal("刚存入的条目应能取出")
		}
		if e.trial.DLCCount != 64 {
			t.Errorf("DLCCount = %d，期望 64", e.trial.DLCCount)
		}
		if e.pkg == nil {
			t.Error("产物应一并缓存——否则用户选定源后仍要二次下载")
		}
	})

	t.Run("不同游戏互不干扰", func(t *testing.T) {
		c := newTrialCache()
		c.put("582010", "A", SourceTrial{Source: "A"}, nil)

		if _, ok := c.get("1367550", "A"); ok {
			t.Error("同名源在不同 AppID 下不应命中")
		}
	})

	t.Run("过期条目不命中", func(t *testing.T) {
		c := newTrialCache()
		c.put("582010", "A", SourceTrial{Source: "A"}, nil)

		// 直接改写时间戳而非等待，避免测试挂 30 分钟
		c.mu.Lock()
		e := c.entries[c.key("582010", "A")]
		e.at = time.Now().Add(-trialCacheTTL - time.Minute)
		c.entries[c.key("582010", "A")] = e
		c.mu.Unlock()

		if _, ok := c.get("582010", "A"); ok {
			t.Error("过期条目不应命中——对比表须反映当下的源状态")
		}
	})

	t.Run("drop 只清指定游戏", func(t *testing.T) {
		c := newTrialCache()
		c.put("582010", "A", SourceTrial{Source: "A"}, nil)
		c.put("582010", "B", SourceTrial{Source: "B"}, nil)
		c.put("1367550", "A", SourceTrial{Source: "A"}, nil)

		c.drop("582010")

		if _, ok := c.get("582010", "A"); ok {
			t.Error("582010 的条目应被清除")
		}
		if _, ok := c.get("582010", "B"); ok {
			t.Error("582010 的全部条目都应被清除")
		}
		if _, ok := c.get("1367550", "A"); !ok {
			t.Error("其他游戏的条目不应受影响——用户只想重查这一个游戏")
		}
	})
}
