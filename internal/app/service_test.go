package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/walker1211/mark2note/internal/ai"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/deck"
	"github.com/walker1211/mark2note/internal/render"
)

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

func TestServiceGeneratePreviewSuccess(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: "configured-output"},
		AI:     config.AICfg{Command: "ccs", Args: []string{"codex"}},
	}
	markdown := "# 标题"
	deckJSON := `{"pages":[{"name":"p1-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},{"name":"p2-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["要点"]}},{"name":"p3-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}]}`
	fixedNow := time.Date(2026, 3, 28, 15, 30, 0, 0, time.UTC)
	wantOutDir, err := filepath.Abs(filepath.Join("configured-output", "article-20260328-153000"))
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

func TestServiceGeneratePreviewReturnsRenderWarnings(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
		Render: config.RenderCfg{Live: config.LiveCfg{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: "apple-live"}},
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
	want := LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first", Assemble: true, OutputDir: filepath.Join(gotOpts.OutDir, "apple-live")}
	if gotOpts.Live != want {
		t.Fatalf("Live = %#v, want %#v", gotOpts.Live, want)
	}
}

func TestServiceGeneratePreviewKeepsLiveOverrideFlagsOverConfig(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: "configured-output"},
		Render: config.RenderCfg{Live: config.LiveCfg{Enabled: false, PhotoFormat: "jpeg", CoverFrame: "middle"}},
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

	_, err := svc.GeneratePreview(Options{InputPath: "article.md", ConfigPath: "config.yaml", Jobs: 2, Live: LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first"}, LiveEnabledChanged: true, LiveCoverFrameChanged: true})
	if err != nil {
		t.Fatalf("GeneratePreview() error = %v", err)
	}
	want := LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first"}
	if gotOpts.Live != want {
		t.Fatalf("Live = %#v, want %#v", gotOpts.Live, want)
	}
}

func TestServiceDefaultRendererReceivesLiveOptions(t *testing.T) {
	renderer := Service{}.effectiveNewRenderer()(Options{Live: LiveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "first"}})
	r, ok := renderer.(render.Renderer)
	if !ok {
		t.Fatalf("renderer type = %T, want render.Renderer", renderer)
	}
	if !r.Live.Enabled || r.Live.PhotoFormat != "jpeg" || r.Live.CoverFrame != "first" || r.Live.Assemble || r.Live.OutputDir != "" {
		t.Fatalf("renderer.Live = %#v", r.Live)
	}
}

func TestServiceGeneratePreviewUsesAnimatedConfigWhenCLILeavesDefaultsUntouched(t *testing.T) {
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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

func TestServiceGeneratePreviewHydratesDefaultWatermarkRuntimeFields(t *testing.T) {
	enabled := true
	cfg := &config.Config{
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
		Output: config.OutputCfg{Dir: "configured-output"},
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

func TestServiceGeneratePreviewFallsBackToConfigThemeWhenOverrideThemeIsInvalid(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: "configured-output"}, Deck: config.DeckCfg{Theme: deck.ThemeEditorialCool}}
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
	if r.rendered.ThemeName != deck.ThemeEditorialCool {
		t.Fatalf("ThemeName = %q, want %q", r.rendered.ThemeName, deck.ThemeEditorialCool)
	}
}

func TestServiceGeneratePreviewFallsBackToDeckJSONThemeWhenInputsMissing(t *testing.T) {
	cfg := &config.Config{Output: config.OutputCfg{Dir: "configured-output"}, Deck: config.DeckCfg{Theme: ""}}
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
	cfg := &config.Config{Output: config.OutputCfg{Dir: "configured-output"}}
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
		Output: config.OutputCfg{Dir: "configured-output"},
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
	cfg := &config.Config{Output: config.OutputCfg{Dir: "configured-output"}, Deck: config.DeckCfg{Theme: "missing-theme"}}
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
