package xhs

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func shanghaiTime(y int, m time.Month, d, hh, mm, ss int) time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return time.Date(y, m, d, hh, mm, ss, 0, loc)
}

func TestPublishRequestValidateAcceptsOnlySelfStandardRequest(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	request := PublishRequest{
		Account:    "creator-a",
		Title:      "标题",
		Content:    "正文",
		Mode:       PublishModeOnlySelf,
		MediaKind:  MediaKindStandard,
		ImagePaths: []string{"cover.jpg"},
	}
	if err := request.Validate(now); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateAcceptsScheduledRequest(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	scheduledAt := shanghaiTime(2026, 4, 10, 12, 20, 0)
	request := PublishRequest{
		Account:      "creator-a",
		Title:        "标题",
		Content:      "正文",
		Mode:         PublishModeSchedule,
		ScheduleTime: &scheduledAt,
		MediaKind:    MediaKindStandard,
		ImagePaths:   []string{"cover.jpg"},
	}
	if err := request.Validate(now); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRequiresImageForStandardRequest(t *testing.T) {
	request := PublishRequest{Account: "creator-a", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard}
	if err := request.Validate(shanghaiTime(2026, 4, 10, 12, 0, 0)); err == nil || !strings.Contains(err.Error(), "at least one image path is required") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRejectsOnlySelfWithScheduleTime(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	scheduledAt := shanghaiTime(2026, 4, 10, 12, 20, 0)
	request := PublishRequest{Account: "creator-a", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, ScheduleTime: &scheduledAt, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := request.Validate(now); err == nil || !strings.Contains(err.Error(), "only-self mode forbids schedule time") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRejectsScheduleWithoutScheduleTime(t *testing.T) {
	request := PublishRequest{Account: "creator-a", Title: "标题", Content: "正文", Mode: PublishModeSchedule, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := request.Validate(shanghaiTime(2026, 4, 10, 12, 0, 0)); err == nil || !strings.Contains(err.Error(), "schedule mode requires schedule time") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateAcceptsLiveSourceWithOrderedSubsetPages(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	request := PublishRequest{
		Account:   "creator-live",
		Title:     "标题",
		Content:   "正文",
		Mode:      PublishModeOnlySelf,
		MediaKind: MediaKindLive,
		Live: LivePublishSource{
			ReportPath:   "report.json",
			RequestedSet: []string{"p01-cover", "p02-bullets"},
			Resolved: &ResolvedLiveSource{Items: []ResolvedLiveItem{
				{PageName: "p01-cover", PhotoPath: "/tmp/p01-cover.jpg", VideoPath: "/tmp/p01-cover.mov"},
				{PageName: "p02-bullets", PhotoPath: "/tmp/p02-bullets.jpg", VideoPath: "/tmp/p02-bullets.mov"},
			}},
		},
	}
	if err := request.Validate(now); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRequiresLiveReportPath(t *testing.T) {
	request := PublishRequest{Account: "creator-live", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindLive}
	if err := request.Validate(shanghaiTime(2026, 4, 10, 12, 0, 0)); err == nil || !strings.Contains(err.Error(), "live report path is required") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRequiresResolvedLiveSource(t *testing.T) {
	request := PublishRequest{Account: "creator-live", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindLive, Live: LivePublishSource{ReportPath: "report.json"}}
	if err := request.Validate(shanghaiTime(2026, 4, 10, 12, 0, 0)); err == nil || !strings.Contains(err.Error(), "live source must be resolved before publish") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRequiresResolvedLiveItems(t *testing.T) {
	request := PublishRequest{Account: "creator-live", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindLive, Live: LivePublishSource{ReportPath: "report.json", Resolved: &ResolvedLiveSource{}}}
	if err := request.Validate(shanghaiTime(2026, 4, 10, 12, 0, 0)); err == nil || !strings.Contains(err.Error(), "resolved live source must contain at least one item") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRequiresAccount(t *testing.T) {
	request := PublishRequest{Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := request.Validate(shanghaiTime(2026, 4, 10, 12, 0, 0)); err == nil || !strings.Contains(err.Error(), "account is required") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRequiresTitleAndContentOrTags(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	missingTitle := PublishRequest{Account: "creator-a", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := missingTitle.Validate(now); err == nil || !strings.Contains(err.Error(), "title is required") {
		t.Fatalf("Validate() error = %v", err)
	}
	missingContent := PublishRequest{Account: "creator-a", Title: "标题", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := missingContent.Validate(now); err == nil || !strings.Contains(err.Error(), "content or tags are required") {
		t.Fatalf("Validate() error = %v", err)
	}
	withTags := PublishRequest{Account: "creator-a", Title: "标题", Tags: []string{"AI编程"}, Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := withTags.Validate(now); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestPublishRequestValidateRejectsCrossMediaFields(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	standard := PublishRequest{Account: "creator-a", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}, Live: LivePublishSource{ReportPath: "report.json", Resolved: &ResolvedLiveSource{Items: []ResolvedLiveItem{{PageName: "p01", PhotoPath: "/tmp/p01.jpg", VideoPath: "/tmp/p01.mov"}}}}}
	if err := standard.Validate(now); err == nil || !strings.Contains(err.Error(), "standard media request must not include live source") {
		t.Fatalf("Validate() error = %v", err)
	}
	live := PublishRequest{Account: "creator-live", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindLive, ImagePaths: []string{"cover.jpg"}, Live: LivePublishSource{ReportPath: "report.json", Resolved: &ResolvedLiveSource{Items: []ResolvedLiveItem{{PageName: "p01", PhotoPath: "/tmp/p01.jpg", VideoPath: "/tmp/p01.mov"}}}}}
	if err := live.Validate(now); err == nil || !strings.Contains(err.Error(), "live media request must not include image paths") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRejectsPastSchedule(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	past := shanghaiTime(2026, 4, 10, 11, 59, 59)
	request := PublishRequest{Account: "creator-a", Title: "标题", Content: "正文", Mode: PublishModeSchedule, ScheduleTime: &past, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := request.Validate(now); err == nil || !strings.Contains(err.Error(), "schedule time must be in the future") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPublishRequestValidateRejectsScheduleLeadTimeUnderFifteenMinutes(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	soon := shanghaiTime(2026, 4, 10, 12, 14, 59)
	request := PublishRequest{Account: "creator-a", Title: "标题", Content: "正文", Mode: PublishModeSchedule, ScheduleTime: &soon, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	if err := request.Validate(now); err == nil || !strings.Contains(err.Error(), "schedule lead time must be at least 15 minutes") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestParseScheduleTimeAcceptsShanghaiFormat(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	got, err := ParseScheduleTime("2026-04-10 12:20:00", now)
	if err != nil {
		t.Fatalf("ParseScheduleTime() error = %v", err)
	}
	want := shanghaiTime(2026, 4, 10, 12, 20, 0)
	if got == nil || !reflect.DeepEqual(*got, want) {
		t.Fatalf("ParseScheduleTime() = %#v, want %v", got, want)
	}
}

func TestParseScheduleTimeRejectsPastTime(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	got, err := ParseScheduleTime("2026-04-10 11:59:59", now)
	if err == nil || !strings.Contains(err.Error(), "schedule time must be in the future") {
		t.Fatalf("ParseScheduleTime() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ParseScheduleTime() = %#v, want nil", got)
	}
}

func TestParseScheduleTimeRejectsLeadTimeUnderFifteenMinutes(t *testing.T) {
	now := shanghaiTime(2026, 4, 10, 12, 0, 0)
	got, err := ParseScheduleTime("2026-04-10 12:14:59", now)
	if err == nil || !strings.Contains(err.Error(), "schedule lead time must be at least 15 minutes") {
		t.Fatalf("ParseScheduleTime() error = %v", err)
	}
	if got != nil {
		t.Fatalf("ParseScheduleTime() = %#v, want nil", got)
	}
}
