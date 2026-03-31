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
	return &cfg, nil
}
