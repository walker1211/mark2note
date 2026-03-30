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

type Options struct {
	OutDir        string
	ChromePath    string
	Jobs          int
	InputPath     string
	ConfigPath    string
	OutDirChanged bool
	Theme         string
	Author        string
}

type Result struct {
	PageCount int
	OutDir    string
}

type DeckRenderer interface {
	Render(deck.Deck) error
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

	d.ThemeName = resolveThemeWithPrecedence(opts.Theme, cfg.Deck.Theme, d.ThemeName)
	author := deck.ResolveCoverAuthor(opts.Author, cfg.Deck.Author)
	d.ShowAuthor = author.Show
	d.AuthorText = author.Text
	d.Themes = deck.RegisteredThemes()

	if err := s.effectiveNewRenderer()(opts).Render(d); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrRenderPreview, err)
	}

	return Result{PageCount: len(d.Pages), OutDir: opts.OutDir}, nil
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
			OutDir:     opts.OutDir,
			ChromePath: opts.ChromePath,
			Jobs:       opts.Jobs,
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
