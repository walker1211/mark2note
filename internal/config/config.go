package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Output OutputCfg `yaml:"output"`
	AI     AICfg     `yaml:"ai"`
	Deck   DeckCfg   `yaml:"deck"`
	Render RenderCfg `yaml:"render"`
	XHS    XHSCfg    `yaml:"xhs"`
}

type OutputCfg struct {
	Dir string `yaml:"dir"`
}

type AICfg struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type DeckCfg struct {
	Theme     string       `yaml:"theme"`
	Author    string       `yaml:"author"`
	Watermark WatermarkCfg `yaml:"watermark"`
}

type WatermarkCfg struct {
	Enabled  *bool  `yaml:"enabled"`
	Text     string `yaml:"text"`
	Position string `yaml:"position"`
}

type RenderCfg struct {
	Viewport      ViewportCfg   `yaml:"viewport"`
	Animated      AnimatedCfg   `yaml:"animated"`
	ImportPhotos  bool          `yaml:"import_photos"`
	ImportAlbum   string        `yaml:"import_album"`
	ImportTimeout time.Duration `yaml:"import_timeout"`
	Live          LiveCfg       `yaml:"live"`
}

type ViewportCfg struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

type AnimatedCfg struct {
	Enabled    bool   `yaml:"enabled"`
	Format     string `yaml:"format"`
	DurationMS int    `yaml:"duration_ms"`
	FPS        int    `yaml:"fps"`
}

type LiveCfg struct {
	Enabled       bool          `yaml:"enabled"`
	PhotoFormat   string        `yaml:"photo_format"`
	CoverFrame    string        `yaml:"cover_frame"`
	Assemble      bool          `yaml:"assemble"`
	OutputDir     string        `yaml:"output_dir"`
	ImportPhotos  bool          `yaml:"import_photos"`
	ImportAlbum   string        `yaml:"import_album"`
	ImportTimeout time.Duration `yaml:"import_timeout"`
}

type rawConfig struct {
	Output OutputCfg    `yaml:"output"`
	AI     AICfg        `yaml:"ai"`
	Deck   DeckCfg      `yaml:"deck"`
	Render rawRenderCfg `yaml:"render"`
	XHS    XHSCfg       `yaml:"xhs"`
}

type rawRenderCfg struct {
	Viewport      ViewportCfg `yaml:"viewport"`
	Animated      AnimatedCfg `yaml:"animated"`
	ImportPhotos  bool        `yaml:"import_photos"`
	ImportAlbum   string      `yaml:"import_album"`
	ImportTimeout string      `yaml:"import_timeout"`
	Live          rawLiveCfg  `yaml:"live"`
}

type rawLiveCfg struct {
	Enabled       bool   `yaml:"enabled"`
	PhotoFormat   string `yaml:"photo_format"`
	CoverFrame    string `yaml:"cover_frame"`
	Assemble      bool   `yaml:"assemble"`
	OutputDir     string `yaml:"output_dir"`
	ImportPhotos  bool   `yaml:"import_photos"`
	ImportAlbum   string `yaml:"import_album"`
	ImportTimeout string `yaml:"import_timeout"`
}

type XHSCfg struct {
	Publish XHSPublishCfg `yaml:"publish"`
}

type XHSPublishCfg struct {
	Account          string                `yaml:"account"`
	Headless         *bool                 `yaml:"headless"`
	BrowserPath      string                `yaml:"browser_path"`
	ProfileDir       string                `yaml:"profile_dir"`
	Mode             string                `yaml:"mode"`
	DeclareOriginal  *bool                 `yaml:"declare_original"`
	AllowContentCopy *bool                 `yaml:"allow_content_copy"`
	ChromeArgs       []string              `yaml:"chrome_args"`
	TopicGeneration  XHSTopicGenerationCfg `yaml:"topic_generation"`
}

type XHSTopicGenerationCfg struct {
	Enabled *bool `yaml:"enabled"`
}

var DefaultXHSPublishChromeArgs = []string{
	"disable-background-networking",
	"disable-component-update",
	"no-first-run",
	"no-default-browser-check",
}

func validateXHSPublishMode(value string) error {
	switch strings.TrimSpace(value) {
	case "", "only-self", "schedule":
		return nil
	default:
		return fmt.Errorf("unsupported value %q", value)
	}
}

func parseConfigDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return parsed, nil
}

func normalizeRawConfig(raw rawConfig) (Config, error) {
	importTimeout, err := parseConfigDuration(raw.Render.ImportTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse render.import_timeout: %w", err)
	}
	liveImportTimeout, err := parseConfigDuration(raw.Render.Live.ImportTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse render.live.import_timeout: %w", err)
	}
	return Config{
		Output: raw.Output,
		AI:     raw.AI,
		Deck:   raw.Deck,
		Render: RenderCfg{
			Viewport:      raw.Render.Viewport,
			Animated:      raw.Render.Animated,
			ImportPhotos:  raw.Render.ImportPhotos,
			ImportAlbum:   raw.Render.ImportAlbum,
			ImportTimeout: importTimeout,
			Live: LiveCfg{
				Enabled:       raw.Render.Live.Enabled,
				PhotoFormat:   raw.Render.Live.PhotoFormat,
				CoverFrame:    raw.Render.Live.CoverFrame,
				Assemble:      raw.Render.Live.Assemble,
				OutputDir:     raw.Render.Live.OutputDir,
				ImportPhotos:  raw.Render.Live.ImportPhotos,
				ImportAlbum:   raw.Render.Live.ImportAlbum,
				ImportTimeout: liveImportTimeout,
			},
		},
		XHS: raw.XHS,
	}, nil
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg, err := normalizeRawConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Output.Dir == "" {
		cfg.Output.Dir = "output"
	}
	if cfg.AI.Command == "" {
		cfg.AI.Command = "ccs"
	}
	if len(cfg.AI.Args) == 0 {
		cfg.AI.Args = []string{"codex", "--bare"}
	}
	if cfg.Deck.Theme == "" {
		cfg.Deck.Theme = "default"
	}
	if cfg.Deck.Watermark.Enabled == nil {
		enabled := true
		cfg.Deck.Watermark.Enabled = &enabled
	}
	if cfg.Deck.Watermark.Text == "" {
		cfg.Deck.Watermark.Text = "walker1211/mark2note"
	}
	if cfg.Deck.Watermark.Position == "" {
		cfg.Deck.Watermark.Position = "bottom-right"
	}
	if cfg.Render.Viewport.Width == 0 {
		cfg.Render.Viewport.Width = 1242
	}
	if cfg.Render.Viewport.Height == 0 {
		cfg.Render.Viewport.Height = 1656
	}
	if cfg.Render.Animated.Format == "" {
		cfg.Render.Animated.Format = "webp"
	}
	if cfg.Render.Animated.DurationMS == 0 {
		cfg.Render.Animated.DurationMS = 2400
	}
	if cfg.Render.Animated.FPS == 0 {
		cfg.Render.Animated.FPS = 8
	}
	if cfg.Render.ImportTimeout == 0 {
		cfg.Render.ImportTimeout = 120 * time.Second
	}
	if cfg.Render.Live.PhotoFormat == "" {
		cfg.Render.Live.PhotoFormat = "jpeg"
	}
	if cfg.Render.Live.CoverFrame == "" {
		cfg.Render.Live.CoverFrame = "middle"
	}
	if cfg.Render.Live.ImportTimeout == 0 {
		cfg.Render.Live.ImportTimeout = 120 * time.Second
	}
	if cfg.XHS.Publish.Mode == "" {
		cfg.XHS.Publish.Mode = "only-self"
	}
	if err := validateXHSPublishMode(cfg.XHS.Publish.Mode); err != nil {
		return nil, fmt.Errorf("validate xhs.publish.mode: %w", err)
	}
	if cfg.XHS.Publish.DeclareOriginal == nil {
		enabled := true
		cfg.XHS.Publish.DeclareOriginal = &enabled
	}
	if cfg.XHS.Publish.AllowContentCopy == nil {
		enabled := false
		cfg.XHS.Publish.AllowContentCopy = &enabled
	}
	if cfg.XHS.Publish.ChromeArgs == nil {
		cfg.XHS.Publish.ChromeArgs = append([]string(nil), DefaultXHSPublishChromeArgs...)
	}
	if cfg.XHS.Publish.TopicGeneration.Enabled == nil {
		enabled := true
		cfg.XHS.Publish.TopicGeneration.Enabled = &enabled
	}
	return &cfg, nil
}
