package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Output OutputCfg `yaml:"output"`
	AI     AICfg     `yaml:"ai"`
	Deck   DeckCfg   `yaml:"deck"`
	Render RenderCfg `yaml:"render"`
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
	Viewport ViewportCfg `yaml:"viewport"`
	Animated AnimatedCfg `yaml:"animated"`
	Live     LiveCfg     `yaml:"live"`
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
	Enabled     bool   `yaml:"enabled"`
	PhotoFormat string `yaml:"photo_format"`
	CoverFrame  string `yaml:"cover_frame"`
	Assemble    bool   `yaml:"assemble"`
	OutputDir   string `yaml:"output_dir"`
}

func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
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
	if cfg.Render.Live.PhotoFormat == "" {
		cfg.Render.Live.PhotoFormat = "jpeg"
	}
	if cfg.Render.Live.CoverFrame == "" {
		cfg.Render.Live.CoverFrame = "middle"
	}
	return &cfg, nil
}
