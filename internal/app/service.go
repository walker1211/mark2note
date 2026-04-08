package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/walker1211/mark2note/internal/ai"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/deck"
	"github.com/walker1211/mark2note/internal/render"
)

type AnimatedOptions struct {
	Enabled    bool
	Format     string
	DurationMS int
	FPS        int
}

type LiveOptions struct {
	Enabled     bool
	PhotoFormat string
	CoverFrame  string
	Assemble    bool
	OutputDir   string
}

type Options struct {
	OutDir                  string
	ChromePath              string
	Jobs                    int
	InputPath               string
	ConfigPath              string
	OutDirChanged           bool
	Theme                   string
	Author                  string
	Animated                AnimatedOptions
	Live                    LiveOptions
	AnimatedEnabledChanged  bool
	AnimatedFormatChanged   bool
	AnimatedDurationChanged bool
	AnimatedFPSChanged      bool
	LiveEnabledChanged      bool
	LivePhotoFormatChanged  bool
	LiveCoverFrameChanged   bool
	LiveAssembleChanged     bool
	LiveOutputDirChanged    bool
	ViewportWidth           int
	ViewportHeight          int
}

type Result struct {
	PageCount int
	OutDir    string
	Warnings  []string
}

type DeckRenderer interface {
	Render(deck.Deck) (render.RenderResult, error)
}

type Service struct {
	LoadConfig    func(string) (*config.Config, error)
	ReadFile      func(string) ([]byte, error)
	BuildDeckJSON func(*config.Config, string) (string, error)
	NewRenderer   func(Options) DeckRenderer
	Now           func() time.Time
}

var (
	ErrLoadConfig    = errors.New("load config failed")
	ErrReadMarkdown  = errors.New("read markdown failed")
	ErrBuildDeckJSON = errors.New("build deck json failed")
	ErrParseDeck     = errors.New("parse deck failed")
	ErrRenderPreview = errors.New("render preview failed")
)

func (s Service) GeneratePreview(opts Options) (Result, error) {
	cfg, err := s.effectiveLoadConfig()(opts.ConfigPath)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrLoadConfig, err)
	}

	if !opts.OutDirChanged {
		opts.OutDir = filepath.Join(cfg.Output.Dir, defaultOutputDirName(opts.InputPath, s.effectiveNow()()))
	}
	if !filepath.IsAbs(opts.OutDir) {
		if abs, absErr := filepath.Abs(opts.OutDir); absErr == nil {
			opts.OutDir = abs
		}
	}

	markdownBytes, err := s.effectiveReadFile()(opts.InputPath)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrReadMarkdown, err)
	}

	rawJSON, err := s.effectiveBuildDeckJSON()(cfg, string(markdownBytes))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrBuildDeckJSON, err)
	}

	d, err := deck.FromJSON(rawJSON, opts.OutDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrParseDeck, err)
	}

	if !opts.AnimatedEnabledChanged {
		opts.Animated.Enabled = cfg.Render.Animated.Enabled
	}
	if !opts.AnimatedFormatChanged {
		opts.Animated.Format = cfg.Render.Animated.Format
	}
	if !opts.AnimatedDurationChanged {
		opts.Animated.DurationMS = cfg.Render.Animated.DurationMS
	}
	if !opts.AnimatedFPSChanged {
		opts.Animated.FPS = cfg.Render.Animated.FPS
	}
	if !opts.LiveEnabledChanged {
		opts.Live.Enabled = cfg.Render.Live.Enabled
	}
	if !opts.LivePhotoFormatChanged {
		opts.Live.PhotoFormat = cfg.Render.Live.PhotoFormat
	}
	if !opts.LiveCoverFrameChanged {
		opts.Live.CoverFrame = cfg.Render.Live.CoverFrame
	}
	if !opts.LiveAssembleChanged {
		opts.Live.Assemble = cfg.Render.Live.Assemble
	}
	if !opts.LiveOutputDirChanged {
		opts.Live.OutputDir = cfg.Render.Live.OutputDir
	}
	if strings.TrimSpace(opts.Live.OutputDir) != "" && !filepath.IsAbs(opts.Live.OutputDir) {
		opts.Live.OutputDir = filepath.Join(opts.OutDir, opts.Live.OutputDir)
	}
	if opts.ViewportWidth == 0 {
		opts.ViewportWidth = cfg.Render.Viewport.Width
	}
	if opts.ViewportHeight == 0 {
		opts.ViewportHeight = cfg.Render.Viewport.Height
	}

	d.ThemeName = resolveThemeWithPrecedence(opts.Theme, cfg.Deck.Theme, d.ThemeName)
	d.ViewportWidth = opts.ViewportWidth
	d.ViewportHeight = opts.ViewportHeight
	author := deck.ResolveCoverAuthor(opts.Author, cfg.Deck.Author)
	d.ShowAuthor = author.Show
	d.AuthorText = author.Text
	d.ShowWatermark = resolveWatermarkEnabled(cfg.Deck.Watermark.Enabled)
	d.WatermarkText = resolveWatermarkText(cfg.Deck.Watermark.Text)
	d.WatermarkPosition = resolveWatermarkPosition(cfg.Deck.Watermark.Position)
	d.Themes = deck.RegisteredThemes()

	renderResult, err := s.effectiveNewRenderer()(opts).Render(d)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrRenderPreview, err)
	}

	return Result{PageCount: len(d.Pages), OutDir: opts.OutDir, Warnings: append([]string(nil), renderResult.Warnings...)}, nil
}

func (s Service) effectiveLoadConfig() func(string) (*config.Config, error) {
	if s.LoadConfig != nil {
		return s.LoadConfig
	}
	return config.Load
}

func (s Service) effectiveReadFile() func(string) ([]byte, error) {
	if s.ReadFile != nil {
		return s.ReadFile
	}
	return os.ReadFile
}

func (s Service) effectiveBuildDeckJSON() func(*config.Config, string) (string, error) {
	if s.BuildDeckJSON != nil {
		return s.BuildDeckJSON
	}
	return func(cfg *config.Config, markdown string) (string, error) {
		b := ai.Builder{}
		b.SetCommand(cfg.AI.Command, cfg.AI.Args)
		return b.BuildDeckJSON(markdown)
	}
}

func (s Service) effectiveNewRenderer() func(Options) DeckRenderer {
	if s.NewRenderer != nil {
		return s.NewRenderer
	}
	return func(opts Options) DeckRenderer {
		return render.Renderer{
			OutDir:         opts.OutDir,
			ChromePath:     opts.ChromePath,
			Jobs:           opts.Jobs,
			ViewportWidth:  opts.ViewportWidth,
			ViewportHeight: opts.ViewportHeight,
			Animated: render.AnimatedOptions{
				Enabled:    opts.Animated.Enabled,
				Format:     opts.Animated.Format,
				DurationMS: opts.Animated.DurationMS,
				FPS:        opts.Animated.FPS,
			},
			Live: render.LiveOptions{
				Enabled:     opts.Live.Enabled,
				PhotoFormat: opts.Live.PhotoFormat,
				CoverFrame:  opts.Live.CoverFrame,
				Assemble:    opts.Live.Assemble,
				OutputDir:   opts.Live.OutputDir,
			},
		}
	}
}

func (s Service) effectiveNow() func() time.Time {
	if s.Now != nil {
		return s.Now
	}
	return time.Now
}

func defaultOutputDirName(inputPath string, now time.Time) string {
	base := filepath.Base(strings.TrimSpace(inputPath))
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == "" || name == "." {
		name = "deck"
	}
	return name + "-" + now.Format("20060102-150405")
}

func resolveThemeWithPrecedence(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		resolved := deck.ResolveDeckTheme(trimmed)
		if trimmed == deck.ThemeDefault || resolved != deck.ThemeDefault {
			return resolved
		}
	}
	return deck.ThemeDefault
}

func resolveWatermarkEnabled(enabled *bool) bool {
	if enabled == nil {
		return true
	}
	return *enabled
}

func resolveWatermarkText(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "walker1211/mark2note"
	}
	return trimmed
}

func resolveWatermarkPosition(position string) string {
	switch strings.TrimSpace(position) {
	case "bottom-left", "bottom-right":
		return strings.TrimSpace(position)
	default:
		return "bottom-right"
	}
}
