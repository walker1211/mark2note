package xhs

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakePublishPage struct {
	calls              []string
	uploaded           []string
	title              string
	content            string
	scheduledAt        time.Time
	openErr            error
	uploadErr          error
	titleErr           error
	contentErr         error
	saveDraftErr       error
	confirmDraftErr    error
	setScheduleErr     error
	submitScheduleErr  error
	confirmScheduleErr error
	orderCounter       *int
	firstActionOrder   int
}

func (f *fakePublishPage) Open(context.Context) error {
	f.calls = append(f.calls, "open")
	f.recordActionOrder()
	return f.openErr
}

func (f *fakePublishPage) UploadImages(_ context.Context, paths []string) error {
	f.calls = append(f.calls, "upload")
	f.recordActionOrder()
	f.uploaded = append([]string(nil), paths...)
	return f.uploadErr
}

func (f *fakePublishPage) FillTitle(_ context.Context, title string) error {
	f.calls = append(f.calls, "title")
	f.recordActionOrder()
	f.title = title
	return f.titleErr
}

func (f *fakePublishPage) FillContent(_ context.Context, content string, tags []string) error {
	f.calls = append(f.calls, "content")
	f.recordActionOrder()
	f.content = content
	if len(tags) > 0 {
		f.content += "|" + tags[0]
	}
	return f.contentErr
}

func (f *fakePublishPage) SaveDraft(context.Context) error {
	f.calls = append(f.calls, "publish-only-self")
	f.recordActionOrder()
	return f.saveDraftErr
}

func (f *fakePublishPage) ConfirmDraftSaved(context.Context) error {
	f.calls = append(f.calls, "confirm-only-self")
	f.recordActionOrder()
	return f.confirmDraftErr
}

func (f *fakePublishPage) SetSchedule(_ context.Context, at time.Time) error {
	f.calls = append(f.calls, "set-schedule")
	f.recordActionOrder()
	f.scheduledAt = at
	return f.setScheduleErr
}
func (f *fakePublishPage) SubmitScheduled(context.Context) error {
	f.calls = append(f.calls, "submit-scheduled")
	f.recordActionOrder()
	return f.submitScheduleErr
}

func (f *fakePublishPage) ConfirmScheduledSubmitted(context.Context) error {
	f.calls = append(f.calls, "confirm-scheduled")
	f.recordActionOrder()
	return f.confirmScheduleErr
}

func (f *fakePublishPage) recordActionOrder() {
	if f.orderCounter == nil {
		return
	}
	*f.orderCounter = *f.orderCounter + 1
	if f.firstActionOrder == 0 {
		f.firstActionOrder = *f.orderCounter
	}
}

func TestPublisherUploadsStandardImagesBeforeTextFill(t *testing.T) {
	page := &fakePublishPage{}
	request := PublishRequest{Title: "标题", Content: "正文", Tags: []string{"效率"}, ImagePaths: []string{"cover.jpg", "detail.jpg"}}
	if err := (Publisher{}).PublishStandardDraft(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardDraft() error = %v", err)
	}
	wantCalls := []string{"open", "upload", "title", "content", "publish-only-self", "confirm-only-self"}
	if !reflect.DeepEqual(page.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", page.calls, wantCalls)
	}
	if !reflect.DeepEqual(page.uploaded, request.ImagePaths) {
		t.Fatalf("uploaded = %#v, want %#v", page.uploaded, request.ImagePaths)
	}
}

func TestUploadInputSelectorsIncludeCreatorUploadInput(t *testing.T) {
	if !reflect.DeepEqual(uploadInputSelectors, []string{"input.upload-input", `input[type="file"][accept*="image"]`, `input[type="file"][multiple]`, `input[type="file"]`, `input[type="file"][multiple][accept*=".jpg"]`, `input[type="file"][accept*=".jpg"]`}) {
		t.Fatalf("uploadInputSelectors = %#v", uploadInputSelectors)
	}
}

func TestTitleSelectorsPreferRealPlaceholder(t *testing.T) {
	if titleSelectors[0] != `input[placeholder="填写标题会有更多赞哦"]` {
		t.Fatalf("titleSelectors = %#v", titleSelectors)
	}
}

func TestContentSelectorsPreferTiptapEditor(t *testing.T) {
	if contentSelectors[0] != `div.tiptap.ProseMirror[contenteditable="true"]` {
		t.Fatalf("contentSelectors = %#v", contentSelectors)
	}
}

func TestPermissionDropdownSelectorMatchesRealWrapper(t *testing.T) {
	selector := `.permission-card-wrapper .d-select-wrapper.permission-card-select`
	if selector == "" {
		t.Fatal("permission dropdown selector is empty")
	}
}

func TestPublisherPublishesOnlySelfExplicitly(t *testing.T) {
	page := &fakePublishPage{}
	request := PublishRequest{Title: "标题", Content: "正文", ImagePaths: []string{"cover.jpg"}}
	if err := (Publisher{}).PublishStandardDraft(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardDraft() error = %v", err)
	}
	if got := page.calls[len(page.calls)-2:]; !reflect.DeepEqual(got, []string{"publish-only-self", "confirm-only-self"}) {
		t.Fatalf("tail calls = %#v", got)
	}
}

func TestPublisherReturnsUploadFailure(t *testing.T) {
	page := &fakePublishPage{uploadErr: errors.New("upload broken")}
	request := PublishRequest{Title: "标题", Content: "正文", ImagePaths: []string{"cover.jpg"}}
	err := (Publisher{}).PublishStandardDraft(context.Background(), page, request)
	if err == nil || !errors.Is(err, ErrUploadFailed) {
		t.Fatalf("PublishStandardDraft() error = %v", err)
	}
}

func TestPublisherSetsScheduleTimeBeforeSubmit(t *testing.T) {
	page := &fakePublishPage{}
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	request := PublishRequest{Title: "标题", Content: "正文", Tags: []string{"效率"}, ImagePaths: []string{"cover.jpg"}, ScheduleTime: &scheduledAt}
	if err := (Publisher{}).PublishStandardScheduled(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardScheduled() error = %v", err)
	}
	wantCalls := []string{"open", "upload", "title", "content", "set-schedule", "submit-scheduled", "confirm-scheduled"}
	if !reflect.DeepEqual(page.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", page.calls, wantCalls)
	}
	if !page.scheduledAt.Equal(scheduledAt) {
		t.Fatalf("scheduledAt = %v, want %v", page.scheduledAt, scheduledAt)
	}
}

func TestPublisherReturnsScheduleErrorWhenControlsMissing(t *testing.T) {
	page := &fakePublishPage{setScheduleErr: errors.New("schedule controls missing")}
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	request := PublishRequest{Title: "标题", Content: "正文", ImagePaths: []string{"cover.jpg"}, ScheduleTime: &scheduledAt}
	err := (Publisher{}).PublishStandardScheduled(context.Background(), page, request)
	if err == nil || !errors.Is(err, ErrScheduleFailed) {
		t.Fatalf("PublishStandardScheduled() error = %v", err)
	}
}
