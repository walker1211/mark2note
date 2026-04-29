package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/walker1211/mark2note/internal/app"
	"github.com/walker1211/mark2note/internal/config"
	"github.com/walker1211/mark2note/internal/xhs"
)

func TestUsageTextMentionsPublishXHSCommand(t *testing.T) {
	text := usageText()
	for _, want := range []string{"publish-xhs", "mark2note publish-xhs --account <name>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("usageText() missing %q", want)
		}
	}
}

func TestPublishXHSUsageTextMentionsConfigDefaults(t *testing.T) {
	text := publishXHSUsageText()
	for _, want := range []string{"--config <file>", "default from xhs.publish.account", "default from xhs.publish.mode", "default from xhs.publish.browser_path", "default from xhs.publish.profile_dir"} {
		if !strings.Contains(text, want) {
			t.Fatalf("publishXHSUsageText() missing %q", want)
		}
	}
}

func TestParsePublishXHSOptionsTracksConfigBackedFlagPresence(t *testing.T) {
	opts, err := parsePublishXHSOptions([]string{"--config", "alt.yaml", "--account", "writer", "--headless=false", "--profile-dir", "/tmp/xhs", "--mode", "schedule", "--title", "标题", "--content", "正文", "--images", "cover.png"})
	if err != nil {
		t.Fatalf("parsePublishXHSOptions() error = %v", err)
	}
	if opts.ConfigPath != "alt.yaml" || !opts.ConfigPathChanged {
		t.Fatalf("ConfigPath = %#v", opts)
	}
	if !opts.AccountChanged || !opts.HeadlessChanged || !opts.ProfileDirChanged || !opts.ModeChanged {
		t.Fatalf("changed flags = %#v", opts)
	}
}

func TestParsePublishXHSOptionsUsesDefaultConfigPath(t *testing.T) {
	opts, err := parsePublishXHSOptions([]string{"--title", "标题", "--content", "正文", "--images", "cover.png"})
	if err != nil {
		t.Fatalf("parsePublishXHSOptions() error = %v", err)
	}
	if opts.ConfigPath != "configs/config.yaml" || opts.ConfigPathChanged {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestParsePublishXHSOptionsTracksOriginalityFlagPresence(t *testing.T) {
	opts, err := parsePublishXHSOptions([]string{"--declare-original=false", "--allow-content-copy=true", "--title", "标题", "--content", "正文", "--images", "cover.png"})
	if err != nil {
		t.Fatalf("parsePublishXHSOptions() error = %v", err)
	}
	if !opts.DeclareOriginalChanged || !opts.AllowContentCopyChanged {
		t.Fatalf("changed flags = %#v", opts)
	}
	if opts.DeclareOriginal != false || opts.AllowContentCopy != true {
		t.Fatalf("opts = %#v", opts)
	}
}

func TestRunAutoPublishXHSWritesMetadataBeforePublishing(t *testing.T) {
	originalPublishXHS := publishXHS
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	originalNowFunc := nowFunc
	defer func() {
		publishXHS = originalPublishXHS
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
		nowFunc = originalNowFunc
	}()

	outDir := t.TempDir()
	imagePath := filepath.Join(outDir, "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", imagePath, err)
	}
	stubAutoPublishMetadataInputs(t, false)
	nowFunc = func() time.Time {
		return time.Date(2026, 4, 29, 10, 11, 12, 0, time.UTC)
	}

	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		metaPath := filepath.Join(outDir, xhsPublishMetaFilename)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("metadata not written before publishXHS: %v", err)
		}
		var meta xhsPublishMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			t.Fatalf("Unmarshal(%q) error = %v", metaPath, err)
		}
		if meta.Title != opts.Title || meta.Account != "walker" || meta.Mode != string(xhs.PublishModeOnlySelf) {
			t.Fatalf("metadata basics = %#v", meta)
		}
		if meta.Content != opts.Content || !reflect.DeepEqual(meta.Tags, opts.Tags) || !reflect.DeepEqual(meta.Images, opts.ImagePaths) {
			t.Fatalf("metadata publish content = %#v, opts = %#v", meta, opts)
		}
		if meta.ChromePath != "/tmp/publish-chrome" || meta.Headless || meta.ProfileDir != "/tmp/xhs-profile" {
			t.Fatalf("metadata browser fields = %#v", meta)
		}
		if !meta.DeclareOriginal || meta.AllowContentCopy {
			t.Fatalf("metadata originality fields = %#v", meta)
		}
		if meta.InputPath != "article.md" || meta.ConfigPath != "configs/config.yaml" || meta.GeneratedAt != "2026-04-29T10:11:12Z" {
			t.Fatalf("metadata source fields = %#v", meta)
		}
		if !reflect.DeepEqual(meta.ChromeArgs, []string{"no-first-run"}) {
			t.Fatalf("metadata chrome_args = %#v", meta.ChromeArgs)
		}
		return app.PublishResult{
			Request: xhs.PublishRequest{Account: opts.Account, MediaKind: xhs.MediaKindStandard},
			Result:  xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := runAutoPublishXHS(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{OutDir: outDir, ImagePaths: []string{imagePath}}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runAutoPublishXHS() = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunAutoPublishXHSRejectsMetadataWriteFailureBeforePublishing(t *testing.T) {
	originalPublishXHS := publishXHS
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	defer func() {
		publishXHS = originalPublishXHS
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
	}()

	baseDir := t.TempDir()
	imagePath := filepath.Join(baseDir, "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", imagePath, err)
	}
	stubAutoPublishMetadataInputs(t, false)
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		t.Fatal("publishXHS called after metadata write failure")
		return app.PublishResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	missingOutDir := filepath.Join(baseDir, "missing")
	code := runAutoPublishXHS(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{OutDir: missingOutDir, ImagePaths: []string{imagePath}}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runAutoPublishXHS() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "auto publish xhs failed: write xhs publish metadata") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func stubAutoPublishMetadataInputs(t *testing.T, titleTooLong bool) {
	t.Helper()
	readFile = func(path string) ([]byte, error) {
		if titleTooLong {
			return []byte("# 自动发布标题自动发布标题自动发布标题\n\n正文"), nil
		}
		return []byte("# 自动发布标题\n\n正文"), nil
	}
	headless := false
	declareOriginal := true
	allowContentCopy := false
	topicGenerationEnabled := true
	loadConfig = func(path string) (*config.Config, error) {
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:          "walker",
			Headless:         &headless,
			BrowserPath:      "/tmp/publish-chrome",
			ProfileDir:       "/tmp/xhs-profile",
			Mode:             string(xhs.PublishModeOnlySelf),
			DeclareOriginal:  &declareOriginal,
			AllowContentCopy: &allowContentCopy,
			ChromeArgs:       []string{"no-first-run"},
			TopicGeneration:  config.XHSTopicGenerationCfg{Enabled: &topicGenerationEnabled},
		}}}, nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		return []string{"AI代理", "工程反思"}, nil
	}
	buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
		t.Fatal("buildPublishTitle called for title within limit")
		return "", nil
	}
}

func TestRunAutoPublishXHSPublishesGeneratedPNGs(t *testing.T) {
	originalGeneratePreview := generatePreview
	originalPublishXHS := publishXHS
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	defer func() {
		generatePreview = originalGeneratePreview
		publishXHS = originalPublishXHS
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
	}()

	outDir := t.TempDir()
	imagePaths := []string{filepath.Join(outDir, "p01-cover.png"), filepath.Join(outDir, "p02-bullets.png")}
	for _, imagePath := range imagePaths {
		if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", imagePath, err)
		}
	}
	generatePreview = func(opts Options) (app.Result, error) {
		if !opts.PublishXHS {
			t.Fatalf("PublishXHS = false, want true")
		}
		return app.Result{PageCount: 2, OutDir: outDir, ImagePaths: imagePaths}, nil
	}
	readFile = func(path string) ([]byte, error) {
		if path != "article.md" {
			t.Fatalf("ReadFile path = %q, want article.md", path)
		}
		return []byte("# 一个AI代理删库之后我开始关心刹车\n\n正文"), nil
	}
	headless := false
	declareOriginal := true
	allowContentCopy := false
	topicGenerationEnabled := true
	loadConfig = func(path string) (*config.Config, error) {
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:          "walker",
			Headless:         &headless,
			BrowserPath:      "/tmp/publish-chrome",
			ProfileDir:       "/tmp/xhs-profile",
			Mode:             string(xhs.PublishModeOnlySelf),
			DeclareOriginal:  &declareOriginal,
			AllowContentCopy: &allowContentCopy,
			ChromeArgs:       []string{"no-first-run", "no-default-browser-check"},
			TopicGeneration:  config.XHSTopicGenerationCfg{Enabled: &topicGenerationEnabled},
		}}}, nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		return []string{"AI代理", "数据安全", "工程反思"}, nil
	}
	buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
		t.Fatal("buildPublishTitle called for title within limit")
		return "", nil
	}

	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		return app.PublishResult{
			Request: xhs.PublishRequest{Account: opts.Account, MediaKind: xhs.MediaKindStandard},
			Result:  xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--publish-xhs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if got.Account != "walker" || got.Headless || got.ChromePath != "/tmp/publish-chrome" || got.ProfileDir != "/tmp/xhs-profile" {
		t.Fatalf("publish defaults not merged: %#v", got)
	}
	if !reflect.DeepEqual(got.ChromeArgs, []string{"no-first-run", "no-default-browser-check"}) {
		t.Fatalf("ChromeArgs = %#v", got.ChromeArgs)
	}
	if got.Title != "一个AI代理删库之后我开始关心刹车" {
		t.Fatalf("Title = %q", got.Title)
	}
	if got.Content != "" {
		t.Fatalf("Content = %q, want empty", got.Content)
	}
	wantTags := []string{"AI代理", "数据安全", "工程反思"}
	if !reflect.DeepEqual(got.Tags, wantTags) {
		t.Fatalf("Tags = %#v, want %#v", got.Tags, wantTags)
	}
	if !reflect.DeepEqual(got.ImagePaths, imagePaths) {
		t.Fatalf("ImagePaths = %#v, want %#v", got.ImagePaths, imagePaths)
	}
	if !got.DeclareOriginal || got.AllowContentCopy {
		t.Fatalf("originality flags = declare:%v copy:%v", got.DeclareOriginal, got.AllowContentCopy)
	}
	for _, want := range []string{"generated 2 preview pages", "xiaohongshu only-self-visible published"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, missing %q", stdout.String(), want)
		}
	}
}

func TestRunAutoPublishXHSRewritesLongTitleWithAI(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		topicGenerationEnabled := true
		titleGenerationEnabled := true
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:         "walker",
			TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &topicGenerationEnabled},
			TitleGeneration: config.XHSTitleGenerationCfg{Enabled: &titleGenerationEnabled, MaxRunes: 20},
		}}}, nil
	}
	buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
		if maxRunes != 20 {
			t.Fatalf("maxRunes = %d, want 20", maxRunes)
		}
		if title != "别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始" {
			t.Fatalf("title passed to AI = %q", title)
		}
		return "别只用AI写Demo，要进真实开源", nil
	}
	var topicTitle string
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		topicTitle = title
		return []string{"AI编程", "开源项目", "工程实践"}, nil
	}

	got, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
	if err != nil {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v", err)
	}
	if got.Title != "别只用AI写Demo，要进真实开源" {
		t.Fatalf("Title = %q", got.Title)
	}
	if topicTitle != got.Title {
		t.Fatalf("topic title = %q, want final title %q", topicTitle, got.Title)
	}
}

func TestRunAutoPublishXHSRejectsLongTitleWhenTitleGenerationDisabled(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		topicGenerationEnabled := true
		titleGenerationEnabled := false
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:         "walker",
			TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &topicGenerationEnabled},
			TitleGeneration: config.XHSTitleGenerationCfg{Enabled: &titleGenerationEnabled, MaxRunes: 20},
		}}}, nil
	}
	buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
		t.Fatal("buildPublishTitle called while title generation disabled")
		return "", nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		t.Fatal("buildPublishTopics called after long title rejection")
		return nil, nil
	}

	_, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
	if err == nil || !strings.Contains(err.Error(), "title exceeds 20 characters") {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v, want long title error", err)
	}
}

func TestRunAutoPublishXHSRejectsAITitleGenerationFailure(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		topicGenerationEnabled := true
		titleGenerationEnabled := true
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:         "walker",
			TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &topicGenerationEnabled},
			TitleGeneration: config.XHSTitleGenerationCfg{Enabled: &titleGenerationEnabled, MaxRunes: 20},
		}}}, nil
	}
	buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
		return "", fmt.Errorf("ai failed")
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		t.Fatal("buildPublishTopics called after title generation failure")
		return nil, nil
	}

	_, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
	if err == nil || !strings.Contains(err.Error(), "generate xhs publish title") {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v, want AI title generation error", err)
	}
}

func TestRunAutoPublishXHSRejectsInvalidAITitle(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	originalBuildPublishTitle := buildPublishTitle
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
		buildPublishTitle = originalBuildPublishTitle
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		topicGenerationEnabled := true
		titleGenerationEnabled := true
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:         "walker",
			TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &topicGenerationEnabled},
			TitleGeneration: config.XHSTitleGenerationCfg{Enabled: &titleGenerationEnabled, MaxRunes: 20},
		}}}, nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		t.Fatal("buildPublishTopics called after invalid title")
		return nil, nil
	}

	cases := []struct {
		name  string
		title string
		want  string
	}{
		{name: "empty", title: " ", want: "empty title returned"},
		{name: "too long", title: "别只用 AI 写 Demo：把代码合进真实开源项目", want: "title still exceeds 20 characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buildPublishTitle = func(cfg *config.Config, markdown string, title string, maxRunes int) (string, error) {
				return tc.title, nil
			}
			_, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("buildAutoPublishXHSOptions() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRunAutoPublishXHSUsesManualTagsWithoutTopicGeneration(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 标题\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		enabled := false
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{Account: "walker", TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &enabled}}}}, nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		t.Fatal("buildPublishTopics called with manual tags")
		return nil, nil
	}

	got, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml", XHSTags: []string{"AI代理", "数据安全"}}, app.Result{ImagePaths: []string{imagePath}})
	if err != nil {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v", err)
	}
	want := []string{"AI代理", "数据安全"}
	if !reflect.DeepEqual(got.Tags, want) || got.Content != "" {
		t.Fatalf("publish topics = %#v content=%q", got.Tags, got.Content)
	}
}

func TestRunAutoPublishXHSRejectsDisabledTopicGenerationWithoutManualTags(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 标题\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		enabled := false
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{Account: "walker", TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &enabled}}}}, nil
	}

	_, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
	if err == nil || !strings.Contains(err.Error(), "topic generation is disabled") {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v, want disabled topic generation error", err)
	}
}

func TestRunAutoPublishXHSRejectsAITopicGenerationFailure(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 标题\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		enabled := true
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{Account: "walker", TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &enabled}}}}, nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		return nil, fmt.Errorf("ai failed")
	}

	_, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
	if err == nil || !strings.Contains(err.Error(), "generate xhs publish topics") {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v, want AI topic generation error", err)
	}
}

func TestRunAutoPublishXHSRejectsEmptyAITopics(t *testing.T) {
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	originalBuildPublishTopics := buildPublishTopics
	defer func() {
		readFile = originalReadFile
		loadConfig = originalLoadConfig
		buildPublishTopics = originalBuildPublishTopics
	}()

	imagePath := filepath.Join(t.TempDir(), "p01-cover.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	readFile = func(path string) ([]byte, error) {
		return []byte("# 标题\n\n正文"), nil
	}
	loadConfig = func(path string) (*config.Config, error) {
		enabled := true
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{Account: "walker", TopicGeneration: config.XHSTopicGenerationCfg{Enabled: &enabled}}}}, nil
	}
	buildPublishTopics = func(cfg *config.Config, markdown string, title string) ([]string, error) {
		return []string{"1061", "#"}, nil
	}

	_, err := buildAutoPublishXHSOptions(Options{InputPath: "article.md", ConfigPath: "configs/config.yaml"}, app.Result{ImagePaths: []string{imagePath}})
	if err == nil || !strings.Contains(err.Error(), "no valid topics") {
		t.Fatalf("buildAutoPublishXHSOptions() error = %v, want empty AI topics error", err)
	}
}

func TestRunAutoPublishXHSSkipsPublishWhenRenderFails(t *testing.T) {
	originalGeneratePreview := generatePreview
	originalPublishXHS := publishXHS
	defer func() {
		generatePreview = originalGeneratePreview
		publishXHS = originalPublishXHS
	}()

	generatePreview = func(opts Options) (app.Result, error) {
		return app.Result{}, fmt.Errorf("%w: chrome failed", app.ErrRenderPreview)
	}
	publishCalled := false
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		publishCalled = true
		return app.PublishResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--publish-xhs"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() = 0, want non-zero")
	}
	if publishCalled {
		t.Fatal("publishXHS called after render failure")
	}
}

func TestRunAutoPublishXHSRejectsMissingGeneratedImages(t *testing.T) {
	originalGeneratePreview := generatePreview
	originalPublishXHS := publishXHS
	defer func() {
		generatePreview = originalGeneratePreview
		publishXHS = originalPublishXHS
	}()

	generatePreview = func(opts Options) (app.Result, error) {
		return app.Result{PageCount: 1, OutDir: t.TempDir()}, nil
	}
	publishCalled := false
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		publishCalled = true
		return app.PublishResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--publish-xhs"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() = 0, want non-zero")
	}
	if publishCalled {
		t.Fatal("publishXHS called without generated images")
	}
	if !strings.Contains(stdout.String(), "generated 1 preview pages") {
		t.Fatalf("stdout = %q, want render summary", stdout.String())
	}
	if !strings.Contains(stderr.String(), "no generated PNG files found") {
		t.Fatalf("stderr = %q, want missing PNG error", stderr.String())
	}
}

func TestRunAutoPublishXHSRejectsMissingGeneratedImageFile(t *testing.T) {
	originalGeneratePreview := generatePreview
	originalPublishXHS := publishXHS
	defer func() {
		generatePreview = originalGeneratePreview
		publishXHS = originalPublishXHS
	}()

	missingPath := filepath.Join(t.TempDir(), "missing.png")
	generatePreview = func(opts Options) (app.Result, error) {
		return app.Result{PageCount: 1, OutDir: t.TempDir(), ImagePaths: []string{missingPath}}, nil
	}
	publishCalled := false
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		publishCalled = true
		return app.PublishResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"--input", "article.md", "--publish-xhs"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() = 0, want non-zero")
	}
	if publishCalled {
		t.Fatal("publishXHS called with missing generated image file")
	}
	if !strings.Contains(stdout.String(), "generated 1 preview pages") {
		t.Fatalf("stdout = %q, want render summary", stdout.String())
	}
	if !strings.Contains(stderr.String(), "generated PNG file not found") || !strings.Contains(stderr.String(), missingPath) {
		t.Fatalf("stderr = %q, want missing PNG file error", stderr.String())
	}
}

func TestRunPublishXHSUsesConfigDefaultsForOriginalityFlags(t *testing.T) {
	originalLoadConfig := loadConfig
	originalPublishXHS := publishXHS
	defer func() {
		loadConfig = originalLoadConfig
		publishXHS = originalPublishXHS
	}()

	declareOriginal := true
	allowContentCopy := false
	loadConfig = func(path string) (*config.Config, error) {
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:          "walker",
			DeclareOriginal:  &declareOriginal,
			AllowContentCopy: &allowContentCopy,
		}}}, nil
	}

	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if !got.DeclareOriginal || got.AllowContentCopy {
		t.Fatalf("merged opts = %#v", got)
	}
}

func TestRunPublishXHSReplaysMetadata(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()

	metaPath := filepath.Join(t.TempDir(), xhsPublishMetaFilename)
	meta := xhsPublishMeta{
		Title:            "元数据回放标题",
		Content:          "",
		Tags:             []string{"AI代理", "工程反思"},
		Images:           []string{"cover.jpg", "detail.jpg"},
		Account:          "creator-meta",
		Mode:             string(xhs.PublishModeOnlySelf),
		ChromePath:       "/tmp/meta-chrome",
		Headless:         false,
		ProfileDir:       "/tmp/meta-profile",
		DeclareOriginal:  true,
		AllowContentCopy: false,
		ChromeArgs:       []string{"no-first-run"},
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", metaPath, err)
	}

	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--meta", metaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	want := app.PublishOptions{
		Account:          "creator-meta",
		Title:            "元数据回放标题",
		Content:          "",
		Tags:             []string{"AI代理", "工程反思"},
		Mode:             string(xhs.PublishModeOnlySelf),
		ImagePaths:       []string{"cover.jpg", "detail.jpg"},
		ChromePath:       "/tmp/meta-chrome",
		Headless:         false,
		ProfileDir:       "/tmp/meta-profile",
		ChromeArgs:       []string{"no-first-run"},
		DeclareOriginal:  true,
		AllowContentCopy: false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("publish opts = %#v, want %#v", got, want)
	}
}

func TestRunPublishXHSRejectsMetadataManualFieldConflict(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()

	metaPath := filepath.Join(t.TempDir(), xhsPublishMetaFilename)
	meta := xhsPublishMeta{
		Title:   "元数据回放标题",
		Tags:    []string{"AI代理"},
		Images:  []string{"cover.jpg"},
		Account: "creator-meta",
		Mode:    string(xhs.PublishModeOnlySelf),
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", metaPath, err)
	}

	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		t.Fatal("publishXHS called for metadata with manual field conflict")
		return app.PublishResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--meta", metaPath, "--title", "other"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--meta cannot be combined with manual publish fields") || !strings.Contains(stderr.String(), "--title") {
		t.Fatalf("stderr = %q", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"publish-xhs", "--meta", metaPath, "--config", "configs/other.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() with --config = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--meta cannot be combined with manual publish fields") || !strings.Contains(stderr.String(), "--config") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsMetadataEmptyTitle(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()

	metaPath := filepath.Join(t.TempDir(), xhsPublishMetaFilename)
	meta := xhsPublishMeta{
		Title:      "",
		Content:    "",
		Tags:       []string{"AI代理"},
		Images:     []string{"cover.jpg"},
		Account:    "creator-meta",
		Mode:       string(xhs.PublishModeOnlySelf),
		ChromePath: "/tmp/meta-chrome",
		Headless:   false,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(metaPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", metaPath, err)
	}

	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		t.Fatal("publishXHS called for invalid metadata")
		return app.PublishResult{}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--meta", metaPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "error validating xhs publish metadata") || !strings.Contains(stderr.String(), "metadata title is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSParsesStandardMediaFlags(t *testing.T) {
	originalGeneratePreview := generatePreview
	originalPublishXHS := publishXHS
	defer func() {
		generatePreview = originalGeneratePreview
		publishXHS = originalPublishXHS
	}()

	previewCalled := false
	generatePreview = func(Options) (app.Result, error) {
		previewCalled = true
		return app.Result{}, nil
	}

	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg,detail.jpg", "--mode", "only-self", "--tags", "效率,AI", "--headless=false", "--profile-dir", "/tmp/xhs-profile"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if previewCalled {
		t.Fatal("generatePreview() called, want publish path only")
	}
	want := app.PublishOptions{
		Account:    "creator-a",
		Title:      "标题",
		Content:    "正文",
		Tags:       []string{"效率", "AI"},
		Mode:       string(xhs.PublishModeOnlySelf),
		ImagePaths: []string{"cover.jpg", "detail.jpg"},
		ChromePath: defaultOptions().ChromePath,
		Headless:   false,
		ProfileDir: "/tmp/xhs-profile",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("publish opts = %#v, want %#v", got, want)
	}
	for _, want := range []string{"xiaohongshu only-self-visible published", "account: creator-a", "media: standard"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestRunPublishXHSPrintsLoginGuidance(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		return app.PublishResult{}, fmt.Errorf("%w: %w", app.ErrPublishExecute, xhs.ErrNotLoggedIn)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	for _, want := range []string{"not logged in to Xiaohongshu creator center", "complete QR login"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want substring %q", stderr.String(), want)
		}
	}
}

func TestRunPublishXHSPrintsUploadInputGuidance(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		return app.PublishResult{}, fmt.Errorf("%w: %w: %w: element not found", app.ErrPublishExecute, xhs.ErrUploadFailed, xhs.ErrUploadInputMissing)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	for _, want := range []string{"upload input not found", "session may be expired", "Open the configured Chrome profile", "complete login or verification"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want substring %q", stderr.String(), want)
		}
	}
}

func TestRunPublishXHSPrintsOnlySelfVisiblePublished(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{"xiaohongshu only-self-visible published", "account: creator-a", "media: standard"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestPrintPublishXHSResultOnlySelfVisibleSummary(t *testing.T) {
	result := app.PublishResult{
		Request: xhs.PublishRequest{Account: "creator-a"},
		Result:  xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true},
	}

	var stdout bytes.Buffer
	code := printPublishXHSResult(&stdout, result)
	if code != 0 {
		t.Fatalf("printPublishXHSResult() = %d, want 0", code)
	}
	for _, want := range []string{"xiaohongshu only-self-visible published", "account: creator-a", "media: standard"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestRunPublishXHSPrintsScheduledPublishSubmitted(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account, ScheduleTime: &scheduledAt}, Result: xhs.PublishResult{Mode: xhs.PublishModeSchedule, MediaKind: xhs.MediaKindStandard, ScheduleTime: &scheduledAt}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--mode", "schedule", "--schedule-at", "2026-04-11 20:30:00", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if got.Mode != string(xhs.PublishModeSchedule) || got.ScheduleAt != "2026-04-11 20:30:00" {
		t.Fatalf("opts = %#v", got)
	}
	if !reflect.DeepEqual(got.ImagePaths, []string{"cover.jpg"}) {
		t.Fatalf("ImagePaths = %#v", got.ImagePaths)
	}
	for _, want := range []string{"xiaohongshu scheduled publish submitted", "account: creator-a", "media: standard", "at: 2026-04-11 20:30:00"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestRunPublishXHSPrintsLiveMediaKind(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		return app.PublishResult{
			Request: xhs.PublishRequest{Account: opts.Account},
			Result:  xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindLive, OnlySelfPublished: true, AttachedCount: 2, AttachedItems: []string{"p01-cover", "p02-bullets"}},
		}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-live", "--title", "标题", "--content", "正文", "--live-report", "report.json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{"xiaohongshu only-self-visible published", "account: creator-live", "media: live", "attached: 2", "items: p01-cover,p02-bullets"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestRunPublishXHSPrintsLiveBridgeFailure(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		return app.PublishResult{}, fmt.Errorf("%w: %w", app.ErrPublishExecute, xhs.ErrLiveBridgeFailed)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-live", "--title", "标题", "--content", "正文", "--live-report", "report.json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "live attach failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsMissingAccount(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--account is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsConflictingTitleSources(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--title-file", "title.txt", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --title / --title-file is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsConflictingContentSources(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--content-file", "body.md", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --content / --content-file is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsInvalidMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg", "--mode", "publish-now"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "mode must be only-self or schedule") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsLivePagesWithoutLiveReport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg", "--live-pages", "p01-cover"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--live-pages requires --live-report") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsScheduleWithoutScheduleAt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg", "--mode", "schedule"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--schedule-at is required when --mode schedule") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsConflictingMediaSources(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg", "--live-report", "report.json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "exactly one media source is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSUsesConfigDefaultsWhenFlagsOmitted(t *testing.T) {
	originalLoadConfig := loadConfig
	originalPublishXHS := publishXHS
	defer func() {
		loadConfig = originalLoadConfig
		publishXHS = originalPublishXHS
	}()

	headless := false
	loadConfig = func(path string) (*config.Config, error) {
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:     "walker",
			Headless:    &headless,
			BrowserPath: "/tmp/publish-chrome",
			ProfileDir:  "/tmp/from-config",
			Mode:        "only-self",
			ChromeArgs:  []string{"no-first-run"},
		}}}, nil
	}

	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if got.Account != "walker" || got.Headless != false || got.ChromePath != "/tmp/publish-chrome" || got.ProfileDir != "/tmp/from-config" || got.Mode != string(xhs.PublishModeOnlySelf) {
		t.Fatalf("merged opts = %#v", got)
	}
	if !reflect.DeepEqual(got.ChromeArgs, []string{"no-first-run"}) {
		t.Fatalf("ChromeArgs = %#v", got.ChromeArgs)
	}
}

func TestRunPublishXHSCLIOverridesConfigDefaults(t *testing.T) {
	originalLoadConfig := loadConfig
	originalPublishXHS := publishXHS
	defer func() {
		loadConfig = originalLoadConfig
		publishXHS = originalPublishXHS
	}()

	headless := false
	loadConfig = func(path string) (*config.Config, error) {
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{
			Account:     "walker",
			Headless:    &headless,
			BrowserPath: "/tmp/publish-chrome",
			ProfileDir:  "/tmp/from-config",
			Mode:        "only-self",
		}}}, nil
	}

	var got app.PublishOptions
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		got = opts
		scheduledAt := ptrSchedule("2026-04-16 10:00:00")
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeSchedule, MediaKind: xhs.MediaKindStandard, ScheduleTime: scheduledAt}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "writer", "--headless=true", "--chrome", "/tmp/cli-chrome", "--profile-dir", "/tmp/from-cli", "--mode", "schedule", "--schedule-at", "2026-04-16 10:00:00", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
	if got.Account != "writer" || got.Headless != true || got.ChromePath != "/tmp/cli-chrome" || got.ProfileDir != "/tmp/from-cli" || got.Mode != string(xhs.PublishModeSchedule) {
		t.Fatalf("merged opts = %#v", got)
	}
}

func TestRunPublishXHSFallsBackWhenDefaultConfigMissing(t *testing.T) {
	originalLoadConfig := loadConfig
	originalPublishXHS := publishXHS
	defer func() {
		loadConfig = originalLoadConfig
		publishXHS = originalPublishXHS
	}()

	loadConfig = func(path string) (*config.Config, error) {
		return nil, os.ErrNotExist
	}

	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		if opts.Account != "creator-a" {
			t.Fatalf("Account = %q, want creator-a", opts.Account)
		}
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeOnlySelf, MediaKind: xhs.MediaKindStandard, OnlySelfPublished: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %s", code, stderr.String())
	}
}

func TestRunPublishXHSRejectsExplicitMissingConfig(t *testing.T) {
	originalLoadConfig := loadConfig
	defer func() { loadConfig = originalLoadConfig }()

	loadConfig = func(path string) (*config.Config, error) {
		return nil, os.ErrNotExist
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--config", "missing.yaml", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "error loading config") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunPublishXHSRejectsScheduleModeFromConfigWithoutScheduleAt(t *testing.T) {
	originalLoadConfig := loadConfig
	defer func() { loadConfig = originalLoadConfig }()

	loadConfig = func(path string) (*config.Config, error) {
		return &config.Config{XHS: config.XHSCfg{Publish: config.XHSPublishCfg{Account: "walker", Mode: "schedule"}}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--title", "标题", "--content", "正文", "--images", "cover.jpg"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--schedule-at is required when --mode schedule") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func ptrSchedule(input string) *time.Time {
	_ = input
	tm := time.Date(2026, 4, 16, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	return &tm
}
