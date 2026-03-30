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
	Theme  string `yaml:"theme"`
	Author string `yaml:"author"`
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
	return &cfg, nil
}
