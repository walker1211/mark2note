package app

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/walker1211/mark2note/internal/xhs"
)

func shanghaiNow(y int, m time.Month, d, hh, mm, ss int) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return time.Date(y, m, d, hh, mm, ss, 0, loc)
}

type fakePublishOrchestrator struct {
	request xhs.PublishRequest
	result  xhs.PublishResult
	err     error
	called  int
}

func (f *fakePublishOrchestrator) Publish(request xhs.PublishRequest, options PublishRuntimeOptions) (xhs.PublishResult, error) {
	f.called++
	f.request = request
	return f.result, f.err
}

func TestPublishServiceBuildsStandardRequestFromInlineFields(t *testing.T) {
	orchestrator := &fakePublishOrchestrator{result: xhs.PublishResult{TargetAccount: "creator-a", Mode: xhs.PublishModeDraft}}
	service := PublishService{
		ReadFile:        func(string) ([]byte, error) { t.Fatal("ReadFile() should not be called"); return nil, nil },
		Now:             func() time.Time { return shanghaiNow(2026, 4, 10, 12, 0, 0) },
		NewOrchestrator: func(PublishRuntimeOptions) PublishOrchestrator { return orchestrator },
	}

	result, err := service.Publish(PublishOptions{
		Account:    "creator-a",
		Title:      "标题",
		Content:    "正文",
		Tags:       []string{"效率", "AI"},
		Mode:       string(xhs.PublishModeDraft),
		ImagePaths: []string{"cover.jpg", "detail.jpg"},
		ChromePath: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		Headless:   true,
		ProfileDir: "/tmp/profile",
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if orchestrator.called != 1 {
		t.Fatalf("orchestrator called %d times, want 1", orchestrator.called)
	}
	wantRequest := xhs.PublishRequest{
		Account:    "creator-a",
		Title:      "标题",
		Content:    "正文",
		Tags:       []string{"效率", "AI"},
		Mode:       xhs.PublishModeDraft,
		MediaKind:  xhs.MediaKindStandard,
		ImagePaths: []string{"cover.jpg", "detail.jpg"},
	}
	if !reflect.DeepEqual(orchestrator.request, wantRequest) {
		t.Fatalf("request = %#v, want %#v", orchestrator.request, wantRequest)
	}
	if result.Request.Account != "creator-a" {
		t.Fatalf("result = %#v", result)
	}
}

func TestPublishServiceBuildsLiveRequestFromFilesAndSchedule(t *testing.T) {
	now := shanghaiNow(2026, 4, 10, 12, 0, 0)
	liveRoot := t.TempDir()
	writePublishTestFile(t, filepath.Join(liveRoot, "p01-cover.jpg"), "jpg")
	writePublishTestFile(t, filepath.Join(liveRoot, "p01-cover.mov"), "mov")
	writePublishTestFile(t, filepath.Join(liveRoot, "p02-bullets.jpg"), "jpg")
	writePublishTestFile(t, filepath.Join(liveRoot, "p02-bullets.mov"), "mov")
	reportPath := writePublishTestReport(t, liveRoot, "partial")
	orchestrator := &fakePublishOrchestrator{result: xhs.PublishResult{TargetAccount: "creator-live", Mode: xhs.PublishModeSchedule}}
	service := PublishService{
		ReadFile: func(path string) ([]byte, error) {
			switch filepath.Base(path) {
			case "title.txt":
				return []byte("文件标题\n"), nil
			case "body.md":
				return []byte("文件正文\n"), nil
			default:
				return nil, errors.New("unexpected path")
			}
		},
		Now:             func() time.Time { return now },
		NewOrchestrator: func(PublishRuntimeOptions) PublishOrchestrator { return orchestrator },
	}

	result, err := service.Publish(PublishOptions{
		Account:        "creator-live",
		TitleFile:      "/tmp/title.txt",
		ContentFile:    "/tmp/body.md",
		Mode:           string(xhs.PublishModeSchedule),
		ScheduleAt:     "2026-04-10 12:20:00",
		LiveReportPath: reportPath,
		LivePages:      []string{"p01-cover", "p02-bullets"},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.Request.MediaKind != xhs.MediaKindLive {
		t.Fatalf("result = %#v", result)
	}
	if orchestrator.request.Title != "文件标题" || orchestrator.request.Content != "文件正文" {
		t.Fatalf("request = %#v", orchestrator.request)
	}
	if orchestrator.request.Live.ReportPath != reportPath || !reflect.DeepEqual(orchestrator.request.Live.RequestedSet, []string{"p01-cover", "p02-bullets"}) {
		t.Fatalf("request.Live = %#v", orchestrator.request.Live)
	}
	if orchestrator.request.Live.Resolved == nil || !reflect.DeepEqual(orchestrator.request.Live.Resolved.Items, []xhs.ResolvedLiveItem{{PageName: "p01-cover", PhotoPath: filepath.Join(liveRoot, "p01-cover.jpg"), VideoPath: filepath.Join(liveRoot, "p01-cover.mov")}, {PageName: "p02-bullets", PhotoPath: filepath.Join(liveRoot, "p02-bullets.jpg"), VideoPath: filepath.Join(liveRoot, "p02-bullets.mov")}}) {
		t.Fatalf("resolved live source = %#v", orchestrator.request.Live.Resolved)
	}
	if orchestrator.request.ScheduleTime == nil {
		t.Fatalf("ScheduleTime = nil")
	}
}

func TestPublishServiceRejectsConflictingTitleSources(t *testing.T) {
	service := PublishService{Now: func() time.Time { return shanghaiNow(2026, 4, 10, 12, 0, 0) }}
	_, err := service.Publish(PublishOptions{Account: "creator-a", Title: "标题", TitleFile: "/tmp/title.txt", Content: "正文", Mode: string(xhs.PublishModeDraft), ImagePaths: []string{"cover.jpg"}})
	if err == nil || !errors.Is(err, ErrPublishRequestInvalid) {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishServiceRejectsLivePagesWithoutLiveReport(t *testing.T) {
	service := PublishService{Now: func() time.Time { return shanghaiNow(2026, 4, 10, 12, 0, 0) }}
	_, err := service.Publish(PublishOptions{Account: "creator-a", Title: "标题", Content: "正文", Mode: string(xhs.PublishModeDraft), LivePages: []string{"p01-cover"}})
	if err == nil || !errors.Is(err, ErrPublishRequestInvalid) {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishServiceRejectsEmptyTitleFile(t *testing.T) {
	service := PublishService{
		ReadFile: func(string) ([]byte, error) { return []byte(" \n\t "), nil },
		Now:      func() time.Time { return shanghaiNow(2026, 4, 10, 12, 0, 0) },
	}
	_, err := service.Publish(PublishOptions{Account: "creator-a", TitleFile: "/tmp/title.txt", Content: "正文", Mode: string(xhs.PublishModeDraft), ImagePaths: []string{"cover.jpg"}})
	if err == nil || !errors.Is(err, ErrPublishRequestInvalid) {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishServiceReturnsScheduledTime(t *testing.T) {
	scheduledAt := shanghaiNow(2026, 4, 10, 12, 20, 0)
	orchestrator := &fakePublishOrchestrator{result: xhs.PublishResult{TargetAccount: "creator-a", Mode: xhs.PublishModeSchedule, MediaKind: xhs.MediaKindStandard, ScheduleTime: &scheduledAt}}
	service := PublishService{
		Now:             func() time.Time { return shanghaiNow(2026, 4, 10, 12, 0, 0) },
		NewOrchestrator: func(PublishRuntimeOptions) PublishOrchestrator { return orchestrator },
	}
	result, err := service.Publish(PublishOptions{Account: "creator-a", Title: "标题", Content: "正文", Mode: string(xhs.PublishModeSchedule), ScheduleAt: "2026-04-10 12:20:00", ImagePaths: []string{"cover.jpg"}})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.Result.ScheduleTime == nil || !result.Result.ScheduleTime.Equal(scheduledAt) {
		t.Fatalf("result = %#v", result)
	}
}

func TestPublishServiceRejectsFailedLiveReport(t *testing.T) {
	liveRoot := t.TempDir()
	reportPath := writePublishTestReport(t, liveRoot, "failed")
	service := PublishService{Now: func() time.Time { return shanghaiNow(2026, 4, 10, 12, 0, 0) }}
	_, err := service.Publish(PublishOptions{Account: "creator-live", Title: "标题", Content: "正文", Mode: string(xhs.PublishModeDraft), LiveReportPath: reportPath})
	if err == nil || !errors.Is(err, ErrPublishRequestInvalid) {
		t.Fatalf("Publish() error = %v", err)
	}
}

func writePublishTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writePublishTestReport(t *testing.T, sourceDir string, status string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "import-result.json")
	content := []byte("{\n  \"source_dir\": \"" + sourceDir + "\",\n  \"status\": \"" + status + "\",\n  \"message\": \"manual verification required\"\n}")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
