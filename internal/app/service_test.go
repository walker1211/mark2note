package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/walker1211/mark2note/internal/ai"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/deck"
	"github.com/walker1211/mark2note/internal/poster"
	"github.com/walker1211/mark2note/internal/render"
	"github.com/walker1211/mark2note/internal/timing"
)

type fakeAICommandRunner struct {
	name   string
	args   []string
	stdout string
	stderr string
	err    error
}

func (r *fakeAICommandRunner) Run(name string, args ...string) (string, string, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return r.stdout, r.stderr, r.err
}

type fakeRenderer struct {
	rendered deck.Deck
	result   render.RenderResult
	err      error
	called   int
}

func (r *fakeRenderer) Render(d deck.Deck) (render.RenderResult, error) {
	r.called++
	r.rendered = d
	return r.result, r.err
}

func deckJSONWithPageCount(count int) string {
	pages := make([]string, 0, count)
	pages = append(pages, fmt.Sprintf(`{"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/%d","theme":"default","cta":"cta"},"content":{"title":"封面"}}`, count))
	for i := 2; i < count; i++ {
		pages = append(pages, fmt.Sprintf(`{"name":"p%02d-bullets","variant":"bullets","meta":{"badge":"第 %d 页","counter":"%d/%d","theme":"default","cta":"cta"},"content":{"title":"列表页 %d","items":["要点"]}}`, i, i, i, count, i))
	}
	pages = append(pages, fmt.Sprintf(`{"name":"p%02d-ending","variant":"ending","meta":{"badge":"第 %d 页","counter":"%d/%d","theme":"default","cta":"cta"},"content":{"title":"结尾","body":"正文"}}`, count, count, count, count))
	return `{"pages":[` + strings.Join(pages, ",") + `]}`
}

func TestServiceGeneratePreviewPassesPromptExtraToAIBuilder(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, AI: config.AICfg{Command: "ccs", Args: []string{"codex"}}}
	runner := &fakeAICommandRunner{stdout: `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`}

	svc := Service{
		LoadConfig:      func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:        func(string) ([]byte, error) { return []byte("# 标题"), nil },
		AICommandRunner: runner,
		NewRenderer:     func(Options) DeckRenderer { return &fakeRenderer{} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, PromptExtra: "封面更抓眼"})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if len(runner.args) < 4 {
		t.Fatalf("runner args = %v, want command args plus --bare and -p prompt", runner.args)
	}
	prompt := runner.args[3]
	if !strings.Contains(prompt, "以下是本次生成的额外约束") || !strings.Contains(prompt, "不得原文复制") {
		t.Fatalf("prompt = %q, want hidden extra constraint wrapper", prompt)
	}
	if !strings.Contains(prompt, "封面更抓眼") {
		t.Fatalf("prompt = %q, want PromptExtra contents", prompt)
	}
}

func TestServiceGeneratePreviewPassesMaxPagesToAIBuilder(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, AI: config.AICfg{Command: "ccs", Args: []string{"codex"}}, Deck: config.DeckCfg{MaxPages: 18}}
	runner := &fakeAICommandRunner{stdout: `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`}
	_svcRenderer := &fakeRenderer{}
	svc := Service{
		LoadConfig:      func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:        func(string) ([]byte, error) { return []byte("# 标题"), nil },
		AICommandRunner: runner,
		NewRenderer:     func(Options) DeckRenderer { return _svcRenderer },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	prompt := runner.args[len(runner.args)-1]
	if !strings.Contains(prompt, "3-18 页") {
		t.Fatalf("prompt = %q, want configured max pages", prompt)
	}
}

func TestServiceGeneratePreviewAcceptsConfiguredMaxPages(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		AI:     config.AICfg{Command: "ccs", Args: []string{"codex"}},
		Deck:   config.DeckCfg{MaxPages: 18},
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSONWithPageCount(18), nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if len(r.rendered.Pages) != 18 {
		t.Fatalf("renderer received %d pages, want 18", len(r.rendered.Pages))
	}
}

func TestServiceGeneratePreviewSuccess(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		AI:     config.AICfg{Command: "ccs", Args: []string{"codex"}},
	}
	markdown := "# 标题"
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	fixedNow := time.Date(2026, 3, 28, 15, 30, 0, 0, time.UTC)
	wantOutDir, err := filepath.Abs(filepath.Join(cfg.Output.Dir, "article-20260328-153000"))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	var gotConfigPath string
	var gotInputPath string
	var gotMarkdown string
	var gotBuilderCfg *config.Config
	var gotNewRendererOpts Options
	r := &fakeRenderer{}

	svc := Service{
		LoadConfig: func(path string) (*config.Config, error) {
			gotConfigPath = path
			return cfg, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			gotInputPath = path
			return []byte(markdown), nil
		},
		BuildDeckJSON: func(c *config.Config, md string) (string, error) {
			gotBuilderCfg = c
			gotMarkdown = md
			return deckJSON, nil
		},
		NewRenderer: func(opts Options) DeckRenderer {
			gotNewRendererOpts = opts
			return r
		},
		Now: func() time.Time { return fixedNow },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}

	if gotConfigPath != "config.yaml" {
		t.Fatalf("LoadConfig path = %q, want %q", gotConfigPath, "config.yaml")
	}
	if gotInputPath != "article.md" {
		t.Fatalf("ReadFile path = %q, want %q", gotInputPath, "article.md")
	}
	if gotBuilderCfg != cfg {
		t.Fatalf("BuildDeckJSON cfg pointer mismatch")
	}
	if gotMarkdown != markdown {
		t.Fatalf("BuildDeckJSON markdown = %q, want %q", gotMarkdown, markdown)
	}
	if gotNewRendererOpts.OutDir == "" {
		t.Fatalf("NewRenderer OutDir is empty")
	}
	if gotNewRendererOpts.OutDir != wantOutDir {
		t.Fatalf("NewRenderer OutDir = %q, want %q", gotNewRendererOpts.OutDir, wantOutDir)
	}
	if gotNewRendererOpts.OutDir != result.OutDir {
		t.Fatalf("NewRenderer OutDir = %q, result OutDir = %q", gotNewRendererOpts.OutDir, result.OutDir)
	}
	if r.called != 1 {
		t.Fatalf("renderer called %d times, want 1", r.called)
	}
	if len(r.rendered.Pages) != 3 {
		t.Fatalf("renderer received %d pages, want 3", len(r.rendered.Pages))
	}
	gotPageNames := []string{r.rendered.Pages[0].Name, r.rendered.Pages[1].Name, r.rendered.Pages[2].Name}
	wantPageNames := []string{"p01-cover", "p02-bullets", "p03-ending"}
	if !reflect.DeepEqual(gotPageNames, wantPageNames) {
		t.Fatalf("renderer page names = %#v, want %#v", gotPageNames, wantPageNames)
	}
	if result.PageCount != 3 {
		t.Fatalf("result.PageCount = %d, want 3", result.PageCount)
	}
	if result.OutDir != wantOutDir {
		t.Fatalf("result.OutDir = %q, want %q", result.OutDir, wantOutDir)
	}
}

func TestServiceGeneratePreviewBuildsCardManifestDeckWithoutAI(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	manifest := `{
		"schema_version":"card-article-manifest/v1",
		"source_app":"news-briefing",
		"document":{"title":"今日 AI 晚报","date":"2026-06-16","period":"1800","summary":["模型发布提速","终端侧竞争升温"]},
		"items":[
			{"id":"a1","category":"AI/科技","title":"OpenAI 发布新模型","summary":"模型能力提升。","impact":"应用开发门槛下降。","source":"The Verge","published_at":"2026-06-16T11:00:00+08:00","url":"https://example.com/a1","image":{"src":"assets/openai.jpg","alt":"模型发布现场"}},
			{"id":"a2","category":"硬件","title":"AI 眼镜更新","summary":"新品强调续航。","impact":"穿戴设备竞争加剧。","source":"Bloomberg","published_at":"2026-06-16T12:30:00+08:00","url":"https://example.com/a2"}
		]
	}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored by card manifest"), nil
			}
			return os.ReadFile(path)
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatal("BuildDeckJSON called for card manifest")
			return "", nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if result.PageCount != 3 || len(r.rendered.Pages) != 3 {
		t.Fatalf("page count = result:%d rendered:%d, want 3", result.PageCount, len(r.rendered.Pages))
	}
	cover := r.rendered.Pages[0]
	if cover.Variant != "cover" || cover.Content.Title != "今日 AI 晚报" {
		t.Fatalf("cover = %#v", cover)
	}
	wantSubtitle := "• 模型发布提速\n• 终端侧竞争升温"
	if cover.Content.Subtitle != wantSubtitle {
		t.Fatalf("cover subtitle = %q, want %q", cover.Content.Subtitle, wantSubtitle)
	}
	imagePage := r.rendered.Pages[1]
	if imagePage.Variant != "image-caption" || imagePage.Content.Title != "OpenAI 发布新模型" {
		t.Fatalf("image page = %#v", imagePage)
	}
	if imagePage.Content.Body != "**摘要：** 模型能力提升。\n\n**影响：** 应用开发门槛下降。" {
		t.Fatalf("image page body = %q", imagePage.Content.Body)
	}
	if imagePage.Meta.CTA != "来源：The Verge / 2026-06-16 11:00" {
		t.Fatalf("image page cta = %q", imagePage.Meta.CTA)
	}
	if !reflect.DeepEqual(imagePage.Content.Images, []deck.ImageBlock{{Src: "assets/openai.jpg", Alt: "模型发布现场"}}) {
		t.Fatalf("image page images = %#v", imagePage.Content.Images)
	}
	textPage := r.rendered.Pages[2]
	if textPage.Variant != "text-caption" || textPage.Content.Title != "AI 眼镜更新" {
		t.Fatalf("text page = %#v", textPage)
	}
	if textPage.Meta.CTA != "来源：Bloomberg / 2026-06-16 12:30" {
		t.Fatalf("text page cta = %q", textPage.Meta.CTA)
	}
}

func TestBuildCardManifestDeckWrapsElectronicPicklesCoverTitle(t *testing.T) {
	deckJSON, err := buildCardManifestDeckJSON([]byte(`{
		"schema_version":"card-article-manifest/v1",
		"source_app":"vidtrace",
		"document":{"title":"每日电子榨菜｜2026-06-16"},
		"items":[
			{"id":"BV111","category":"B站热门","title":"第一条","sections":[{"label":"内容概览","body":"第一条内容。"}]},
			{"id":"BV222","category":"B站热门","title":"第二条","sections":[{"label":"内容概览","body":"第二条内容。"}]}
		]
	}`))
	if err != nil {
		t.Fatalf("buildCardManifestDeckJSON() error = %v", err)
	}
	var got deck.Deck
	if err := json.Unmarshal([]byte(deckJSON), &got); err != nil {
		t.Fatalf("Unmarshal(deckJSON) error = %v", err)
	}
	if got.Pages[0].Content.Title != "每日电子榨菜\n2026-06-16" {
		t.Fatalf("cover title = %q, want natural line break", got.Pages[0].Content.Title)
	}
}

func TestServiceGeneratePreviewBuildsNoImageCardManifestItemsAsTextCaption(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	manifest := `{"schema_version":"card-article-manifest/v1","source_app":"news-briefing","document":{"title":"今日速览","subtitle":"无图新闻"},"items":[{"id":"a1","category":"AI","title":"第一条","summary":"摘要一","source":"Source A"},{"id":"a2","category":"科技","title":"第二条","impact":"影响二","source":"Source B"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatal("BuildDeckJSON called for card manifest")
			return "", nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	for _, page := range r.rendered.Pages[1:] {
		if page.Variant != "text-caption" {
			t.Fatalf("page %s variant = %q, want text-caption", page.Name, page.Variant)
		}
		if len(page.Content.Images) != 0 {
			t.Fatalf("page %s images = %#v, want none", page.Name, page.Content.Images)
		}
	}
}

func TestServiceGeneratePreviewBuildsCardManifestSectionsWithoutSummaryImpactLabels(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	manifest := `{
		"schema_version":"card-article-manifest/v1",
		"source_app":"vidtrace",
		"document":{"title":"每日电子榨菜"},
		"items":[
			{
				"id":"BV111",
				"category":"B站热门",
				"title":"海底生存实况",
				"summary":"不应该出现的摘要",
				"impact":"不应该出现的影响",
				"source":"游戏人阿管",
				"sections":[
					{"label":"内容概览","body":"这是一段海底生存游戏实况解说。"},
					{"label":"互动数据","body":"点赞 12,345｜收藏 6,789｜投币 2,345"}
				]
			},
			{"id":"BV222","category":"B站热门","title":"第二条视频","sections":[{"label":"内容概览","body":"第二条内容。"}]}
		]
	}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	body := r.rendered.Pages[1].Content.Body
	want := "**内容概览：** 这是一段海底生存游戏实况解说。\n\n**互动数据：** 点赞 12,345｜收藏 6,789｜投币 2,345"
	if body != want {
		t.Fatalf("section body = %q, want %q", body, want)
	}
	for _, forbidden := range []string{"摘要：", "影响：", "不应该出现"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("section body should not contain %q: %q", forbidden, body)
		}
	}
}

func TestServiceGeneratePreviewFitsCardManifestSections(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	longOverview := strings.Repeat("这是一段很长的视频内容概览，用来说明视频主题、叙事结构和主要看点。", 10)
	longKeyPoints := strings.Repeat("这是一段很长的关键看点，用来覆盖反转、冲突、制作细节和观看理由。", 8)
	manifest := fmt.Sprintf(`{
		"schema_version":"card-article-manifest/v1",
		"source_app":"vidtrace",
		"document":{"title":"每日电子榨菜"},
		"items":[
			{"id":"BV111","category":"B站热门","title":"配图视频","source":"游戏人阿管","image":{"src":"https://example.com/cover.jpg","alt":"视频封面"},"sections":[{"label":"内容概览","body":%q},{"label":"关键看点","body":%q},{"label":"互动数据","body":"点赞 12,345｜收藏 6,789｜投币 2,345"}]},
			{"id":"BV222","category":"B站热门","title":"无图视频","source":"游戏人阿管","sections":[{"label":"内容概览","body":%q},{"label":"关键看点","body":%q},{"label":"互动数据","body":"点赞 22,345｜收藏 7,789｜投币 3,345"}]}
		]
	}`, longOverview, longKeyPoints, longOverview, longKeyPoints)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	imageBody := r.rendered.Pages[1].Content.Body
	if utf8.RuneCountInString(imageBody) > cardManifestImageCaptionBodyMaxRunes {
		t.Fatalf("image-caption section body runes = %d, want <= %d", utf8.RuneCountInString(imageBody), cardManifestImageCaptionBodyMaxRunes)
	}
	for _, want := range []string{"内容概览：", "关键看点：", "互动数据：", "…"} {
		if !strings.Contains(imageBody, want) {
			t.Fatalf("image-caption section body should contain %q: %q", want, imageBody)
		}
	}
	textBody := r.rendered.Pages[2].Content.Body
	if utf8.RuneCountInString(textBody) > cardManifestTextCaptionBodyMaxRunes {
		t.Fatalf("text-caption section body runes = %d, want <= %d", utf8.RuneCountInString(textBody), cardManifestTextCaptionBodyMaxRunes)
	}
	for _, want := range []string{"内容概览：", "关键看点：", "互动数据：", "…"} {
		if !strings.Contains(textBody, want) {
			t.Fatalf("text-caption section body should contain %q: %q", want, textBody)
		}
	}
}

func TestServiceGeneratePreviewReturnsErrorForUnknownCardManifestSectionField(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	manifest := `{"schema_version":"card-article-manifest/v1","document":{"title":"每日电子榨菜"},"items":[{"id":"BV111","title":"海底生存实况","sections":[{"label":"内容概览","body":"正文","unknown":"bad"}]}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		NewRenderer: func(Options) DeckRenderer { return &fakeRenderer{} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse card manifest json") || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
}

func TestServiceGeneratePreviewFitsCardManifestTextForNewsCards(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	longSummary := strings.Repeat("这是一段很长的摘要，用来覆盖主要事实、背景、争议和当前公开信息。", 10)
	longImpact := strings.Repeat("这是一段很长的影响分析，用来说明企业客户、开发者生态和后续监管变化。", 8)
	manifest := fmt.Sprintf(`{
		"schema_version":"card-article-manifest/v1",
		"source_app":"news-briefing",
		"document":{"title":"今日 AI 晚报","summary":["第一条速览内容会保留","第二条速览内容会保留","第三条速览内容会保留","第四条速览内容会保留","第五条速览内容会保留","第六条速览内容会保留","第七条速览内容不应进入封面"]},
		"items":[
			{"id":"a1","category":"AI/科技","title":"配图新闻","summary":%q,"impact":%q,"source":"The Verge","image":{"src":"assets/news.jpg","alt":"新闻图"}},
			{"id":"a2","category":"AI/科技","title":"无图新闻","summary":%q,"impact":%q,"source":"Hacker News"}
		]
	}`, longSummary, longImpact, longSummary, longImpact)
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatal("BuildDeckJSON called for card manifest")
			return "", nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	coverLines := strings.Split(r.rendered.Pages[0].Content.Subtitle, "\n")
	if len(coverLines) != 6 {
		t.Fatalf("cover subtitle lines = %d, want 6: %q", len(coverLines), r.rendered.Pages[0].Content.Subtitle)
	}
	if strings.Contains(r.rendered.Pages[0].Content.Subtitle, "第七条") {
		t.Fatalf("cover subtitle should omit seventh item: %q", r.rendered.Pages[0].Content.Subtitle)
	}
	imageBody := r.rendered.Pages[1].Content.Body
	if utf8.RuneCountInString(imageBody) > cardManifestImageCaptionBodyMaxRunes {
		t.Fatalf("image-caption body runes = %d, want <= %d", utf8.RuneCountInString(imageBody), cardManifestImageCaptionBodyMaxRunes)
	}
	if !strings.Contains(imageBody, "摘要：") || !strings.Contains(imageBody, "影响：") || !strings.Contains(imageBody, "…") {
		t.Fatalf("image-caption body should preserve labels and ellipsis: %q", imageBody)
	}
	textBody := r.rendered.Pages[2].Content.Body
	if utf8.RuneCountInString(textBody) > cardManifestTextCaptionBodyMaxRunes {
		t.Fatalf("text-caption body runes = %d, want <= %d", utf8.RuneCountInString(textBody), cardManifestTextCaptionBodyMaxRunes)
	}
	if !strings.Contains(textBody, "摘要：") || !strings.Contains(textBody, "影响：") || !strings.Contains(textBody, "…") {
		t.Fatalf("text-caption body should preserve labels and ellipsis: %q", textBody)
	}
}

func TestServiceGeneratePreviewInlinesCardManifestImagesRelativeToManifest(t *testing.T) {
	root := t.TempDir()
	manifestDir := filepath.Join(root, "manifest")
	if err := os.MkdirAll(filepath.Join(manifestDir, "assets"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "assets", "news.png"), []byte("\x89PNG\r\n\x1a\nimage"), 0o644); err != nil {
		t.Fatalf("WriteFile(image) error = %v", err)
	}
	manifestPath := filepath.Join(manifestDir, "manifest.json")
	manifest := `{"schema_version":"card-article-manifest/v1","document":{"title":"今日速览"},"items":[{"id":"a1","title":"配图新闻","summary":"摘要一","source":"Source A","image":{"src":"assets/news.png","alt":"新闻图"}},{"id":"a2","title":"第二条","summary":"摘要二","source":"Source B"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == filepath.Join(root, "article.md") {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: filepath.Join(root, "article.md"), ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	got := r.rendered.Pages[1].Content.Images[0].Src
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("card manifest image src = %q, want data URI from manifest-relative path", got)
	}
}

func TestServiceGeneratePreviewReturnsErrorForInvalidCardManifest(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(`{"schema_version":"unsupported","document":{"title":"标题"},"items":[]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatal("BuildDeckJSON called for invalid card manifest")
			return "", nil
		},
		NewRenderer: func(Options) DeckRenderer { return &fakeRenderer{} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "build deck json failed") || !strings.Contains(err.Error(), "unsupported card manifest schema_version") {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
}

func TestServiceGeneratePreviewEmitsCardManifestTimingStage(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "manifest.json")
	manifest := `{"schema_version":"card-article-manifest/v1","document":{"title":"今日速览"},"items":[{"id":"a1","title":"第一条","summary":"摘要一","source":"Source A"},{"id":"a2","title":"第二条","summary":"摘要二","source":"Source B"}]}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	var timingOutput strings.Builder
	oldTimingOutput := timing.SetOutput(&timingOutput)
	defer timing.SetOutput(oldTimingOutput)
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: root}, Deck: config.DeckCfg{MaxPages: 12}}, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "article.md" {
				return []byte("# ignored"), nil
			}
			return os.ReadFile(path)
		},
		NewRenderer: func(Options) DeckRenderer { return &fakeRenderer{} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, CardManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	output := timingOutput.String()
	if !strings.Contains(output, "stage=app.GeneratePreview.build_card_manifest_deck") {
		t.Fatalf("timing output = %q, want build_card_manifest_deck stage", output)
	}
	if strings.Contains(output, "stage=app.GeneratePreview.ai_build_deck_json") {
		t.Fatalf("timing output = %q, want no ai_build_deck_json stage", output)
	}
}

func TestServiceGeneratePreviewBuildsElectronicPicklesDeckWithoutAI(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{MaxPages: 12}}
	titles := []string{
		"歌剧老师锐评《乘风破浪的姐姐2026》四公（上）",
		"2026新一卷数学！难出天际？？？！",
		"摔麦喷媒体，空降看球赛，特朗普还有多少活要整？",
		"【某幻】《盛世天下》再返深宫，活下来！",
		"现实里得体，拍照又上镜的妆容。(含发型)",
		"《别替我吹了》",
		"12 个脑洞大开的美食制作方式，有手就会！",
		"新三国up锐评老三国01：这才叫真正的三国！",
		"和日本女友去她丈母娘家里",
		"有人职场内耗你 有人教你放宽心",
	}
	var markdown strings.Builder
	markdown.WriteString("# 每日电子榨菜｜2026-06-10\n\n## 小红书卡片\n")
	for i, title := range titles {
		markdown.WriteString(fmt.Sprintf("\n### 第 %d 页｜%s（点赞 1,000｜收藏 200｜投币 30）\n\n", i+1, title))
		markdown.WriteString(fmt.Sprintf("![视频封面](https://example.com/cover-%02d.jpg)\n\n", i+1))
		markdown.WriteString("* **内容概览**：这是一条视频内容概览。\n")
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte(markdown.String()), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatal("BuildDeckJSON called for electronic pickles markdown")
			return "", nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if result.PageCount != 11 || len(r.rendered.Pages) != 11 {
		t.Fatalf("page count = result:%d rendered:%d, want 11", result.PageCount, len(r.rendered.Pages))
	}
	cover := r.rendered.Pages[0]
	if cover.Variant != "cover" || cover.Content.Title != "每日电子榨菜｜2026-06-10" {
		t.Fatalf("cover = %#v", cover)
	}
	wantSubtitle := strings.Join([]string{"今日彩蛋", "1. " + titles[0], "2. " + titles[1], "3. " + titles[2], "4. " + titles[3], "5. " + titles[4], "6. " + titles[5]}, "\n")
	if cover.Content.Subtitle != wantSubtitle {
		t.Fatalf("cover subtitle = %q, want %q", cover.Content.Subtitle, wantSubtitle)
	}
	if cover.Meta.CTA != "B站热门" {
		t.Fatalf("cover cta = %q", cover.Meta.CTA)
	}
	firstVideo := r.rendered.Pages[1]
	if firstVideo.Variant != "image-caption" || firstVideo.Content.Title != titles[0] {
		t.Fatalf("first video page = %#v", firstVideo)
	}
	if !strings.Contains(firstVideo.Meta.Badge, "点赞 1,000｜收藏 200｜投币 30") {
		t.Fatalf("first video badge = %q, want interaction stats", firstVideo.Meta.Badge)
	}
	if firstVideo.Meta.CTA != "B站热门" {
		t.Fatalf("first video cta = %q", firstVideo.Meta.CTA)
	}
	if len(firstVideo.Content.Images) != 1 || firstVideo.Content.Images[0].Src != "https://example.com/cover-01.jpg" {
		t.Fatalf("first video images = %#v", firstVideo.Content.Images)
	}
	if firstVideo.Content.Body != "这是一条视频内容概览。" {
		t.Fatalf("first video body = %q", firstVideo.Content.Body)
	}
}

func TestServiceGeneratePreviewMovesLeadingMarkdownImageToCover(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	coverImage := "https://example.com/cover.png"
	markdown := "# 标题\n\n![封面](" + coverImage + ")\n\n## 背景\n\n正文"
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-image-caption","variant":"image-caption","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"背景","body":"正文","images":[{"src":"https://example.com/cover.png","alt":"封面"}]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte(markdown), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if got := r.rendered.Pages[0].Content.Images; !reflect.DeepEqual(got, []deck.ImageBlock{{Src: coverImage, Alt: "封面"}}) {
		t.Fatalf("cover images = %#v", got)
	}
	if got := r.rendered.Pages[1].Content.Images; len(got) != 0 {
		t.Fatalf("second page images = %#v, want image moved out", got)
	}
	if r.rendered.Pages[1].Content.Body != "正文" {
		t.Fatalf("second page body = %q", r.rendered.Pages[1].Content.Body)
	}
}

func TestServiceGeneratePreviewKeepsLeadingMarkdownImageWhenAltIsNotCover(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	image := "https://example.com/news.png"
	markdown := "# 标题\n\n![新闻图](" + image + ")\n\n## 背景\n\n正文"
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-image-caption","variant":"image-caption","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"背景","body":"正文","images":[{"src":"https://example.com/news.png","alt":"新闻图"}]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte(markdown), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if got := r.rendered.Pages[0].Content.Images; len(got) != 0 {
		t.Fatalf("cover images = %#v, want ordinary leading image kept out of cover", got)
	}
	if got := r.rendered.Pages[1].Content.Images; !reflect.DeepEqual(got, []deck.ImageBlock{{Src: image, Alt: "新闻图"}}) {
		t.Fatalf("second page images = %#v, want ordinary leading image kept", got)
	}
}

func TestServiceGeneratePreviewInlinesLocalImageAssetsBeforeRendering(t *testing.T) {
	root := t.TempDir()
	articleDir := filepath.Join(root, "article")
	imagePath := filepath.Join(articleDir, "media", "inline-local-001.png")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(imagePath, []byte("\x89PNG\r\n\x1a\nimage"), 0o644); err != nil {
		t.Fatalf("WriteFile(image) error = %v", err)
	}
	articlePath := filepath.Join(articleDir, "article.md")
	if err := os.WriteFile(articlePath, []byte("# 标题\n\n![封面](media/inline-local-001.png)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(article) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: root}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-image-caption","variant":"image-caption","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"图文","images":[{"src":"media/inline-local-001.png","alt":"封面"}] }},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      os.ReadFile,
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: articlePath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	got := r.rendered.Pages[0].Content.Images[0].Src
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("rendered cover image src = %q, want local image data URI", got)
	}
	if len(r.rendered.Pages[1].Content.Images) != 0 {
		t.Fatalf("second page images = %#v, want leading image moved to cover", r.rendered.Pages[1].Content.Images)
	}
}

func TestServiceGeneratePreviewHydratesAssetManifestBeforeRendering(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "posters.yaml")
	if err := os.WriteFile(manifestPath, []byte("posters:\n  噬谎者:\n    src: https://example.com/usogui.jpg\n  死亡笔记:\n    src: https://example.com/death-note.jpg\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: root}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"门面","compare":{"leftLabel":"《噬谎者》","rightLabel":"《死亡笔记》","rows":[{"left":"《噬谎者》：规则都算进局里。","right":"《死亡笔记》：夜神月和 L 的对抗。"}]} }},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, AssetManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.Pages[1].Variant != "gallery-steps" {
		t.Fatalf("hydrated variant = %q, want gallery-steps", r.rendered.Pages[1].Variant)
	}
	if len(r.rendered.Pages[1].Content.Images) != 2 {
		t.Fatalf("hydrated images = %#v", r.rendered.Pages[1].Content.Images)
	}
}

func TestServiceGeneratePreviewAutoPostersHydratesBeforeRendering(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Output: config.OutputCfg{Dir: root}}
	markdown := "# 标题\n\n推荐《噬谎者》和《死亡笔记》。"
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"门面","compare":{"leftLabel":"《噬谎者》","rightLabel":"《死亡笔记》","rows":[{"left":"《噬谎者》：规则都算进局里。","right":"《死亡笔记》：夜神月和 L 的对抗。"}]} }},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	var gotMarkdown string
	var gotSources []string
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte(markdown), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		EnrichPosters: func(_ context.Context, md string, sources []string) (poster.Manifest, poster.EnrichReport, error) {
			gotMarkdown = md
			gotSources = append([]string(nil), sources...)
			return poster.Manifest{Posters: map[string]poster.PosterAsset{
				"噬谎者":  {Src: "https://example.com/usogui.jpg"},
				"死亡笔记": {Src: "https://example.com/death-note.jpg"},
			}}, poster.EnrichReport{Titles: []string{"噬谎者", "死亡笔记"}}, nil
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, AutoPosters: true, PosterSources: []string{"anilist"}})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if gotMarkdown != markdown || !reflect.DeepEqual(gotSources, []string{"anilist"}) {
		t.Fatalf("enrich inputs = %q / %#v", gotMarkdown, gotSources)
	}
	if r.rendered.Pages[1].Variant != "gallery-steps" || len(r.rendered.Pages[1].Content.Images) != 2 {
		t.Fatalf("hydrated page = %#v", r.rendered.Pages[1])
	}
}

func TestServiceGeneratePreviewAutoPostersDoesNotAddGlobalDeadline(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{Output: config.OutputCfg{Dir: root}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题\n\n推荐《冰果》。"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		EnrichPosters: func(ctx context.Context, _ string, _ []string) (poster.Manifest, poster.EnrichReport, error) {
			if deadline, ok := ctx.Deadline(); ok {
				t.Fatalf("enrich context deadline = %v, want no global deadline", deadline)
			}
			return poster.Manifest{Posters: map[string]poster.PosterAsset{}}, poster.EnrichReport{Titles: []string{"冰果"}}, nil
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, AutoPosters: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
}

func TestServiceGeneratePreviewAssetManifestOverridesAutoPosters(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "posters.yaml")
	if err := os.WriteFile(manifestPath, []byte("posters:\n  噬谎者:\n    src: https://example.com/manual-usogui.jpg\n  死亡笔记:\n    src: https://example.com/death-note.jpg\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: root}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"门面","compare":{"leftLabel":"《噬谎者》","rightLabel":"《死亡笔记》","rows":[{"left":"《噬谎者》：规则都算进局里。","right":"《死亡笔记》：夜神月和 L 的对抗。"}]} }},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		EnrichPosters: func(_ context.Context, _ string, _ []string) (poster.Manifest, poster.EnrichReport, error) {
			return poster.Manifest{Posters: map[string]poster.PosterAsset{
				"噬谎者": {Src: "https://example.com/auto-usogui.jpg"},
			}}, poster.EnrichReport{Titles: []string{"噬谎者"}}, nil
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, AutoPosters: true, AssetManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if got := r.rendered.Pages[1].Content.Images[0].Src; got != "https://example.com/manual-usogui.jpg" {
		t.Fatalf("first poster src = %q, want manual override", got)
	}
}

func TestServiceGeneratePreviewConfigViewportOverridesDeckJSONViewport(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Viewport: config.ViewportCfg{Width: 1242, Height: 1656}},
	}
	deckJSON := `{"viewport":{"width":720,"height":960},"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ViewportWidth != 1242 || r.rendered.ViewportHeight != 1656 {
		t.Fatalf("viewport = %dx%d, want config viewport 1242x1656", r.rendered.ViewportWidth, r.rendered.ViewportHeight)
	}
}

func TestServiceGenerateFromDeckReadsDeckAndSkipsMarkdownBuilder(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(720, 960)
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	fixedNow := time.Date(2026, 4, 24, 15, 12, 0, 0, time.UTC)
	wantOutDir, err := filepath.Abs(filepath.Join(cfg.Output.Dir, filepath.Base(deckDir)+"-20260424-151200"))
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	readPaths := []string{}
	r := &fakeRenderer{}
	var gotOpts Options
	svc := Service{
		LoadConfig: func(path string) (*config.Config, error) {
			if path != "config.yaml" {
				t.Fatalf("LoadConfig path = %q, want config.yaml", path)
			}
			return cfg, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			readPaths = append(readPaths, path)
			if path == deckPath {
				return []byte(deckJSON), nil
			}
			return nil, os.ErrNotExist
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatalf("BuildDeckJSON should not be called in from-deck mode")
			return "", nil
		},
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return r
		},
		Now: func() time.Time { return fixedNow },
	}

	result, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	metaPath := filepath.Join(deckDir, "render-meta.json")
	if !reflect.DeepEqual(readPaths, []string{deckPath, metaPath}) {
		t.Fatalf("ReadFile paths = %#v, want deck then optional meta path", readPaths)
	}
	if gotOpts.OutDir != wantOutDir || result.OutDir != wantOutDir {
		t.Fatalf("out dirs = opts:%q result:%q want %q", gotOpts.OutDir, result.OutDir, wantOutDir)
	}
	if r.called != 1 {
		t.Fatalf("renderer called %d times, want 1", r.called)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("rendered ThemeName = %q", r.rendered.ThemeName)
	}
	if r.rendered.ViewportWidth != 720 || r.rendered.ViewportHeight != 960 {
		t.Fatalf("rendered viewport = %dx%d, want 720x960", r.rendered.ViewportWidth, r.rendered.ViewportHeight)
	}
	if result.PageCount != 3 {
		t.Fatalf("PageCount = %d, want 3", result.PageCount)
	}
}

func TestServiceGenerateFromDeckAcceptsConfiguredMaxPages(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := deckJSONWithPageCount(18)
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{MaxPages: 18}}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			if path == deckPath {
				return []byte(deckJSON), nil
			}
			return nil, os.ErrNotExist
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			t.Fatalf("BuildDeckJSON should not be called in from-deck mode")
			return "", nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if len(r.rendered.Pages) != 18 {
		t.Fatalf("renderer received %d pages, want 18", len(r.rendered.Pages))
	}
}

func TestServiceGenerateFromDeckUsesRenderMetaBeforeConfigDefaults(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(1242, 1656)
	metaJSON := legacyRenderMetaWithProvenanceJSON()
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deckDir, "render-meta.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(render-meta.json) error = %v", err)
	}
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{
			Theme:  deck.ThemeDefault,
			Author: "配置作者",
			Watermark: config.WatermarkCfg{
				Text:     "配置水印",
				Position: "bottom-right",
			},
		},
		Render: config.RenderCfg{Viewport: config.ViewportCfg{Width: 1242, Height: 1656}},
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case deckPath:
				return []byte(deckJSON), nil
			case filepath.Join(deckDir, "render-meta.json"):
				return []byte(metaJSON), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("ThemeName = %q, want default", r.rendered.ThemeName)
	}
	if r.rendered.ViewportWidth != 720 || r.rendered.ViewportHeight != 960 {
		t.Fatalf("viewport = %dx%d, want 720x960", r.rendered.ViewportWidth, r.rendered.ViewportHeight)
	}
	if !r.rendered.ShowAuthor || r.rendered.AuthorText != "@旧作者" {
		t.Fatalf("author = show:%v text:%q", r.rendered.ShowAuthor, r.rendered.AuthorText)
	}
	if !r.rendered.ShowWatermark || r.rendered.WatermarkText != "旧水印" || r.rendered.WatermarkPosition != "bottom-left" {
		t.Fatalf("watermark = show:%v text:%q position:%q", r.rendered.ShowWatermark, r.rendered.WatermarkText, r.rendered.WatermarkPosition)
	}
}

func TestServiceGenerateFromDeckPreservesDisabledAuthorFromRenderMeta(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(720, 960)
	metaJSON := legacyRenderMetaDisabledAuthorJSON()
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deckDir, "render-meta.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(render-meta.json) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{Author: "配置作者"}}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case deckPath:
				return []byte(deckJSON), nil
			case filepath.Join(deckDir, "render-meta.json"):
				return []byte(metaJSON), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if r.rendered.ShowAuthor || r.rendered.AuthorText != "" {
		t.Fatalf("author = show:%v text:%q, want disabled", r.rendered.ShowAuthor, r.rendered.AuthorText)
	}
}

func TestServiceGenerateFromDeckCLIThemeOverridesRenderMeta(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(720, 960)
	metaJSON := legacyRenderMetaThemeOnlyJSON()
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deckDir, "render-meta.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(render-meta.json) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case deckPath:
				return []byte(deckJSON), nil
			case filepath.Join(deckDir, "render-meta.json"):
				return []byte(metaJSON), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Theme: deck.ThemeTechNoir, Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeTechNoir {
		t.Fatalf("ThemeName = %q, want tech-noir", r.rendered.ThemeName)
	}
}

func TestServiceGenerateFromDeckIgnoresLegacyRenderMetaPageThemeKeys(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(720, 960)
	metaJSON := legacyRenderMetaShortPageThemeKeysJSON()
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deckDir, "render-meta.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(render-meta.json) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case deckPath:
				return []byte(deckJSON), nil
			case filepath.Join(deckDir, "render-meta.json"):
				return []byte(metaJSON), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if r.called != 1 {
		t.Fatalf("renderer called %d times, want 1", r.called)
	}
}

func TestServiceGenerateFromDeckHonorsExplicitOutDir(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	outDir := t.TempDir()
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	var gotOpts Options
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			if path == deckPath {
				return []byte(deckJSON), nil
			}
			return nil, os.ErrNotExist
		},
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	result, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", OutDir: outDir, OutDirChanged: true, Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if gotOpts.OutDir != outDir || result.OutDir != outDir {
		t.Fatalf("out dirs = opts:%q result:%q want %q", gotOpts.OutDir, result.OutDir, outDir)
	}
}

func TestServiceGenerateFromDeckPassesImportAndLiveOptionsToRenderer(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{
			ImportPhotos:  true,
			ImportAlbum:   "PNG 相册",
			ImportTimeout: 45 * time.Second,
			Live: config.LiveCfg{
				Enabled:       true,
				PhotoFormat:   "jpeg",
				CoverFrame:    "first",
				Assemble:      true,
				OutputDir:     "apple-live",
				ImportPhotos:  true,
				ImportAlbum:   "Live 相册",
				ImportTimeout: 75 * time.Second,
			},
		},
	}
	var gotOpts Options
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			if path == deckPath {
				return []byte(deckJSON), nil
			}
			return nil, os.ErrNotExist
		},
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if !gotOpts.ImportPhotos || gotOpts.ImportAlbum != "PNG 相册" || gotOpts.ImportTimeout != 45*time.Second {
		t.Fatalf("png import options = photos:%v album:%q timeout:%v", gotOpts.ImportPhotos, gotOpts.ImportAlbum, gotOpts.ImportTimeout)
	}
	wantLive := LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: filepath.Join(gotOpts.OutDir, "apple-live"), ImportPhotos: true, ImportAlbum: "Live 相册", ImportTimeout: 75 * time.Second}
	if gotOpts.Live != wantLive {
		t.Fatalf("Live = %#v, want %#v", gotOpts.Live, wantLive)
	}
}

func TestServiceGenerateFromDeckWritesFreshLayoutArtifacts(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(720, 960)
	metaJSON := legacyRenderMetaWithProvenanceJSON()
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deckDir, "render-meta.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(render-meta.json) error = %v", err)
	}
	outDir := t.TempDir()
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case deckPath:
				return []byte(deckJSON), nil
			case filepath.Join(deckDir, "render-meta.json"):
				return []byte(metaJSON), nil
			default:
				return nil, os.ErrNotExist
			}
		},
		NewRenderer: func(Options) DeckRenderer { return &fakeRenderer{} },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", OutDir: outDir, OutDirChanged: true, Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	savedDeckBytes, err := os.ReadFile(filepath.Join(outDir, "deck.json"))
	if err != nil {
		t.Fatalf("ReadFile(output deck.json) error = %v", err)
	}
	reloadedDeck, err := deck.FromJSON(string(savedDeckBytes), outDir)
	if err != nil {
		t.Fatalf("deck.FromJSON(output deck.json) error = %v", err)
	}
	if reloadedDeck.ThemeName != deck.ThemeDefault || reloadedDeck.ViewportWidth != 720 || reloadedDeck.ViewportHeight != 960 {
		t.Fatalf("reloaded deck = %#v", reloadedDeck)
	}
	metaBytes, err := os.ReadFile(filepath.Join(outDir, "render-meta.json"))
	if err != nil {
		t.Fatalf("ReadFile(output render-meta.json) error = %v", err)
	}
	var savedMeta renderMeta
	if err := json.Unmarshal(metaBytes, &savedMeta); err != nil {
		t.Fatalf("json.Unmarshal(output render-meta.json) error = %v", err)
	}
	if savedMeta.InputPath != "article.md" || savedMeta.ConfigPath != "old-config.yaml" {
		t.Fatalf("saved meta provenance = input:%q config:%q", savedMeta.InputPath, savedMeta.ConfigPath)
	}
	if savedMeta.AuthorText != "@旧作者" || savedMeta.WatermarkText != "旧水印" || savedMeta.WatermarkPosition != "bottom-left" {
		t.Fatalf("saved meta author/watermark = %#v", savedMeta)
	}
}

func TestServiceGenerateFromDeckAllowsMissingRenderMeta(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := legacyShuffleDeckJSON(720, 960)
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck.json) error = %v", err)
	}
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{Author: "配置作者"}}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile: func(path string) ([]byte, error) {
			if path == deckPath {
				return []byte(deckJSON), nil
			}
			return nil, os.ErrNotExist
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if r.rendered.ViewportWidth != 720 || r.rendered.ViewportHeight != 960 {
		t.Fatalf("viewport = %dx%d, want deck viewport 720x960", r.rendered.ViewportWidth, r.rendered.ViewportHeight)
	}
	if !r.rendered.ShowAuthor || r.rendered.AuthorText != "@配置作者" {
		t.Fatalf("author = show:%v text:%q", r.rendered.ShowAuthor, r.rendered.AuthorText)
	}
}

func TestServiceGeneratePreviewWritesLayoutArtifacts(t *testing.T) {
	outDir := t.TempDir()
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{
			Theme:  deck.ThemeDefault,
			Author: "全局作者",
			Watermark: config.WatermarkCfg{
				Text:     "水印",
				Position: "bottom-left",
			},
		},
		Render: config.RenderCfg{Viewport: config.ViewportCfg{Width: 720, Height: 960}},
	}
	deckJSON := `{"pages":[{"name":"ignored-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"ignored-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"ignored-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return &fakeRenderer{} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", OutDir: outDir, OutDirChanged: true, Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	deckBytes, err := os.ReadFile(filepath.Join(outDir, "deck.json"))
	if err != nil {
		t.Fatalf("ReadFile(deck.json) error = %v", err)
	}
	var savedDeck struct {
		Theme    string `json:"theme"`
		Viewport struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
		Pages []struct {
			Name    string `json:"name"`
			Variant string `json:"variant"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(deckBytes, &savedDeck); err != nil {
		t.Fatalf("json.Unmarshal(deck.json) error = %v", err)
	}
	if savedDeck.Theme != deck.ThemeDefault {
		t.Fatalf("saved deck theme = %q, want default", savedDeck.Theme)
	}
	if savedDeck.Viewport.Width != 720 || savedDeck.Viewport.Height != 960 {
		t.Fatalf("saved deck viewport = %#v", savedDeck.Viewport)
	}
	assertJSONMissingKey(t, deckBytes, "page_theme_keys")
	gotPageNames := []string{savedDeck.Pages[0].Name, savedDeck.Pages[1].Name, savedDeck.Pages[2].Name}
	wantPageNames := []string{"p01-cover", "p02-bullets", "p03-ending"}
	if !reflect.DeepEqual(gotPageNames, wantPageNames) {
		t.Fatalf("saved deck page names = %#v, want %#v", gotPageNames, wantPageNames)
	}
	reloadedDeck, err := deck.FromJSON(string(deckBytes), outDir)
	if err != nil {
		t.Fatalf("deck.FromJSON(deck.json) error = %v", err)
	}
	if reloadedDeck.ThemeName != deck.ThemeDefault || reloadedDeck.ViewportWidth != 720 || reloadedDeck.ViewportHeight != 960 {
		t.Fatalf("reloaded deck theme/viewport = %#v", reloadedDeck)
	}

	metaBytes, err := os.ReadFile(filepath.Join(outDir, "render-meta.json"))
	if err != nil {
		t.Fatalf("ReadFile(render-meta.json) error = %v", err)
	}
	var meta struct {
		SchemaVersion int    `json:"schema_version"`
		InputPath     string `json:"input_path"`
		ConfigPath    string `json:"config_path"`
		Theme         string `json:"theme"`
		Viewport      struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"viewport"`
		AuthorText        string `json:"author_text"`
		ShowWatermark     bool   `json:"show_watermark"`
		WatermarkText     string `json:"watermark_text"`
		WatermarkPosition string `json:"watermark_position"`
	}
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		t.Fatalf("json.Unmarshal(render-meta.json) error = %v", err)
	}
	if meta.SchemaVersion != 1 || meta.InputPath != "article.md" || meta.ConfigPath != "config.yaml" {
		t.Fatalf("render meta identity = %#v", meta)
	}
	if meta.Theme != deck.ThemeDefault || meta.Viewport.Width != 720 || meta.Viewport.Height != 960 {
		t.Fatalf("render meta theme/viewport = %#v", meta)
	}
	if meta.AuthorText != "@全局作者" || !meta.ShowWatermark || meta.WatermarkText != "水印" || meta.WatermarkPosition != "bottom-left" {
		t.Fatalf("render meta author/watermark = %#v", meta)
	}
	assertJSONMissingKey(t, metaBytes, "page_theme_keys")
}

func TestServiceGeneratePreviewRemovesStaleLayoutArtifactsWhenRendererFails(t *testing.T) {
	outDir := t.TempDir()
	writeTestArtifact(t, filepath.Join(outDir, "deck.json"), "stale deck")
	writeTestArtifact(t, filepath.Join(outDir, "render-meta.json"), "stale meta")
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	renderErr := errors.New("chrome failed")
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return &fakeRenderer{err: renderErr} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", OutDir: outDir, OutDirChanged: true, Jobs: 2})
	if !errors.Is(err, ErrRenderPreview) {
		t.Fatalf("GeneratePreview() error = %v, want ErrRenderPreview", err)
	}
	assertLayoutArtifactsAbsent(t, outDir)
}

func TestWriteLayoutArtifactsRemovesStaleMetaWhenDeckWriteFails(t *testing.T) {
	outDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(outDir, "deck.json"), 0o755); err != nil {
		t.Fatalf("Mkdir(deck.json) error = %v", err)
	}
	writeTestArtifact(t, filepath.Join(outDir, "render-meta.json"), "stale meta")
	d := deck.Deck{ThemeName: deck.ThemeDefault}

	err := writeLayoutArtifacts(outDir, "article.md", "config.yaml", d)
	if err == nil {
		t.Fatalf("writeLayoutArtifacts() error = nil, want non-nil")
	}
	assertLayoutArtifactsAbsent(t, outDir)
}

func TestWriteLayoutArtifactsDoesNotLeavePartialDeckWhenMetaWriteFails(t *testing.T) {
	outDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(outDir, "render-meta.json"), 0o755); err != nil {
		t.Fatalf("Mkdir(render-meta.json) error = %v", err)
	}
	d := deck.Deck{
		ThemeName:      deck.ThemeDefault,
		ViewportWidth:  720,
		ViewportHeight: 960,
		Pages: []deck.Page{
			{Name: "p01-cover", Variant: "cover", Meta: deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "cta1"}, Content: deck.PageContent{Title: "封面"}},
			{Name: "p02-bullets", Variant: "bullets", Meta: deck.PageMeta{Badge: "第 2 页", Counter: "2/3", Theme: "orange", CTA: "cta2"}, Content: deck.PageContent{Title: "中间", Items: []string{"要点"}}},
			{Name: "p03-ending", Variant: "ending", Meta: deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta3"}, Content: deck.PageContent{Title: "结尾", Body: "正文"}},
		},
	}

	err := writeLayoutArtifacts(outDir, "article.md", "config.yaml", d)
	if err == nil {
		t.Fatalf("writeLayoutArtifacts() error = nil, want non-nil")
	}
	_, statErr := os.Stat(filepath.Join(outDir, "deck.json"))
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("Stat(deck.json) error = %v, want not exist", statErr)
	}
}

func TestServiceGeneratePreviewDoesNotWriteLayoutArtifactsWhenRendererFails(t *testing.T) {
	outDir := t.TempDir()
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	renderErr := errors.New("chrome failed")
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return &fakeRenderer{err: renderErr} },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", OutDir: outDir, OutDirChanged: true, Jobs: 2})
	if !errors.Is(err, ErrRenderPreview) {
		t.Fatalf("GeneratePreview() error = %v, want ErrRenderPreview", err)
	}
	for _, name := range []string{"deck.json", "render-meta.json"} {
		_, statErr := os.Stat(filepath.Join(outDir, name))
		if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Stat(%s) error = %v, want not exist", name, statErr)
		}
	}
}

func TestWriteLayoutArtifactsRemovesStaleMetaWhenMetaWriteFails(t *testing.T) {
	outDir := t.TempDir()
	writeTestArtifact(t, filepath.Join(outDir, "render-meta.json"), "stale meta")
	if err := os.Chmod(filepath.Join(outDir, "render-meta.json"), 0o400); err != nil {
		t.Fatalf("Chmod(render-meta.json) error = %v", err)
	}
	d := deck.Deck{
		ThemeName:      deck.ThemeDefault,
		ViewportWidth:  720,
		ViewportHeight: 960,
		Pages: []deck.Page{
			{Name: "p01-cover", Variant: "cover", Meta: deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "cta1"}, Content: deck.PageContent{Title: "封面"}},
			{Name: "p02-bullets", Variant: "bullets", Meta: deck.PageMeta{Badge: "第 2 页", Counter: "2/3", Theme: "orange", CTA: "cta2"}, Content: deck.PageContent{Title: "中间", Items: []string{"要点"}}},
			{Name: "p03-ending", Variant: "ending", Meta: deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta3"}, Content: deck.PageContent{Title: "结尾", Body: "正文"}},
		},
	}

	err := writeLayoutArtifacts(outDir, "article.md", "config.yaml", d)
	if err == nil {
		t.Fatalf("writeLayoutArtifacts() error = nil, want non-nil")
	}
	assertLayoutArtifactsAbsent(t, outDir)
}

func legacyShuffleDeckJSON(width int, height int) string {
	return fmt.Sprintf(`{"theme":"shuffle-light","viewport":{"width":%d,"height":%d},"page_theme_keys":["default-green","lifestyle-light","default-orange"],"pages":[{"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`, width, height)
}

func legacyRenderMetaWithProvenanceJSON() string {
	return `{"schema_version":1,"input_path":"article.md","config_path":"old-config.yaml","theme":"shuffle-light","viewport":{"width":720,"height":960},"author_text":"@旧作者","show_watermark":true,"watermark_text":"旧水印","watermark_position":"bottom-left","page_theme_keys":["default-green","lifestyle-light","default-orange"]}`
}

func legacyRenderMetaDisabledAuthorJSON() string {
	return `{"schema_version":1,"theme":"shuffle-light","viewport":{"width":720,"height":960},"show_author":false,"show_watermark":true,"watermark_text":"旧水印","watermark_position":"bottom-right","page_theme_keys":["default-green","lifestyle-light","default-orange"]}`
}

func legacyRenderMetaThemeOnlyJSON() string {
	return `{"schema_version":1,"theme":"shuffle-light","viewport":{"width":720,"height":960},"page_theme_keys":["default-green","lifestyle-light","default-orange"]}`
}

func legacyRenderMetaShortPageThemeKeysJSON() string {
	return `{"schema_version":1,"theme":"shuffle-light","viewport":{"width":720,"height":960},"show_author":false,"show_watermark":true,"page_theme_keys":["default-green"]}`
}

func writeTestArtifact(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func assertJSONMissingKey(t *testing.T, raw []byte, key string) {
	t.Helper()
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := values[key]; ok {
		t.Fatalf("JSON contains legacy key %q", key)
	}
}

func assertLayoutArtifactsAbsent(t *testing.T, outDir string) {
	t.Helper()
	for _, name := range []string{"deck.json", "render-meta.json"} {
		_, err := os.Stat(filepath.Join(outDir, name))
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(%s) error = %v, want not exist", name, err)
		}
	}
}

func TestServiceGeneratePreviewReturnsImagePaths(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		AI:     config.AICfg{Command: "ccs", Args: []string{"codex"}},
	}
	imagePaths := []string{
		filepath.Join(t.TempDir(), "p01-cover.png"),
		filepath.Join(t.TempDir(), "p02-bullets.png"),
	}
	r := &fakeRenderer{result: render.RenderResult{ImagePaths: imagePaths}}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`, nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !reflect.DeepEqual(result.ImagePaths, imagePaths) {
		t.Fatalf("ImagePaths = %#v, want %#v", result.ImagePaths, imagePaths)
	}
}

func TestServiceGeneratePreviewReturnsRenderWarnings(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		AI:     config.AICfg{Command: "ccs", Args: []string{"codex"}},
	}
	r := &fakeRenderer{result: render.RenderResult{Warnings: []string{"animated export skipped: img2webp not found"}}}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`, nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	result, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	wantWarnings := []string{"animated export skipped: img2webp not found"}
	if !reflect.DeepEqual(result.Warnings, wantWarnings) {
		t.Fatalf("Warnings = %#v, want %#v", result.Warnings, wantWarnings)
	}
}

func TestServiceGeneratePreviewUsesLiveConfigWhenOptionsUntouched(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Live: config.LiveCfg{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: "apple-live", ImportPhotos: true, ImportAlbum: "Live 相册", ImportTimeout: 75 * time.Second}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	want := LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: filepath.Join(gotOpts.OutDir, "apple-live"), ImportPhotos: true, ImportAlbum: "Live 相册", ImportTimeout: 75 * time.Second}
	if gotOpts.Live != want {
		t.Fatalf("Live = %#v, want %#v", gotOpts.Live, want)
	}
}

func TestServiceGeneratePreviewUsesPNGImportConfigWhenOptionsUntouched(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{ImportPhotos: true, ImportAlbum: "PNG 相册", ImportTimeout: 45 * time.Second},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !gotOpts.ImportPhotos || gotOpts.ImportAlbum != "PNG 相册" || gotOpts.ImportTimeout != 45*time.Second {
		t.Fatalf("import options = photos:%v album:%q timeout:%v", gotOpts.ImportPhotos, gotOpts.ImportAlbum, gotOpts.ImportTimeout)
	}
}

func TestServiceGeneratePreviewKeepsLiveOverrideFlagsOverConfig(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Live: config.LiveCfg{Enabled: false, PhotoFormat: "jpeg", CoverFrame: "middle", ImportPhotos: true, ImportAlbum: "config live", ImportTimeout: 75 * time.Second}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Live: LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", ImportPhotos: false, ImportAlbum: "cli live", ImportTimeout: 15 * time.Second}, LiveEnabledChanged: true, LiveCoverFrameChanged: true, LiveImportPhotosChanged: true, LiveImportAlbumChanged: true, LiveImportTimeoutChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	want := LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", ImportPhotos: false, ImportAlbum: "cli live", ImportTimeout: 15 * time.Second}
	if gotOpts.Live != want {
		t.Fatalf("Live = %#v, want %#v", gotOpts.Live, want)
	}
}

func TestServiceGeneratePreviewKeepsPNGImportOverrideFlagsOverConfig(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{ImportPhotos: true, ImportAlbum: "config png", ImportTimeout: 45 * time.Second},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, ImportPhotos: false, ImportAlbum: "cli png", ImportTimeout: 15 * time.Second, ImportPhotosChanged: true, ImportAlbumChanged: true, ImportTimeoutChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if gotOpts.ImportPhotos || gotOpts.ImportAlbum != "cli png" || gotOpts.ImportTimeout != 15*time.Second {
		t.Fatalf("import options = photos:%v album:%q timeout:%v", gotOpts.ImportPhotos, gotOpts.ImportAlbum, gotOpts.ImportTimeout)
	}
}

func TestServiceDefaultRendererReceivesLiveDeliveryOptions(t *testing.T) {
	renderer := Service{}.effectiveNewRenderer()(Options{Live: LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: "/tmp/apple-live", ImportPhotos: true, ImportAlbum: "  Camera Roll  ", ImportTimeout: 45 * time.Second}})
	r, ok := renderer.(render.Renderer)
	if !ok {
		t.Fatalf("renderer type = %T, want render.Renderer", renderer)
	}
	if !r.Live.Enabled || r.Live.PhotoFormat != "jpeg" || r.Live.CoverFrame != "first" || !r.Live.Assemble || r.Live.OutputDir != "/tmp/apple-live" || !r.Live.ImportPhotos || r.Live.ImportAlbum != "  Camera Roll  " || r.Live.ImportTimeout != 45*time.Second {
		t.Fatalf("renderer.Live = %#v", r.Live)
	}
}

func TestServiceDefaultRendererReceivesPNGImportOptions(t *testing.T) {
	renderer := Service{}.effectiveNewRenderer()(Options{ImportPhotos: true, ImportAlbum: "  Camera Roll  ", ImportTimeout: 45 * time.Second})
	r, ok := renderer.(render.Renderer)
	if !ok {
		t.Fatalf("renderer type = %T, want render.Renderer", renderer)
	}
	if !r.ImportPhotos || r.ImportAlbum != "  Camera Roll  " || r.ImportTimeout != 45*time.Second {
		t.Fatalf("renderer import options = photos:%v album:%q timeout:%v", r.ImportPhotos, r.ImportAlbum, r.ImportTimeout)
	}
}

func TestServiceGeneratePreviewRejectsInvalidPNGImportTimeoutBeforeRendererRuns(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`, nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, ImportPhotos: true, ImportTimeout: 0, ImportPhotosChanged: true, ImportTimeoutChanged: true})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrRenderPreview) {
		t.Fatalf("error = %v, want ErrRenderPreview", err)
	}
	if !strings.Contains(err.Error(), "import timeout must be > 0") {
		t.Fatalf("error = %v, want import timeout validation message", err)
	}
	if r.called != 0 {
		t.Fatalf("renderer called %d times, want 0", r.called)
	}
}

func TestServiceGeneratePreviewRejectsInvalidLiveImportTimeoutBeforeRendererRuns(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`, nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Live: LiveOptions{Enabled: true, ImportPhotos: true, Assemble: true, ImportTimeout: -1 * time.Second}, LiveImportPhotosChanged: true, LiveAssembleChanged: true, LiveImportTimeoutChanged: true})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrRenderPreview) {
		t.Fatalf("error = %v, want ErrRenderPreview", err)
	}
	if !strings.Contains(err.Error(), "live import timeout must be > 0") {
		t.Fatalf("error = %v, want live import timeout validation message", err)
	}
	if r.called != 0 {
		t.Fatalf("renderer called %d times, want 0", r.called)
	}
}

func TestServiceGeneratePreviewRejectsImportWithoutAssembleBeforeRendererRuns(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:   func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`, nil
		},
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Live: LiveOptions{Enabled: true, ImportPhotos: true, Assemble: false}, LiveImportPhotosChanged: true})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrRenderPreview) {
		t.Fatalf("error = %v, want ErrRenderPreview", err)
	}
	if !strings.Contains(err.Error(), "live import requires live assemble") {
		t.Fatalf("error = %v, want parameter-style validation message", err)
	}
	if r.called != 0 {
		t.Fatalf("renderer called %d times, want 0", r.called)
	}
}

func TestServiceGeneratePreviewUsesAnimatedConfigWhenCLILeavesDefaultsUntouched(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Animated: config.AnimatedCfg{Enabled: true, Format: "webp", DurationMS: 3200, FPS: 10}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !gotOpts.Animated.Enabled || gotOpts.Animated.Format != "webp" || gotOpts.Animated.DurationMS != 3200 || gotOpts.Animated.FPS != 10 {
		t.Fatalf("Animated = %#v", gotOpts.Animated)
	}
}

func TestServiceGeneratePreviewKeepsCLIAnimatedOverridesOverConfig(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Animated: config.AnimatedCfg{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Animated: AnimatedOptions{Enabled: true, Format: "webp", DurationMS: 3600, FPS: 12}, AnimatedEnabledChanged: true, AnimatedFormatChanged: true, AnimatedDurationChanged: true, AnimatedFPSChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !gotOpts.Animated.Enabled || gotOpts.Animated.DurationMS != 3600 || gotOpts.Animated.FPS != 12 {
		t.Fatalf("Animated = %#v", gotOpts.Animated)
	}
}

func TestServiceGeneratePreviewMixesAnimatedConfigAndPartialCLIOverrides(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Animated: config.AnimatedCfg{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Animated: AnimatedOptions{Enabled: false, Format: "gif", DurationMS: 3600, FPS: 0}, AnimatedEnabledChanged: true, AnimatedDurationChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	want := AnimatedOptions{Enabled: false, Format: "webp", DurationMS: 3600, FPS: 8}
	if gotOpts.Animated != want {
		t.Fatalf("Animated = %#v, want %#v", gotOpts.Animated, want)
	}
}

func TestServiceGeneratePreviewKeepsCLIAnimatedFormatOverrideOnly(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Animated: config.AnimatedCfg{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Animated: AnimatedOptions{Format: "gif"}, AnimatedFormatChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	want := AnimatedOptions{Enabled: true, Format: "gif", DurationMS: 2400, FPS: 8}
	if gotOpts.Animated != want {
		t.Fatalf("Animated = %#v, want %#v", gotOpts.Animated, want)
	}
}

func TestServiceGeneratePreviewKeepsCLIAnimatedFPSOverrideOnly(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Animated: config.AnimatedCfg{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 8}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Animated: AnimatedOptions{FPS: 12}, AnimatedFPSChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	want := AnimatedOptions{Enabled: true, Format: "webp", DurationMS: 2400, FPS: 12}
	if gotOpts.Animated != want {
		t.Fatalf("Animated = %#v, want %#v", gotOpts.Animated, want)
	}
}

func TestServiceGeneratePreviewUsesViewportFromConfigWhenOptionsUnset(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Render: config.RenderCfg{Viewport: config.ViewportCfg{Width: 720, Height: 960}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	var gotOpts Options
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer: func(opts Options) DeckRenderer {
			gotOpts = opts
			return r
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if gotOpts.ViewportWidth != 720 || gotOpts.ViewportHeight != 960 {
		t.Fatalf("viewport opts = %dx%d, want 720x960", gotOpts.ViewportWidth, gotOpts.ViewportHeight)
	}
	if r.rendered.ViewportWidth != 720 || r.rendered.ViewportHeight != 960 {
		t.Fatalf("rendered viewport = %dx%d, want 720x960", r.rendered.ViewportWidth, r.rendered.ViewportHeight)
	}
}

func TestServiceGeneratePreviewAppliesThemeAndAuthorOverrides(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		AI:     config.AICfg{Command: "ccs", Args: []string{"codex"}},
		Deck:   config.DeckCfg{Theme: deck.ThemeEditorialCool, Author: "全局作者"},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Theme: deck.ThemeWarmPaper, Author: "@单次作者"})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeWarmPaper {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeWarmPaper)
	}
	if !r.rendered.ShowAuthor {
		t.Fatalf("ShowAuthor = false, want true")
	}
	if r.rendered.AuthorText != "@单次作者" {
		t.Fatalf("AuthorText = %q, want %q", r.rendered.AuthorText, "@单次作者")
	}
}

func TestServiceGeneratePreviewFallsBackToConfigThemeAndAuthor(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck:   config.DeckCfg{Theme: deck.ThemeEditorialCool, Author: "  全局作者  "},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeEditorialCool {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeEditorialCool)
	}
	if !r.rendered.ShowAuthor || r.rendered.AuthorText != "@全局作者" {
		t.Fatalf("rendered author = %#v", r.rendered)
	}
}

func TestServiceGeneratePreviewUsesWeeklyDeckTheme(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{
			Theme:     deck.ThemeWarmPaper,
			ThemeMode: "weekly",
			WeeklyThemes: map[string]string{
				"mon": deck.ThemePlumInk,
				"wed": deck.ThemeSageMist,
			},
		},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		Now:           func() time.Time { return time.Date(2026, 4, 29, 10, 0, 0, 0, time.Local) },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeSageMist {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeSageMist)
	}
}

func TestServiceGeneratePreviewFallsBackToFixedThemeWhenWeeklyDayMissing(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{
			Theme:     deck.ThemeWarmPaper,
			ThemeMode: "weekly",
			WeeklyThemes: map[string]string{
				"mon": deck.ThemePlumInk,
			},
		},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		Now:           func() time.Time { return time.Date(2026, 5, 3, 10, 0, 0, 0, time.Local) },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeWarmPaper {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeWarmPaper)
	}
}

func TestServiceGeneratePreviewThemeOverrideBeatsWeeklyTheme(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{
			Theme:     deck.ThemeWarmPaper,
			ThemeMode: "weekly",
			WeeklyThemes: map[string]string{
				"wed": deck.ThemeSageMist,
			},
		},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		Now:           func() time.Time { return time.Date(2026, 4, 29, 10, 0, 0, 0, time.Local) },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Theme: deck.ThemePlumInk})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemePlumInk {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemePlumInk)
	}
}

func TestResolveThemeWithPrecedenceAcceptsDefault(t *testing.T) {
	if got := resolveThemeWithPrecedence("", deck.ThemeDefault, deck.ThemeWarmPaper); got != deck.ThemeDefault {
		t.Fatalf("resolveThemeWithPrecedence() = %q, want %q", got, deck.ThemeDefault)
	}
}

func TestResolveThemeWithPrecedenceFallsBackToDefaultForFirstUnknownValue(t *testing.T) {
	if got := resolveThemeWithPrecedence("shuffle-light", deck.ThemeWarmPaper); got != deck.ThemeDefault {
		t.Fatalf("resolveThemeWithPrecedence() = %q, want %q", got, deck.ThemeDefault)
	}
}

func TestServiceGeneratePreviewRetiredOverrideFallsBackToDefaultBeforeWeeklyTheme(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{
			Theme:     deck.ThemeWarmPaper,
			ThemeMode: "weekly",
			WeeklyThemes: map[string]string{
				"wed": deck.ThemeSageMist,
			},
		},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
		Now:           func() time.Time { return time.Date(2026, 4, 29, 10, 0, 0, 0, time.Local) },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Theme: "shuffle-light"})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeDefault)
	}
}

func TestServiceGenerateFromDeckRetiredMetaThemeFallsBackToDefaultBeforeConfig(t *testing.T) {
	deckDir := t.TempDir()
	deckPath := filepath.Join(deckDir, "deck.json")
	deckJSON := `{"theme":"fresh-green","viewport":{"width":720,"height":960},"pages":[{"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"default","cta":"cta1"},"content":{"title":"封面"}},{"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"default","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"default","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	metaJSON := `{"schema_version":1,"theme":"editorial-mono","viewport":{"width":720,"height":960}}`
	if err := os.WriteFile(deckPath, []byte(deckJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(deck) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(deckDir, "render-meta.json"), []byte(metaJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(meta) error = %v", err)
	}

	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck:   config.DeckCfg{Theme: deck.ThemeTechNoir},
	}
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:  func(string) (*config.Config, error) { return cfg, nil },
		NewRenderer: func(Options) DeckRenderer { return r },
	}

	_, err := svc.GenerateFromDeck(Options{FromDeckPath: deckPath, ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GenerateFromDeck() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeDefault)
	}
}

func TestServiceGeneratePreviewHydratesDefaultWatermarkRuntimeFields(t *testing.T) {
	enabled := true
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{Watermark: config.WatermarkCfg{
			Enabled:  &enabled,
			Text:     "   ",
			Position: "bottom-left",
		}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !r.rendered.ShowWatermark {
		t.Fatalf("ShowWatermark = false, want true")
	}
	if r.rendered.WatermarkText != "walker1211/mark2note" {
		t.Fatalf("WatermarkText = %q, want %q", r.rendered.WatermarkText, "walker1211/mark2note")
	}
	if r.rendered.WatermarkPosition != "bottom-left" {
		t.Fatalf("WatermarkPosition = %q, want %q", r.rendered.WatermarkPosition, "bottom-left")
	}
}

func TestServiceGeneratePreviewTreatsNilWatermarkEnabledAsDefaultOn(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{Watermark: config.WatermarkCfg{
			Text:     "   ",
			Position: "top-left",
		}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !r.rendered.ShowWatermark {
		t.Fatalf("ShowWatermark = false, want true")
	}
	if r.rendered.WatermarkText != "walker1211/mark2note" {
		t.Fatalf("WatermarkText = %q, want %q", r.rendered.WatermarkText, "walker1211/mark2note")
	}
	if r.rendered.WatermarkPosition != "bottom-right" {
		t.Fatalf("WatermarkPosition = %q, want %q", r.rendered.WatermarkPosition, "bottom-right")
	}
}

func TestServiceGeneratePreviewHydratesDisabledWatermarkRuntimeFields(t *testing.T) {
	enabled := false
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{Watermark: config.WatermarkCfg{
			Enabled:  &enabled,
			Text:     "  自定义水印  ",
			Position: "bottom-right",
		}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ShowWatermark {
		t.Fatalf("ShowWatermark = true, want false")
	}
	if r.rendered.WatermarkText != "自定义水印" {
		t.Fatalf("WatermarkText = %q, want %q", r.rendered.WatermarkText, "自定义水印")
	}
	if r.rendered.WatermarkPosition != "bottom-right" {
		t.Fatalf("WatermarkPosition = %q, want %q", r.rendered.WatermarkPosition, "bottom-right")
	}
}

func TestServiceGeneratePreviewFallsBackToDefaultWatermarkPositionWhenInvalid(t *testing.T) {
	enabled := true
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck: config.DeckCfg{Watermark: config.WatermarkCfg{
			Enabled:  &enabled,
			Text:     "watermark",
			Position: "top-right",
		}},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.WatermarkPosition != "bottom-right" {
		t.Fatalf("WatermarkPosition = %q, want %q", r.rendered.WatermarkPosition, "bottom-right")
	}
}

func TestServiceGeneratePreviewFallsBackToDefaultWhenOverrideThemeIsInvalid(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{Theme: deck.ThemeEditorialCool}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Theme: "missing-theme"})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeDefault)
	}
}

func TestServiceGeneratePreviewFallsBackToDeckJSONThemeWhenInputsMissing(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{Theme: ""}}
	deckJSON := `{"theme":"warm-paper","pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeWarmPaper {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeWarmPaper)
	}
}

func TestServiceGeneratePreviewKeepsDefaultThemeWhenInputsMissing(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeDefault)
	}
	if r.rendered.ShowAuthor {
		t.Fatalf("ShowAuthor = true, want false")
	}
}

func TestServiceGeneratePreviewNormalizesWhitespaceAuthor(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: t.TempDir()},
		Deck:   config.DeckCfg{Author: "  全局作者  "},
	}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Author: "   "})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if !r.rendered.ShowAuthor || r.rendered.AuthorText != "@全局作者" {
		t.Fatalf("rendered author = %#v", r.rendered)
	}
}

func TestServiceGeneratePreviewFallsBackToDefaultWhenConfigThemeIsInvalid(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: t.TempDir()}, Deck: config.DeckCfg{Theme: "missing-theme"}}
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	r := &fakeRenderer{}
	svc := Service{
		LoadConfig:    func(string) (*config.Config, error) { return cfg, nil },
		ReadFile:      func(string) ([]byte, error) { return []byte("# 标题"), nil },
		BuildDeckJSON: func(*config.Config, string) (string, error) { return deckJSON, nil },
		NewRenderer:   func(Options) DeckRenderer { return r },
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	if r.rendered.ThemeName != deck.ThemeDefault {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeDefault)
	}
}

func TestServiceGeneratePreviewWrapsSchemaInvalidDeck(t *testing.T) {
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: "output"}}, nil
		},
		ReadFile: func(string) ([]byte, error) {
			return []byte("# 标题"), nil
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return `{"pages":[]}`, nil
		},
		NewRenderer: func(Options) DeckRenderer {
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrParseDeck) {
		t.Fatalf("error = %v, want ErrParseDeck", err)
	}
	if errors.Is(err, ai.ErrAIInvalidJSON) {
		t.Fatalf("error = %v, should not be ai.ErrAIInvalidJSON", err)
	}
	if !strings.Contains(err.Error(), "parse deck") {
		t.Fatalf("error = %v, want parse deck context", err)
	}
}

func TestServiceGeneratePreviewPreservesAIInvalidJSONError(t *testing.T) {
	svc := Service{
		LoadConfig: func(string) (*config.Config, error) {
			return &config.Config{Output: config.OutputCfg{Dir: "output"}}, nil
		},
		ReadFile: func(string) ([]byte, error) {
			return []byte("# 标题"), nil
		},
		BuildDeckJSON: func(*config.Config, string) (string, error) {
			return "", fmt.Errorf("%w: trailing comma", ai.ErrAIInvalidJSON)
		},
		NewRenderer: func(Options) DeckRenderer {
			return &fakeRenderer{}
		},
	}

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2})
	if err == nil {
		t.Fatalf("GeneratePreview() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrBuildDeckJSON) {
		t.Fatalf("error = %v, want ErrBuildDeckJSON", err)
	}
	if !errors.Is(err, ai.ErrAIInvalidJSON) {
		t.Fatalf("error = %v, want ai.ErrAIInvalidJSON", err)
	}
	if errors.Is(err, ErrParseDeck) {
		t.Fatalf("error = %v, should not be parse deck failure", err)
	}
}
