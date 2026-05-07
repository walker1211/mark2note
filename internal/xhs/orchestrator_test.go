package xhs

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/walker1211/mark2note/internal/render"
)

type fakeBrowserSession struct {
	page           PublishPage
	openErr        error
	loginErr       error
	pageErr        error
	openCalls      int
	closeCalls     int
	publisherCalls int
}

func (f *fakeBrowserSession) Open(context.Context) error {
	f.openCalls++
	return f.openErr
}

func (f *fakeBrowserSession) EnsureLoggedIn(context.Context) error { return f.loginErr }
func (f *fakeBrowserSession) Close() error {
	f.closeCalls++
	return nil
}
func (f *fakeBrowserSession) PublisherPage(context.Context) (PublishPage, error) {
	if f.pageErr != nil {
		return nil, f.pageErr
	}
	return f.page, nil
}

func TestOrchestratorRunsStandardOnlySelfFlow(t *testing.T) {
	session := &fakeBrowserSession{page: &fakePublishPage{}}
	request := PublishRequest{Account: "writer", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	result, err := NewOrchestrator(session).Publish(context.Background(), request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !result.OnlySelfPublished || result.MediaKind != MediaKindStandard || result.Mode != PublishModeOnlySelf {
		t.Fatalf("result = %#v", result)
	}
	if session.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", session.closeCalls)
	}
}

func TestOrchestratorPreservesBrowserContextOnStandardFailure(t *testing.T) {
	session := &fakeBrowserSession{page: &fakePublishPage{uploadErr: errors.New("upload broken")}}
	request := PublishRequest{Account: "writer", Title: "标题", Content: "正文", Mode: PublishModeOnlySelf, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}}
	result, err := NewOrchestrator(session).Publish(context.Background(), request)
	if err == nil {
		t.Fatal("Publish() error = nil, want error")
	}
	if !result.BrowserKept {
		t.Fatalf("result = %#v, want BrowserKept", result)
	}
	if session.closeCalls != 0 {
		t.Fatalf("closeCalls = %d, want 0", session.closeCalls)
	}
}

func TestOrchestratorRunsScheduledStandardFlow(t *testing.T) {
	session := &fakeBrowserSession{page: &fakePublishPage{}}
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	request := PublishRequest{Account: "writer", Title: "标题", Content: "正文", Mode: PublishModeSchedule, MediaKind: MediaKindStandard, ImagePaths: []string{"cover.jpg"}, ScheduleTime: &scheduledAt}
	result, err := NewOrchestrator(session).Publish(context.Background(), request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.OnlySelfPublished || result.ScheduleTime == nil || !result.ScheduleTime.Equal(scheduledAt) {
		t.Fatalf("result = %#v", result)
	}
	if session.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", session.closeCalls)
	}
}

func TestOrchestratorRunsLiveBridgeBeforeTextFill(t *testing.T) {
	orderCounter := 0
	page := &fakePublishPage{orderCounter: &orderCounter}
	session := &fakeBrowserSession{page: page}
	bridge := &capturingLiveBridge{result: LiveAttachResult{AttachedCount: 2, ItemNames: []string{"p01-cover", "p02-bullets"}, UIPreserved: true}, orderCounter: &orderCounter}
	request := livePublishRequest(PublishModeOnlySelf, nil)
	orchestrator := NewOrchestrator(session)
	orchestrator.liveBridge = bridge

	result, err := orchestrator.Publish(context.Background(), request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !result.OnlySelfPublished {
		t.Fatalf("result = %#v", result)
	}
	if bridge.calls != 1 {
		t.Fatalf("live bridge calls = %d, want 1", bridge.calls)
	}
	if len(page.calls) == 0 || page.calls[0] != "open" {
		t.Fatalf("page calls = %#v", page.calls)
	}
	if !reflect.DeepEqual(result.AttachedItems, []string{"p01-cover", "p02-bullets"}) || result.AttachedCount != 2 {
		t.Fatalf("result = %#v", result)
	}
	if bridge.attachOrder >= page.firstActionOrder {
		t.Fatalf("attachOrder = %d, firstActionOrder = %d", bridge.attachOrder, page.firstActionOrder)
	}
}

func TestOrchestratorDoesNotDowngradeLiveAttachFailure(t *testing.T) {
	page := &fakePublishPage{}
	session := &fakeBrowserSession{page: page}
	bridge := &capturingLiveBridge{err: fmtLiveAttachError(ErrLiveBridgeFailed, "photos selection failed")}
	request := livePublishRequest(PublishModeOnlySelf, nil)
	orchestrator := NewOrchestrator(session)
	orchestrator.liveBridge = bridge

	result, err := orchestrator.Publish(context.Background(), request)
	if err == nil || !errors.Is(err, ErrLiveBridgeFailed) {
		t.Fatalf("Publish() error = %v", err)
	}
	if result.AttachedCount != 0 || len(result.AttachedItems) != 0 {
		t.Fatalf("result = %#v", result)
	}
	if len(page.uploaded) != 0 {
		t.Fatalf("uploaded = %#v, want none", page.uploaded)
	}
	if len(page.calls) != 0 {
		t.Fatalf("page calls = %#v, want no editor actions", page.calls)
	}
}

func TestOrchestratorKeepsBrowserContextAfterLiveAttachFailure(t *testing.T) {
	session := &fakeBrowserSession{page: &fakePublishPage{}}
	bridge := &capturingLiveBridge{err: fmtLiveAttachError(ErrLiveBridgeFailed, "permission denied")}
	request := livePublishRequest(PublishModeOnlySelf, nil)
	orchestrator := NewOrchestrator(session)
	orchestrator.liveBridge = bridge

	result, err := orchestrator.Publish(context.Background(), request)
	if err == nil {
		t.Fatal("Publish() error = nil, want error")
	}
	if !result.BrowserKept {
		t.Fatalf("result = %#v, want BrowserKept", result)
	}
	if session.closeCalls != 0 {
		t.Fatalf("closeCalls = %d, want 0", session.closeCalls)
	}
}

func TestOrchestratorRunsScheduledLiveFlow(t *testing.T) {
	page := &fakePublishPage{}
	session := &fakeBrowserSession{page: page}
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	bridge := &capturingLiveBridge{result: LiveAttachResult{AttachedCount: 2, ItemNames: []string{"p01-cover", "p02-bullets"}, UIPreserved: true}}
	request := livePublishRequest(PublishModeSchedule, &scheduledAt)
	orchestrator := NewOrchestrator(session)
	orchestrator.liveBridge = bridge

	result, err := orchestrator.Publish(context.Background(), request)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	wantCalls := []string{"open", "title", "content", "set-schedule", "submit-scheduled", "confirm-scheduled"}
	if !reflect.DeepEqual(page.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", page.calls, wantCalls)
	}
	if result.ScheduleTime == nil || !result.ScheduleTime.Equal(scheduledAt) {
		t.Fatalf("result = %#v", result)
	}
}

type capturingLiveBridge struct {
	request      LiveAttachRequest
	result       LiveAttachResult
	err          error
	calls        int
	attachOrder  int
	orderCounter *int
}

func (f *capturingLiveBridge) Attach(_ context.Context, req LiveAttachRequest) (LiveAttachResult, error) {
	f.calls++
	if f.orderCounter != nil {
		*f.orderCounter = *f.orderCounter + 1
		f.attachOrder = *f.orderCounter
	}
	f.request = req
	return f.result, f.err
}

func livePublishRequest(mode PublishMode, scheduleTime *time.Time) PublishRequest {
	return PublishRequest{
		Account:      "writer",
		Title:        "标题",
		Content:      "正文",
		Mode:         mode,
		ScheduleTime: scheduleTime,
		MediaKind:    MediaKindLive,
		Live: LivePublishSource{
			Resolved: &ResolvedLiveSource{
				Report: renderLiveReport("mark2note-live"),
				Items: []ResolvedLiveItem{
					{PageName: "p01-cover", PhotoPath: "/tmp/p01-cover.jpg", VideoPath: "/tmp/p01-cover.mov"},
					{PageName: "p02-bullets", PhotoPath: "/tmp/p02-bullets.jpg", VideoPath: "/tmp/p02-bullets.mov"},
				},
			},
		},
	}
}

func renderLiveReport(albumName string) render.DeliveryReport {
	return render.DeliveryReport{AlbumName: albumName, Status: "partial", SourceDir: "/tmp/apple-live"}
}

func fmtLiveAttachError(base error, detail string) error {
	return errors.Join(base, errors.New(detail))
}
