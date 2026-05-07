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

	"github.com/walker1211/mark2note/internal/ai"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/deck"
	"github.com/walker1211/mark2note/internal/poster"
	"github.com/walker1211/mark2note/internal/render"
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
	if !strings.Contains(prompt, "以下是本次生成的额外偏好") {
		t.Fatalf("prompt = %q, want extra guidance wrapper", prompt)
	}
	if !strings.Contains(prompt, "封面更抓眼") {
		t.Fatalf("prompt = %q, want PromptExtra contents", prompt)
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
