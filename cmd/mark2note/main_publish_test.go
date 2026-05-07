package main

import (
	"bytes"
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

func TestRunAutoPublishXHSPublishesGeneratedPNGs(t *testing.T) {
	originalGeneratePreview := generatePreview
	originalPublishXHS := publishXHS
	originalReadFile := readFile
	originalLoadConfig := loadConfig
	defer func() {
		generatePreview = originalGeneratePreview
		publishXHS = originalPublishXHS
		readFile = originalReadFile
		loadConfig = originalLoadConfig
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
		}}}, nil
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
	code := run([]string{"--input", "article.md", "--publish-xhs", "--xhs-tags", "AI代理,数据安全,工程反思"}, &stdout, &stderr)
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
	if got.Content != "#AI代理 #数据安全 #工程反思" {
		t.Fatalf("Content = %q", got.Content)
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
