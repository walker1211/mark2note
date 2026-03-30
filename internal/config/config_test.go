package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadAppliesDefaultAIConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: custom-output\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Output.Dir != "custom-output" {
		t.Fatalf("Output.Dir = %q, want %q", cfg.Output.Dir, "custom-output")
	}
	if cfg.AI.Command != "ccs" {
		t.Fatalf("AI.Command = %q, want %q", cfg.AI.Command, "ccs")
	}
	if !reflect.DeepEqual(cfg.AI.Args, []string{"codex", "--bare"}) {
		t.Fatalf("AI.Args = %v, want %v", cfg.AI.Args, []string{"codex", "--bare"})
	}
}

func TestLoadAppliesDefaultDeckConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: custom-output\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Deck.Theme != "default" {
		t.Fatalf("Deck.Theme = %q, want %q", cfg.Deck.Theme, "default")
	}
	if cfg.Deck.Author != "" {
		t.Fatalf("Deck.Author = %q, want empty", cfg.Deck.Author)
	}
}

func TestLoadKeepsExplicitDeckThemeAndAuthor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "deck:\n  theme: warm-paper\n  author: 默认作者\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Deck.Theme != "warm-paper" || cfg.Deck.Author != "默认作者" {
		t.Fatalf("cfg.Deck = %#v", cfg.Deck)
	}
}
