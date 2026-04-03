package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/walker1211/mark2note/internal/app"
	"github.com/walker1211/mark2note/internal/config"
)

func TestDefaultOptions(t *testing.T) {
	opts := defaultOptions()
	if opts.OutDir != "output" {
		t.Fatalf("OutDir = %q, want %q", opts.OutDir, "output")
	}
	if opts.ConfigPath != "configs/config.yaml" {
		t.Fatalf("ConfigPath = %q, want %q", opts.ConfigPath, "configs/config.yaml")
	}
	if opts.Jobs != 2 {
		t.Fatalf("Jobs = %d, want %d", opts.Jobs, 2)
	}
}

func TestUsageTextMentionsConfiguredDefaultOutputDir(t *testing.T) {
	for _, want := range []string{"<output.dir>/<markdown-file-name>-<timestamp>", "article-20260328-153000"} {
		if !strings.Contains(usageText(), want) {
			t.Fatalf("usageText() missing %q", want)
		}
	}
}

func TestUsageTextMentionsThemeAndAuthorFlags(t *testing.T) {
	text := usageText()
	for _, want := range []string{"--theme <name>", "--author <name>", "deck.theme", "deck.author", "default / warm-paper / editorial-cool / lifestyle-light / tech-noir / editorial-mono", "one-off deck theme override", "one-off cover author input (blank falls back to deck.author)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usageText() missing %q", want)
		}
	}
}

func TestUsageTextMentionsDefaultConfigPath(t *testing.T) {
	text := usageText()
	for _, want := range []string{"configs/config.yaml", "--config ./configs/config.yaml", "--config ./config.yaml"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usageText() missing %q", want)
		}
	}
}

func TestIsHelpRequest(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "help command", args: []string{"help"}, want: true},
		{name: "short flag", args: []string{"-h"}, want: false},
		{name: "long flag", args: []string{"--help"}, want: false},
		{name: "mixed args", args: []string{"help", "--input", "article.md"}, want: false},
		{name: "empty", args: nil, want: false},
	}
	for _, tt := range tests {
		if got := isHelpRequest(tt.args); got != tt.want {
			t.Fatalf("%s: isHelpRequest(%v) = %v, want %v", tt.name, tt.args, got, tt.want)
		}
	}
}

func TestParseOptionsUsesConfigsConfigYAMLByDefault(t *testing.T) {
	opts, err := parseOptions([]string{"--input", "article.md"})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if opts.ConfigPath != "configs/config.yaml" {
		t.Fatalf("ConfigPath = %q, want %q", opts.ConfigPath, "configs/config.yaml")
	}
}

func TestParseOptionsOverridesFlags(t *testing.T) {
	opts, err := parseOptions([]string{"--out", "tmp/out", "--chrome", "/tmp/chrome", "--jobs", "3", "--input", "article.md", "--config", "custom.yaml"})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if opts.OutDir != "tmp/out" {
		t.Fatalf("OutDir = %q, want %q", opts.OutDir, "tmp/out")
	}
	if opts.ChromePath != "/tmp/chrome" {
		t.Fatalf("ChromePath = %q, want %q", opts.ChromePath, "/tmp/chrome")
	}
	if opts.InputPath != "article.md" {
		t.Fatalf("InputPath = %q, want %q", opts.InputPath, "article.md")
	}
	if opts.ConfigPath != "custom.yaml" {
		t.Fatalf("ConfigPath = %q, want %q", opts.ConfigPath, "custom.yaml")
	}
	if opts.Jobs != 3 {
		t.Fatalf("Jobs = %d, want %d", opts.Jobs, 3)
	}
}

func TestParseOptionsParsesThemeAndAuthor(t *testing.T) {
	opts, err := parseOptions([]string{"--input", "article.md", "--theme", "warm-paper", "--author", "搁剑听风"})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if opts.Theme != "warm-paper" || opts.Author != "搁剑听风" {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestRunPassesThemeAuthorAndDefaultConfigToService(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	var got Options
	generatePreview = func(opts Options) (app.Result, error) {
		got = opts
		return app.Result{PageCount: 3, OutDir: t.TempDir()}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--theme", "warm-paper", "--author", "搁剑听风"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if got.Theme != "warm-paper" || got.Author != "搁剑听风" {
		t.Fatalf("opts = %#v", got)
	}
	if got.ConfigPath != "configs/config.yaml" {
		t.Fatalf("ConfigPath = %q, want %q", got.ConfigPath, "configs/config.yaml")
	}
}

func TestRunExplicitConfigStillWins(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	var got Options
	generatePreview = func(opts Options) (app.Result, error) {
		got = opts
		return app.Result{PageCount: 1, OutDir: t.TempDir()}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--config", "./config.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if got.ConfigPath != "./config.yaml" {
		t.Fatalf("ConfigPath = %q, want %q", got.ConfigPath, "./config.yaml")
	}
}

func TestRunDoesNotImplicitlyFallbackToRootConfigYAML(t *testing.T) {
	originalLoadConfig := loadConfig
	originalReadFile := readFile
	originalBuildDeckJSON := buildDeckJSON
	originalGeneratePreview := generatePreview
	defer func() {
		loadConfig = originalLoadConfig
		readFile = originalReadFile
		buildDeckJSON = originalBuildDeckJSON
		generatePreview = originalGeneratePreview
	}()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte("output:\n  dir: fallback\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "article.md"), []byte("# title\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	loadConfig = originalLoadConfig
	readFile = func(path string) ([]byte, error) {
		t.Fatalf("ReadFile() should not be called when config loading fails, got %q", path)
		return nil, nil
	}
	buildDeckJSON = func(cfg *config.Config, markdown string) (string, error) {
		t.Fatalf("BuildDeckJSON() should not be called when config loading fails")
		return "", nil
	}
	generatePreview = originalGeneratePreview

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "error loading config") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), filepath.Join("configs", "config.yaml")) {
		t.Fatalf("stderr should mention default config path, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "open config.yaml:") {
		t.Fatalf("stderr suggests root config fallback: %q", stderr.String())
	}
}

func TestRunCaptureHTMLPassesOptionsToRenderer(t *testing.T) {
	originalCaptureHTML := captureHTML
	defer func() { captureHTML = originalCaptureHTML }()

	var got Options
	captureHTML = func(opts Options) error {
		got = opts
		return nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"capture-html", "--input", "preview", "--chrome", "/tmp/chrome", "--jobs", "3"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	want := Options{OutDir: "output", ChromePath: "/tmp/chrome", Jobs: 3, InputPath: "preview", ConfigPath: "configs/config.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("opts = %#v, want %#v", got, want)
	}
	if stdout.String() != "captured html to png\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCaptureHTMLPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"capture-html", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if stdout.Len() == 0 {
		t.Fatalf("stdout is empty")
	}
	if !strings.Contains(stdout.String(), "capture-html --input <path>") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunCaptureHTMLRequiresInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"capture-html"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--input is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCaptureHTMLRejectsInvalidJobs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"capture-html", "--input", "preview", "--jobs", "0"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "jobs must be >= 1") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunCaptureHTMLMapsRendererError(t *testing.T) {
	originalCaptureHTML := captureHTML
	defer func() { captureHTML = originalCaptureHTML }()

	captureHTML = func(Options) error {
		return fmt.Errorf("capture html path preview: no .html files found")
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"capture-html", "--input", "preview"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "capture html failed: capture html path preview: no .html files found\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseOptionsRequiresInput(t *testing.T) {
	_, err := parseOptions(nil)
	if err == nil {
		t.Fatalf("parseOptions() error = nil, want missing input error")
	}
	if !strings.Contains(err.Error(), "--input is required") {
		t.Fatalf("parseOptions() error = %q", err)
	}
	if !strings.Contains(err.Error(), usageText()) {
		t.Fatalf("parseOptions() error missing usage text: %q", err)
	}
}

func TestParseOptionsRejectsInvalidJobs(t *testing.T) {
	if _, err := parseOptions([]string{"--input", "article.md", "--jobs", "0"}); err == nil {
		t.Fatalf("parseOptions() error = nil, want error for jobs <= 0")
	}
}

func TestParseOptionsRejectsUnexpectedPositionalArgs(t *testing.T) {
	if _, err := parseOptions([]string{"--input", "article.md", "extra"}); err == nil {
		t.Fatalf("parseOptions() error = nil, want error for positional args")
	}
}

func TestParseOptionsMarksOutDirChangedForEqualsSyntax(t *testing.T) {
	opts, err := parseOptions([]string{"--input", "article.md", "--out=/tmp/out"})
	if err != nil {
		t.Fatalf("parseOptions() error = %v", err)
	}
	if !opts.OutDirChanged {
		t.Fatalf("OutDirChanged = false, want true")
	}
}

func TestBuildRendererUsesAbsoluteOutDir(t *testing.T) {
	opts := Options{
		OutDir:     "output/mark2note",
		ChromePath: "/tmp/chrome",
		Jobs:       2,
	}

	r := buildRenderer(opts)

	if !filepath.IsAbs(r.OutDir) {
		t.Fatalf("renderer out dir should be absolute, got %q", r.OutDir)
	}
	if r.ChromePath != opts.ChromePath {
		t.Fatalf("ChromePath = %q, want %q", r.ChromePath, opts.ChromePath)
	}
	if r.Jobs != opts.Jobs {
		t.Fatalf("Jobs = %d, want %d", r.Jobs, opts.Jobs)
	}
}

func TestAbsolutePathResolvesRelativePath(t *testing.T) {
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}

	got := absolutePath("output/preview")
	if !filepath.IsAbs(got) {
		t.Fatalf("absolutePath() should return absolute path, got %q", got)
	}
	if !strings.HasSuffix(got, string(filepath.Separator)+filepath.Join("output", "preview")) {
		t.Fatalf("absolutePath() = %q, want suffix %q", got, filepath.Join("output", "preview"))
	}
}

func TestRunPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if stdout.String() != usageText()+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunPrintsHelpForMixedHelpArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help", "--input", "article.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if stdout.String() != usageText()+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunRequiresInputWhenArgsEmpty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--input is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), usageText()) {
		t.Fatalf("stderr missing usage text: %q", stderr.String())
	}
}

func TestRunPrintsGeneratedPagesFromServiceResult(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{PageCount: 3, OutDir: t.TempDir()}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "generated 3 preview pages") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunMapsServiceLoadConfigError(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: missing config", app.ErrLoadConfig)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "error loading config: missing config\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunMapsServiceReadMarkdownError(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: open article.md", app.ErrReadMarkdown)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stderr.String() != "error reading markdown: open article.md\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunMapsServiceBuildDeckJSONError(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: bad ai output", app.ErrBuildDeckJSON)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stderr.String() != "error building deck json: bad ai output\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunMapsServiceRenderFailure(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: deck invalid", app.ErrParseDeck)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stderr.String() != "render preview failed: deck invalid\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunStripsNestedParseDeckPrefix(t *testing.T) {
	originalGeneratePreview := generatePreview
	defer func() { generatePreview = originalGeneratePreview }()

	generatePreview = func(Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: parse deck: deck must contain 3 to 12 pages", app.ErrParseDeck)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--out", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if stderr.String() != "render preview failed: deck must contain 3 to 12 pages\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
