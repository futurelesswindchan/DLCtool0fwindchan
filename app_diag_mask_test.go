package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestDescribeTokenState 锁定凭据状态描述不泄露任何原值片段。
//
// 这是本文件最重要的一条：describeTokenState 的返回值会进入诊断包，
// 而诊断包会被发到群里。一旦有人为了「方便排障」在此拼上密钥前几位，
// 本测试即失败。
func TestDescribeTokenState(t *testing.T) {
	const secret = "abcdef0123456789abcdef0123456789"

	cases := []struct {
		name        string
		token       string
		wantPresent bool
	}{
		{"空凭据", "", false},
		{"短凭据", "abc123", true},
		{"正常凭据", secret, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, present := describeTokenState(c.token)

			if present != c.wantPresent {
				t.Errorf("present = %v，期望 %v", present, c.wantPresent)
			}
			if state == "" {
				t.Error("状态描述不应为空——空白会让阅读者误以为采集失败")
			}

			// 核心断言：状态描述中不得出现原值的任何可识别片段。
			if c.token != "" && strings.Contains(state, c.token) {
				t.Errorf("状态描述泄露了完整凭据: %q", state)
			}
			if len(c.token) >= 8 && strings.Contains(state, c.token[:8]) {
				t.Errorf("状态描述泄露了凭据前 8 位: %q", state)
			}
		})
	}
}

// TestMaskedConfigOmitsToken 验证脱敏投影序列化后不含 token 字段与原值。
//
// 直接检查序列化后的 JSON 文本而非结构体字段：诊断包里躺着的是文本，
// 结构体上「没有 Token 字段」与「JSON 里不出现密钥」是两件事——
// 例如误加了 json 内联标签就会让前者成立而后者失败。
func TestMaskedConfigOmitsToken(t *testing.T) {
	const secret = "s3cr3t-token-value-should-never-appear"

	cfg := &AppConfig{
		SteamPath: `D:\steam`,
		Theme:     "dark",
		RepoSources: []RepoSource{
			{Name: "Hubcap", Kind: KindAPIZip, Repo: "https://example.com", Token: secret, Enabled: true},
			{Name: "MAU", Kind: KindGitHubBranch, Repo: "owner/name", Enabled: true},
		},
	}

	out := maskedConfig{
		SteamPath:   cfg.SteamPath,
		Theme:       cfg.Theme,
		RepoSources: make([]maskedRepoSource, 0, len(cfg.RepoSources)),
	}
	for _, s := range cfg.RepoSources {
		state, _ := describeTokenState(s.Token)
		out.RepoSources = append(out.RepoSources, maskedRepoSource{
			Name:       s.Name,
			Kind:       string(s.Kind),
			Repo:       s.Repo,
			Enabled:    s.Enabled,
			TokenState: state,
		})
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	text := string(data)

	if strings.Contains(text, secret) {
		t.Fatalf("脱敏后的 JSON 仍含原始凭据:\n%s", text)
	}
	if strings.Contains(strings.ToLower(text), `"token"`) {
		t.Errorf("脱敏后的 JSON 不应出现 token 字段:\n%s", text)
	}

	// 源名与仓库地址是排障必需信息，必须保留——脱敏过度会让诊断包失去价值。
	if !strings.Contains(text, "Hubcap") || !strings.Contains(text, "owner/name") {
		t.Errorf("脱敏丢失了必要的排障信息:\n%s", text)
	}
}

// TestCountEnabledSources 覆盖启用计数的三种情形。
func TestCountEnabledSources(t *testing.T) {
	cases := []struct {
		name    string
		sources []RepoSource
		want    int
	}{
		{"空列表", nil, 0},
		{"全部启用", []RepoSource{{Enabled: true}, {Enabled: true}}, 2},
		{"部分启用", []RepoSource{{Enabled: true}, {Enabled: false}, {Enabled: true}}, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := countEnabledSources(c.sources); got != c.want {
				t.Errorf("countEnabledSources = %d，期望 %d", got, c.want)
			}
		})
	}
}

// TestFallbackText 锁定空值占位行为。
//
// 只含空白的字符串也应触发占位：配置里残留的空格在报告中呈现为空白行，
// 与「未设置」在视觉上无从区分。
func TestFallbackText(t *testing.T) {
	if got := fallbackText("", "（未设置）"); got != "（未设置）" {
		t.Errorf("空字符串未触发占位，得到 %q", got)
	}
	if got := fallbackText("   ", "（未设置）"); got != "（未设置）" {
		t.Errorf("纯空白未触发占位，得到 %q", got)
	}
	if got := fallbackText(`D:\steam`, "（未设置）"); got != `D:\steam` {
		t.Errorf("有效值被替换，得到 %q", got)
	}
}

// TestWriteZipEntry 验证条目内容原样写入且不带 BOM。
//
// BOM 会让 config.masked.json 无法被 json 解析器读取，而这个失败要等到
// 有人真的去解析诊断包时才会暴露。
func TestWriteZipEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	const content = "中文内容 test 123"
	if err := writeZipEntry(zw, "a/b.txt", content); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("收尾失败: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("读取 zip 失败: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("条目数 = %d，期望 1", len(zr.File))
	}
	if zr.File[0].Name != "a/b.txt" {
		t.Errorf("条目名 = %q，期望 a/b.txt", zr.File[0].Name)
	}

	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("打开条目失败: %v", err)
	}
	defer rc.Close()

	var got bytes.Buffer
	if _, err := got.ReadFrom(rc); err != nil {
		t.Fatalf("读取条目失败: %v", err)
	}
	if got.String() != content {
		t.Errorf("内容不一致：得到 %q，期望 %q", got.String(), content)
	}
	if bytes.HasPrefix(got.Bytes(), []byte{0xEF, 0xBB, 0xBF}) {
		t.Error("内容带 UTF-8 BOM，会导致 json 解析失败")
	}
}
