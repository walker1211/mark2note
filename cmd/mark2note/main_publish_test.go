package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/walker1211/mark2note/internal/app"
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
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeDraft, MediaKind: xhs.MediaKindStandard, DraftSaved: true}}, nil
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"publish-xhs", "--account", "creator-a", "--title", "标题", "--content", "正文", "--images", "cover.jpg,detail.jpg", "--mode", "draft", "--tags", "效率,AI", "--headless=false", "--profile-dir", "/tmp/xhs-profile"}, &stdout, &stderr)
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
		Mode:       string(xhs.PublishModeDraft),
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

func TestRunPublishXHSPrintsOnlySelfVisiblePublished(t *testing.T) {
	originalPublishXHS := publishXHS
	defer func() { publishXHS = originalPublishXHS }()
	publishXHS = func(opts app.PublishOptions) (app.PublishResult, error) {
		return app.PublishResult{Request: xhs.PublishRequest{Account: opts.Account}, Result: xhs.PublishResult{Mode: xhs.PublishModeDraft, MediaKind: xhs.MediaKindStandard, DraftSaved: true}}, nil
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
			Result:  xhs.PublishResult{Mode: xhs.PublishModeDraft, MediaKind: xhs.MediaKindLive, DraftSaved: true, AttachedCount: 2, AttachedItems: []string{"p01-cover", "p02-bullets"}},
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
	if !strings.Contains(stderr.String(), "mode must be draft or schedule") {
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
