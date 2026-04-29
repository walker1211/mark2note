package app

import (
	"encoding/json"
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
	Enabled       bool
	PhotoFormat   string
	CoverFrame    string
	Assemble      bool
	OutputDir     string
	ImportPhotos  bool
	ImportAlbum   string
	ImportTimeout time.Duration
}

type Options struct {
	OutDir                   string
	ChromePath               string
	ChromePathChanged        bool
	Jobs                     int
	InputPath                string
	FromDeckPath             string
	ConfigPath               string
	OutDirChanged            bool
	Theme                    string
	Author                   string
	PromptExtra              string
	PublishXHS               bool
	XHSTags                  []string
	XHSTagsChanged           bool
	Animated                 AnimatedOptions
	Live                     LiveOptions
	ImportPhotos             bool
	ImportAlbum              string
	ImportTimeout            time.Duration
	ImportPhotosChanged      bool
	ImportAlbumChanged       bool
	ImportTimeoutChanged     bool
	AnimatedEnabledChanged   bool
	AnimatedFormatChanged    bool
	AnimatedDurationChanged  bool
	AnimatedFPSChanged       bool
	LiveEnabledChanged       bool
	LivePhotoFormatChanged   bool
	LiveCoverFrameChanged    bool
	LiveAssembleChanged      bool
	LiveOutputDirChanged     bool
	LiveImportPhotosChanged  bool
	LiveImportAlbumChanged   bool
	LiveImportTimeoutChanged bool
	ViewportWidth            int
	ViewportHeight           int
}

type Result struct {
	PageCount          int
	OutDir             string
	Warnings           []string
	ImagePaths         []string
	ImportReport       *render.DeliveryReport
	ImportReportPath   string
	DeliveryReport     *render.DeliveryReport
	DeliveryReportPath string
}

type DeckRenderer interface {
	Render(deck.Deck) (render.RenderResult, error)
}

type Service struct {
	LoadConfig      func(string) (*config.Config, error)
	ReadFile        func(string) ([]byte, error)
	BuildDeckJSON   func(*config.Config, string) (string, error)
	NewRenderer     func(Options) DeckRenderer
	Now             func() time.Time
	PromptExtra     string
	AICommandRunner ai.CommandRunner
}

var (
	ErrLoadConfig     = errors.New("load config failed")
	ErrReadMarkdown   = errors.New("read markdown failed")
	ErrReadDeck       = errors.New("read deck failed")
	ErrReadRenderMeta = errors.New("read render meta failed")
	ErrBuildDeckJSON  = errors.New("build deck json failed")
	ErrParseDeck      = errors.New("parse deck failed")
	ErrRenderPreview  = errors.New("render preview failed")
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

	s.PromptExtra = opts.PromptExtra
	rawJSON, err := s.effectiveBuildDeckJSON()(cfg, string(markdownBytes))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrBuildDeckJSON, err)
	}

	d, err := deck.FromJSON(rawJSON, opts.OutDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrParseDeck, err)
	}

	return s.renderDeck(opts, cfg, d, sourceRenderMeta{})
}

func (s Service) GenerateFromDeck(opts Options) (Result, error) {
	cfg, err := s.effectiveLoadConfig()(opts.ConfigPath)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrLoadConfig, err)
	}
	if !opts.OutDirChanged {
		opts.OutDir = filepath.Join(cfg.Output.Dir, defaultDeckOutputDirName(opts.FromDeckPath, s.effectiveNow()()))
	}
	if !filepath.IsAbs(opts.OutDir) {
		if abs, absErr := filepath.Abs(opts.OutDir); absErr == nil {
			opts.OutDir = abs
		}
	}
	deckBytes, err := s.effectiveReadFile()(opts.FromDeckPath)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrReadDeck, err)
	}
	d, err := deck.FromJSON(string(deckBytes), opts.OutDir)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrParseDeck, err)
	}
	meta, err := s.readRenderMetaForDeck(opts.FromDeckPath)
	if err != nil {
		return Result{}, err
	}
	return s.renderDeck(opts, cfg, d, sourceRenderMeta{FromDeck: true, Meta: meta})
}

func (s Service) renderDeck(opts Options, cfg *config.Config, d deck.Deck, source sourceRenderMeta) (Result, error) {
	meta := source.Meta
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
	if !opts.ImportPhotosChanged {
		opts.ImportPhotos = cfg.Render.ImportPhotos
	}
	if !opts.ImportAlbumChanged {
		opts.ImportAlbum = cfg.Render.ImportAlbum
	}
	if !opts.ImportTimeoutChanged {
		opts.ImportTimeout = cfg.Render.ImportTimeout
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
	if !opts.LiveImportPhotosChanged {
		opts.Live.ImportPhotos = cfg.Render.Live.ImportPhotos
	}
	if !opts.LiveImportAlbumChanged {
		opts.Live.ImportAlbum = cfg.Render.Live.ImportAlbum
	}
	if !opts.LiveImportTimeoutChanged {
		opts.Live.ImportTimeout = cfg.Render.Live.ImportTimeout
	}
	if strings.TrimSpace(opts.Live.OutputDir) != "" && !filepath.IsAbs(opts.Live.OutputDir) {
		opts.Live.OutputDir = filepath.Join(opts.OutDir, opts.Live.OutputDir)
	}
	if opts.ImportPhotos && opts.ImportTimeout <= 0 {
		return Result{}, fmt.Errorf("%w: invalid parameter: import timeout must be > 0", ErrRenderPreview)
	}
	if opts.Live.ImportPhotos && !opts.Live.Assemble {
		return Result{}, fmt.Errorf("%w: invalid parameter: live import requires live assemble", ErrRenderPreview)
	}
	if opts.Live.ImportPhotos && opts.Live.ImportTimeout <= 0 {
		return Result{}, fmt.Errorf("%w: invalid parameter: live import timeout must be > 0", ErrRenderPreview)
	}
	if meta != nil {
		if len(meta.PageThemeKeys) > 0 {
			d.PageThemeKeys = append([]string(nil), meta.PageThemeKeys...)
		}
		if meta.Viewport.Width > 0 && opts.ViewportWidth == 0 {
			opts.ViewportWidth = meta.Viewport.Width
		}
		if meta.Viewport.Height > 0 && opts.ViewportHeight == 0 {
			opts.ViewportHeight = meta.Viewport.Height
		}
	}
	if source.FromDeck && opts.ViewportWidth == 0 && d.ViewportWidth > 0 {
		opts.ViewportWidth = d.ViewportWidth
	}
	if source.FromDeck && opts.ViewportHeight == 0 && d.ViewportHeight > 0 {
		opts.ViewportHeight = d.ViewportHeight
	}
	if opts.ViewportWidth == 0 {
		opts.ViewportWidth = cfg.Render.Viewport.Width
	}
	if opts.ViewportHeight == 0 {
		opts.ViewportHeight = cfg.Render.Viewport.Height
	}

	metaTheme := ""
	if meta != nil {
		metaTheme = meta.Theme
	}
	configTheme := resolveConfigDeckTheme(cfg.Deck, s.effectiveNow()())
	d.ThemeName = resolveThemeWithPrecedence(opts.Theme, metaTheme, configTheme, d.ThemeName)
	if err := assignPageThemesForRender(&d); err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrParseDeck, err)
	}
	d.ViewportWidth = opts.ViewportWidth
	d.ViewportHeight = opts.ViewportHeight
	if strings.TrimSpace(opts.Author) != "" {
		author := deck.ResolveCoverAuthor(opts.Author, "")
		d.ShowAuthor = author.Show
		d.AuthorText = author.Text
	} else if meta != nil && meta.ShowAuthor != nil {
		d.ShowAuthor = *meta.ShowAuthor
		d.AuthorText = strings.TrimSpace(meta.AuthorText)
	} else if meta != nil && strings.TrimSpace(meta.AuthorText) != "" {
		d.ShowAuthor = true
		d.AuthorText = strings.TrimSpace(meta.AuthorText)
	} else {
		author := deck.ResolveCoverAuthor("", cfg.Deck.Author)
		d.ShowAuthor = author.Show
		d.AuthorText = author.Text
	}
	if meta != nil {
		d.ShowWatermark = meta.ShowWatermark
		d.WatermarkText = resolveWatermarkText(meta.WatermarkText)
		d.WatermarkPosition = resolveWatermarkPosition(meta.WatermarkPosition)
	} else {
		d.ShowWatermark = resolveWatermarkEnabled(cfg.Deck.Watermark.Enabled)
		d.WatermarkText = resolveWatermarkText(cfg.Deck.Watermark.Text)
		d.WatermarkPosition = resolveWatermarkPosition(cfg.Deck.Watermark.Position)
	}
	d.Themes = deck.RegisteredThemes()

	renderResult, err := s.effectiveNewRenderer()(opts).Render(d)
	result := Result{
		PageCount:          len(d.Pages),
		OutDir:             opts.OutDir,
		Warnings:           append([]string(nil), renderResult.Warnings...),
		ImagePaths:         append([]string(nil), renderResult.ImagePaths...),
		ImportReport:       renderResult.ImportReport,
		ImportReportPath:   renderResult.ImportReportPath,
		DeliveryReport:     renderResult.DeliveryReport,
		DeliveryReportPath: renderResult.DeliveryReportPath,
	}
	if err != nil {
		if cleanupErr := removeLayoutArtifacts(opts.OutDir); cleanupErr != nil {
			return result, fmt.Errorf("%w: %v; cleanup layout artifacts: %v", ErrRenderPreview, err, cleanupErr)
		}
		return result, fmt.Errorf("%w: %v", ErrRenderPreview, err)
	}
	artifactInputPath := opts.InputPath
	artifactConfigPath := opts.ConfigPath
	if meta != nil {
		if strings.TrimSpace(meta.InputPath) != "" {
			artifactInputPath = meta.InputPath
		}
		if strings.TrimSpace(meta.ConfigPath) != "" {
			artifactConfigPath = meta.ConfigPath
		}
	}
	if err := writeLayoutArtifacts(opts.OutDir, artifactInputPath, artifactConfigPath, d); err != nil {
		return result, fmt.Errorf("%w: %v", ErrRenderPreview, err)
	}

	return result, nil
}

func assignPageThemesForRender(d *deck.Deck) error {
	d.PageThemeKeys = nil
	return d.Validate()
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
		b := ai.Builder{PromptExtra: s.PromptExtra, Runner: s.AICommandRunner}
		b.SetCommand(cfg.AI.Command, cfg.AI.Args)
		return b.BuildDeckJSON(markdown)
	}
}

type persistedDeck struct {
	Theme         string      `json:"theme"`
	PageThemeKeys []string    `json:"page_theme_keys,omitempty"`
	Viewport      viewportRef `json:"viewport"`
	Pages         []deck.Page `json:"pages"`
}

type renderMeta struct {
	SchemaVersion     int         `json:"schema_version"`
	InputPath         string      `json:"input_path"`
	ConfigPath        string      `json:"config_path"`
	Theme             string      `json:"theme"`
	Viewport          viewportRef `json:"viewport"`
	ShowAuthor        *bool       `json:"show_author,omitempty"`
	AuthorText        string      `json:"author_text,omitempty"`
	ShowWatermark     bool        `json:"show_watermark"`
	WatermarkText     string      `json:"watermark_text,omitempty"`
	WatermarkPosition string      `json:"watermark_position,omitempty"`
	PageThemeKeys     []string    `json:"page_theme_keys,omitempty"`
}

type sourceRenderMeta struct {
	FromDeck bool
	Meta     *renderMeta
}

type viewportRef struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func removeLayoutArtifacts(outDir string) error {
	for _, name := range []string{"deck.json", "render-meta.json"} {
		path := filepath.Join(outDir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func writeLayoutArtifacts(outDir string, inputPath string, configPath string, d deck.Deck) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create layout artifact dir: %w", err)
	}
	viewport := viewportRef{Width: d.ViewportWidth, Height: d.ViewportHeight}
	deckArtifact := persistedDeck{
		Theme:         d.ThemeName,
		PageThemeKeys: append([]string(nil), d.PageThemeKeys...),
		Viewport:      viewport,
		Pages:         append([]deck.Page(nil), d.Pages...),
	}
	deckPath := filepath.Join(outDir, "deck.json")
	if err := writeJSONFile(deckPath, deckArtifact); err != nil {
		if cleanupErr := removeLayoutArtifacts(outDir); cleanupErr != nil {
			return fmt.Errorf("%w; cleanup layout artifacts: %v", err, cleanupErr)
		}
		return err
	}
	showAuthor := d.ShowAuthor
	meta := renderMeta{
		SchemaVersion:     1,
		InputPath:         inputPath,
		ConfigPath:        configPath,
		Theme:             d.ThemeName,
		Viewport:          viewport,
		ShowAuthor:        &showAuthor,
		AuthorText:        d.AuthorText,
		ShowWatermark:     d.ShowWatermark,
		WatermarkText:     d.WatermarkText,
		WatermarkPosition: d.WatermarkPosition,
		PageThemeKeys:     append([]string(nil), d.PageThemeKeys...),
	}
	if err := writeJSONFile(filepath.Join(outDir, "render-meta.json"), meta); err != nil {
		if cleanupErr := removeLayoutArtifacts(outDir); cleanupErr != nil {
			return fmt.Errorf("%w; cleanup layout artifacts: %v", err, cleanupErr)
		}
		return err
	}
	return nil
}

func (s Service) readRenderMetaForDeck(fromDeckPath string) (*renderMeta, error) {
	metaPath := filepath.Join(filepath.Dir(fromDeckPath), "render-meta.json")
	content, err := s.effectiveReadFile()(metaPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrReadRenderMeta, err)
	}
	var meta renderMeta
	if err := json.Unmarshal(content, &meta); err != nil {
		return nil, fmt.Errorf("%w: parse render-meta.json: %v", ErrReadRenderMeta, err)
	}
	return &meta, nil
}

func writeJSONFile(path string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
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
			ImportPhotos:   opts.ImportPhotos,
			ImportAlbum:    opts.ImportAlbum,
			ImportTimeout:  opts.ImportTimeout,
			Animated: render.AnimatedOptions{
				Enabled:    opts.Animated.Enabled,
				Format:     opts.Animated.Format,
				DurationMS: opts.Animated.DurationMS,
				FPS:        opts.Animated.FPS,
			},
			Live: render.LiveOptions{
				Enabled:       opts.Live.Enabled,
				PhotoFormat:   opts.Live.PhotoFormat,
				CoverFrame:    opts.Live.CoverFrame,
				Assemble:      opts.Live.Assemble,
				OutputDir:     opts.Live.OutputDir,
				ImportPhotos:  opts.Live.ImportPhotos,
				ImportAlbum:   opts.Live.ImportAlbum,
				ImportTimeout: opts.Live.ImportTimeout,
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

func defaultDeckOutputDirName(fromDeckPath string, now time.Time) string {
	parent := filepath.Base(filepath.Dir(strings.TrimSpace(fromDeckPath)))
	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		parent = "deck"
	}
	return parent + "-" + now.Format("20060102-150405")
}

func resolveConfigDeckTheme(cfg config.DeckCfg, now time.Time) string {
	if strings.TrimSpace(cfg.ThemeMode) != "weekly" {
		return strings.TrimSpace(cfg.Theme)
	}
	weekday := strings.ToLower(now.Weekday().String()[:3])
	weeklyTheme := strings.TrimSpace(cfg.WeeklyThemes[weekday])
	if weeklyTheme != "" {
		return weeklyTheme
	}
	return strings.TrimSpace(cfg.Theme)
}

func resolveThemeWithPrecedence(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		return deck.ResolveDeckTheme(trimmed)
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
