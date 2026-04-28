package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestLoadAppliesDefaultDeckWatermark(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: custom-output\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Deck.Watermark.Enabled == nil {
		t.Fatalf("Watermark.Enabled unexpectedly nil")
	}
	if !*cfg.Deck.Watermark.Enabled {
		t.Fatalf("Watermark.Enabled = %v, want true", *cfg.Deck.Watermark.Enabled)
	}
	if cfg.Deck.Watermark.Text != "walker1211/mark2note" {
		t.Fatalf("Watermark.Text = %q, want %q", cfg.Deck.Watermark.Text, "walker1211/mark2note")
	}
	if cfg.Deck.Watermark.Position != "bottom-right" {
		t.Fatalf("Watermark.Position = %q, want %q", cfg.Deck.Watermark.Position, "bottom-right")
	}
}

func TestLoadKeepsExplicitDeckWatermark(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "deck:\n  watermark:\n    enabled: false\n    text: custom\n    position: bottom-left\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Deck.Watermark.Enabled == nil {
		t.Fatalf("Watermark.Enabled unexpectedly nil")
	}
	if *cfg.Deck.Watermark.Enabled {
		t.Fatalf("Watermark.Enabled = %v, want false", *cfg.Deck.Watermark.Enabled)
	}
	if cfg.Deck.Watermark.Text != "custom" {
		t.Fatalf("Watermark.Text = %q, want %q", cfg.Deck.Watermark.Text, "custom")
	}
	if cfg.Deck.Watermark.Position != "bottom-left" {
		t.Fatalf("Watermark.Position = %q, want %q", cfg.Deck.Watermark.Position, "bottom-left")
	}
}

func TestLoadAppliesDefaultRenderConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: custom-output\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Render.Viewport.Width != 1242 {
		t.Fatalf("Viewport.Width = %d, want 1242", cfg.Render.Viewport.Width)
	}
	if cfg.Render.Viewport.Height != 1656 {
		t.Fatalf("Viewport.Height = %d, want 1656", cfg.Render.Viewport.Height)
	}
	if cfg.Render.Animated.Enabled {
		t.Fatalf("Animated.Enabled = true, want false")
	}
	if cfg.Render.Animated.Format != "webp" {
		t.Fatalf("Animated.Format = %q, want webp", cfg.Render.Animated.Format)
	}
	if cfg.Render.Animated.DurationMS != 2400 {
		t.Fatalf("Animated.DurationMS = %d, want 2400", cfg.Render.Animated.DurationMS)
	}
	if cfg.Render.Animated.FPS != 8 {
		t.Fatalf("Animated.FPS = %d, want 8", cfg.Render.Animated.FPS)
	}
}

func TestLoadAppliesDefaultRenderImportConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: custom-output\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Render.ImportPhotos {
		t.Fatalf("Render.ImportPhotos = true, want false")
	}
	if cfg.Render.ImportAlbum != "" {
		t.Fatalf("Render.ImportAlbum = %q, want empty", cfg.Render.ImportAlbum)
	}
	if cfg.Render.ImportTimeout != 120*time.Second {
		t.Fatalf("Render.ImportTimeout = %v, want %v", cfg.Render.ImportTimeout, 120*time.Second)
	}
}

func TestLoadAppliesDefaultRenderLiveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: custom-output\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Render.Live.Enabled {
		t.Fatalf("Render.Live.Enabled = true, want false")
	}
	if cfg.Render.Live.PhotoFormat != "jpeg" {
		t.Fatalf("Render.Live.PhotoFormat = %q, want jpeg", cfg.Render.Live.PhotoFormat)
	}
	if cfg.Render.Live.CoverFrame != "middle" {
		t.Fatalf("Render.Live.CoverFrame = %q, want middle", cfg.Render.Live.CoverFrame)
	}
	if cfg.Render.Live.Assemble {
		t.Fatalf("Render.Live.Assemble = true, want false")
	}
	if cfg.Render.Live.OutputDir != "" {
		t.Fatalf("Render.Live.OutputDir = %q, want empty", cfg.Render.Live.OutputDir)
	}
	if cfg.Render.Live.ImportPhotos {
		t.Fatalf("Render.Live.ImportPhotos = true, want false")
	}
	if cfg.Render.Live.ImportAlbum != "" {
		t.Fatalf("Render.Live.ImportAlbum = %q, want empty", cfg.Render.Live.ImportAlbum)
	}
	if cfg.Render.Live.ImportTimeout != 120*time.Second {
		t.Fatalf("Render.Live.ImportTimeout = %v, want %v", cfg.Render.Live.ImportTimeout, 120*time.Second)
	}
}

func TestLoadAppliesDefaultRenderLiveFieldsWhenPartiallyConfigured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  live:\n    enabled: true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Render.Live.Enabled {
		t.Fatalf("Render.Live.Enabled = false, want true")
	}
	if cfg.Render.Live.PhotoFormat != "jpeg" {
		t.Fatalf("Render.Live.PhotoFormat = %q, want jpeg", cfg.Render.Live.PhotoFormat)
	}
	if cfg.Render.Live.CoverFrame != "middle" {
		t.Fatalf("Render.Live.CoverFrame = %q, want middle", cfg.Render.Live.CoverFrame)
	}
	if cfg.Render.Live.Assemble {
		t.Fatalf("Render.Live.Assemble = true, want false")
	}
	if cfg.Render.Live.OutputDir != "" {
		t.Fatalf("Render.Live.OutputDir = %q, want empty", cfg.Render.Live.OutputDir)
	}
}

func TestLoadRejectsNegativeRenderImportTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  import_timeout: -1s\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "render.import_timeout") {
		t.Fatalf("Load() error = %v, want render.import_timeout validation error", err)
	}
}

func TestLoadRejectsNegativeRenderLiveImportTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  live:\n    import_timeout: -1s\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "render.live.import_timeout") {
		t.Fatalf("Load() error = %v, want render.live.import_timeout validation error", err)
	}
}

func TestLoadRejectsZeroRenderImportTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  import_timeout: 0s\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "render.import_timeout") {
		t.Fatalf("Load() error = %v, want render.import_timeout validation error", err)
	}
}

func TestLoadRejectsZeroRenderLiveImportTimeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  live:\n    import_timeout: 0s\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "render.live.import_timeout") {
		t.Fatalf("Load() error = %v, want render.live.import_timeout validation error", err)
	}
}

func TestLoadKeepsExplicitRenderImportConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  import_photos: true\n  import_album: PNG 相册\n  import_timeout: 45s\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Render.ImportPhotos {
		t.Fatalf("Render.ImportPhotos = false, want true")
	}
	if cfg.Render.ImportAlbum != "PNG 相册" {
		t.Fatalf("Render.ImportAlbum = %q, want PNG 相册", cfg.Render.ImportAlbum)
	}
	if cfg.Render.ImportTimeout != 45*time.Second {
		t.Fatalf("Render.ImportTimeout = %v, want %v", cfg.Render.ImportTimeout, 45*time.Second)
	}
}

func TestLoadKeepsExplicitRenderLiveConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  live:\n    enabled: true\n    photo_format: webp\n    cover_frame: first\n    assemble: true\n    output_dir: exported-live\n    import_photos: true\n    import_album: Live 相册\n    import_timeout: 75s\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Render.Live.Enabled {
		t.Fatalf("Render.Live.Enabled = false, want true")
	}
	if cfg.Render.Live.PhotoFormat != "webp" {
		t.Fatalf("Render.Live.PhotoFormat = %q, want webp", cfg.Render.Live.PhotoFormat)
	}
	if cfg.Render.Live.CoverFrame != "first" {
		t.Fatalf("Render.Live.CoverFrame = %q, want first", cfg.Render.Live.CoverFrame)
	}
	if !cfg.Render.Live.Assemble {
		t.Fatalf("Render.Live.Assemble = false, want true")
	}
	if cfg.Render.Live.OutputDir != "exported-live" {
		t.Fatalf("Render.Live.OutputDir = %q, want exported-live", cfg.Render.Live.OutputDir)
	}
	if !cfg.Render.Live.ImportPhotos {
		t.Fatalf("Render.Live.ImportPhotos = false, want true")
	}
	if cfg.Render.Live.ImportAlbum != "Live 相册" {
		t.Fatalf("Render.Live.ImportAlbum = %q, want Live 相册", cfg.Render.Live.ImportAlbum)
	}
	if cfg.Render.Live.ImportTimeout != 75*time.Second {
		t.Fatalf("Render.Live.ImportTimeout = %v, want %v", cfg.Render.Live.ImportTimeout, 75*time.Second)
	}
}

func TestLoadKeepsExplicitRenderConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "render:\n  viewport:\n    width: 720\n    height: 960\n  animated:\n    enabled: true\n    format: webp\n    duration_ms: 3200\n    fps: 10\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Render.Viewport.Width != 720 {
		t.Fatalf("Viewport.Width = %d, want 720", cfg.Render.Viewport.Width)
	}
	if cfg.Render.Viewport.Height != 960 {
		t.Fatalf("Viewport.Height = %d, want 960", cfg.Render.Viewport.Height)
	}
	if !cfg.Render.Animated.Enabled {
		t.Fatalf("Animated.Enabled = false, want true")
	}
	if cfg.Render.Animated.Format != "webp" {
		t.Fatalf("Animated.Format = %q, want webp", cfg.Render.Animated.Format)
	}
	if cfg.Render.Animated.DurationMS != 3200 {
		t.Fatalf("Animated.DurationMS = %d, want 3200", cfg.Render.Animated.DurationMS)
	}
	if cfg.Render.Animated.FPS != 10 {
		t.Fatalf("Animated.FPS = %d, want 10", cfg.Render.Animated.FPS)
	}
}

func TestLoadAppliesDefaultXHSPublishConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: out\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.XHS.Publish.Account != "" {
		t.Fatalf("Account = %q, want empty", cfg.XHS.Publish.Account)
	}
	if cfg.XHS.Publish.Headless != nil {
		t.Fatalf("Headless = %v, want nil", cfg.XHS.Publish.Headless)
	}
	if cfg.XHS.Publish.BrowserPath != "" {
		t.Fatalf("BrowserPath = %q, want empty", cfg.XHS.Publish.BrowserPath)
	}
	if cfg.XHS.Publish.ProfileDir != "" {
		t.Fatalf("ProfileDir = %q, want empty", cfg.XHS.Publish.ProfileDir)
	}
	if cfg.XHS.Publish.Mode != "only-self" {
		t.Fatalf("Mode = %q, want only-self", cfg.XHS.Publish.Mode)
	}
	if !reflect.DeepEqual(cfg.XHS.Publish.ChromeArgs, DefaultXHSPublishChromeArgs) {
		t.Fatalf("ChromeArgs = %#v, want %#v", cfg.XHS.Publish.ChromeArgs, DefaultXHSPublishChromeArgs)
	}
}

func TestLoadKeepsExplicitXHSPublishConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "xhs:\n  publish:\n    account: walker\n    headless: false\n    browser_path: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome\n    profile_dir: ~/.config/mark2note/xhs/profiles/walker\n    mode: schedule\n    chrome_args:\n      - --no-first-run\n      - proxy-server=http://127.0.0.1:8080\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.XHS.Publish.Account != "walker" {
		t.Fatalf("Account = %q, want walker", cfg.XHS.Publish.Account)
	}
	if cfg.XHS.Publish.Headless == nil || *cfg.XHS.Publish.Headless {
		t.Fatalf("Headless = %#v, want false", cfg.XHS.Publish.Headless)
	}
	if cfg.XHS.Publish.BrowserPath != "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" {
		t.Fatalf("BrowserPath = %q", cfg.XHS.Publish.BrowserPath)
	}
	if cfg.XHS.Publish.ProfileDir != "~/.config/mark2note/xhs/profiles/walker" {
		t.Fatalf("ProfileDir = %q", cfg.XHS.Publish.ProfileDir)
	}
	if cfg.XHS.Publish.Mode != "schedule" {
		t.Fatalf("Mode = %q, want schedule", cfg.XHS.Publish.Mode)
	}
	wantChromeArgs := []string{"--no-first-run", "proxy-server=http://127.0.0.1:8080"}
	if !reflect.DeepEqual(cfg.XHS.Publish.ChromeArgs, wantChromeArgs) {
		t.Fatalf("ChromeArgs = %#v, want %#v", cfg.XHS.Publish.ChromeArgs, wantChromeArgs)
	}
}

func TestLoadKeepsExplicitEmptyXHSPublishChromeArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "xhs:\n  publish:\n    chrome_args: []\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.XHS.Publish.ChromeArgs == nil || len(cfg.XHS.Publish.ChromeArgs) != 0 {
		t.Fatalf("ChromeArgs = %#v, want explicit empty slice", cfg.XHS.Publish.ChromeArgs)
	}
}

func TestLoadRejectsInvalidXHSPublishMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("xhs:\n  publish:\n    mode: publish-now\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "validate xhs.publish.mode") {
		t.Fatalf("Load() error = %v, want mode validation error", err)
	}
}

func TestLoadAppliesDefaultXHSPublishOriginalityConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("output:\n  dir: out\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.XHS.Publish.DeclareOriginal == nil || !*cfg.XHS.Publish.DeclareOriginal {
		t.Fatalf("DeclareOriginal = %#v, want true", cfg.XHS.Publish.DeclareOriginal)
	}
	if cfg.XHS.Publish.AllowContentCopy == nil || *cfg.XHS.Publish.AllowContentCopy {
		t.Fatalf("AllowContentCopy = %#v, want false", cfg.XHS.Publish.AllowContentCopy)
	}
}

func TestLoadKeepsExplicitXHSPublishOriginalityConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "xhs:\n  publish:\n    declare_original: false\n    allow_content_copy: true\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.XHS.Publish.DeclareOriginal == nil || *cfg.XHS.Publish.DeclareOriginal {
		t.Fatalf("DeclareOriginal = %#v, want false", cfg.XHS.Publish.DeclareOriginal)
	}
	if cfg.XHS.Publish.AllowContentCopy == nil || !*cfg.XHS.Publish.AllowContentCopy {
		t.Fatalf("AllowContentCopy = %#v, want true", cfg.XHS.Publish.AllowContentCopy)
	}
}
