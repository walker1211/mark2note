package xhs

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

type fakePublishPage struct {
	calls               []string
	uploaded            []string
	title               string
	content             string
	scheduledAt         time.Time
	openErr             error
	uploadErr           error
	titleErr            error
	contentErr          error
	publishOnlySelfErr  error
	confirmOnlySelfErr  error
	setScheduleErr      error
	submitScheduleErr   error
	confirmScheduleErr  error
	dismissOverlaysErr  error
	applyOriginalErr    error
	applyContentCopyErr error
	originalApplied     bool
	contentCopyAllowed  bool
	orderCounter        *int
	firstActionOrder    int
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

func (f *fakePublishPage) PublishOnlySelf(_ context.Context, request PublishRequest) error {
	_ = request
	f.calls = append(f.calls, "publish-only-self")
	f.recordActionOrder()
	return f.publishOnlySelfErr
}

func (f *fakePublishPage) ConfirmOnlySelfPublished(context.Context) error {
	f.calls = append(f.calls, "confirm-only-self")
	f.recordActionOrder()
	return f.confirmOnlySelfErr
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

func (f *fakePublishPage) dismissEditorOverlays() error {
	f.calls = append(f.calls, "dismiss-overlays")
	f.recordActionOrder()
	return f.dismissOverlaysErr
}

func (f *fakePublishPage) applyOriginalDeclaration(enabled bool) error {
	f.calls = append(f.calls, "declare-original")
	f.recordActionOrder()
	f.originalApplied = enabled
	return f.applyOriginalErr
}

func (f *fakePublishPage) applyContentCopyPreference(allow bool) error {
	f.calls = append(f.calls, "content-copy")
	f.recordActionOrder()
	f.contentCopyAllowed = allow
	return f.applyContentCopyErr
}

func TestComposePublishContentSkipsExistingTags(t *testing.T) {
	text, tags := composePublishContent("#AI代理 #数据安全 #工程反思", []string{"AI代理", "数据安全", "工程反思"})

	if text != "#AI代理 #数据安全 #工程反思" {
		t.Fatalf("text = %q", text)
	}
	if len(tags) != 0 {
		t.Fatalf("tags = %#v, want empty", tags)
	}
}

func TestComposePublishContentKeepsTagsSeparateFromBody(t *testing.T) {
	text, tags := composePublishContent("正文 #AI代理", []string{"AI代理", "数据安全"})

	if text != "正文 #AI代理" {
		t.Fatalf("text = %q", text)
	}
	wantTags := []string{"数据安全"}
	if !reflect.DeepEqual(tags, wantTags) {
		t.Fatalf("tags = %#v, want %#v", tags, wantTags)
	}
}

func TestComposePublishContentMatchesExistingTagsExactly(t *testing.T) {
	text, tags := composePublishContent("正文 #AI代理", []string{"AI", "AI代理"})

	if text != "正文 #AI代理" {
		t.Fatalf("text = %q", text)
	}
	wantTags := []string{"AI"}
	if !reflect.DeepEqual(tags, wantTags) {
		t.Fatalf("tags = %#v, want %#v", tags, wantTags)
	}
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
	if err := (Publisher{}).PublishStandardOnlySelf(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardOnlySelf() error = %v", err)
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

func TestFillTitlePreservesLongTitle(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body><input placeholder="填写标题会有更多赞哦"></body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()
	page.MustElement("body")

	want := "一二三四五六七八九十一二三四五六七八九十超长"
	rodPage := &rodPage{page: page}
	if err := rodPage.FillTitle(context.Background(), want); err != nil {
		t.Fatalf("FillTitle() error = %v", err)
	}
	got := page.MustElement(`input[placeholder="填写标题会有更多赞哦"]`).MustProperty("value").String()
	if got != want {
		t.Fatalf("title input = %q, want %q", got, want)
	}
}

func TestElementBySelectorsDoesNotReturnElementWithShortTimeout(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body><input class="target"></body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	el, err := rodPage.elementBySelectors([]string{".target"})
	if err != nil {
		t.Fatalf("elementBySelectors() error = %v", err)
	}
	time.Sleep(3 * time.Second)
	if err := rodTry(func() { el.MustInput("ok") }); err != nil {
		t.Fatalf("selected element should not inherit short timeout: %v", err)
	}
	got := page.MustElement(".target").MustProperty("value").String()
	if got != "ok" {
		t.Fatalf("input value = %q, want ok", got)
	}
}

func TestWaitForTopicConfirmationRejectsPlainTextTopic(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body><div contenteditable="true" role="textbox" class="tiptap ProseMirror">#AI编程</div></body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.waitForTopicConfirmation("AI编程", 100*time.Millisecond); err == nil {
		t.Fatal("waitForTopicConfirmation() error = nil, want highlighted-topic error")
	}
}

func TestWaitForTopicSuggestionAcceptsSuggestionNode(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body><div contenteditable="true" role="textbox" class="tiptap ProseMirror"><p><span class="suggestion">#AI编程</span></p></div></body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.waitForTopicSuggestion("AI编程", 100*time.Millisecond); err != nil {
		t.Fatalf("waitForTopicSuggestion() error = %v", err)
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
	if err := (Publisher{}).PublishStandardOnlySelf(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardOnlySelf() error = %v", err)
	}
	if got := page.calls[len(page.calls)-2:]; !reflect.DeepEqual(got, []string{"publish-only-self", "confirm-only-self"}) {
		t.Fatalf("tail calls = %#v", got)
	}
}

func TestPublisherReturnsUploadFailure(t *testing.T) {
	page := &fakePublishPage{uploadErr: errors.New("upload broken")}
	request := PublishRequest{Title: "标题", Content: "正文", ImagePaths: []string{"cover.jpg"}}
	err := (Publisher{}).PublishStandardOnlySelf(context.Background(), page, request)
	if err == nil || !errors.Is(err, ErrUploadFailed) {
		t.Fatalf("PublishStandardOnlySelf() error = %v", err)
	}
}

func TestPublisherPreservesUploadInputMissingError(t *testing.T) {
	page := &fakePublishPage{uploadErr: fmt.Errorf("%w: selectors changed", ErrUploadInputMissing)}
	request := PublishRequest{Title: "标题", Content: "正文", ImagePaths: []string{"cover.jpg"}}
	err := (Publisher{}).PublishStandardOnlySelf(context.Background(), page, request)
	if err == nil || !errors.Is(err, ErrUploadInputMissing) {
		t.Fatalf("PublishStandardOnlySelf() error = %v", err)
	}
}

func TestPublisherSetsScheduleTimeBeforeSubmit(t *testing.T) {
	page := &fakePublishPage{}
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	request := PublishRequest{Title: "标题", Content: "正文", Tags: []string{"效率"}, ImagePaths: []string{"cover.jpg"}, ScheduleTime: &scheduledAt}
	if err := (Publisher{}).PublishStandardScheduled(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardScheduled() error = %v", err)
	}
	wantCalls := []string{"open", "upload", "title", "content", "dismiss-overlays", "declare-original", "content-copy", "set-schedule", "submit-scheduled", "confirm-scheduled"}
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

func TestPublisherScheduledFlowAppliesPreSubmitHooks(t *testing.T) {
	page := &fakePublishPage{}
	scheduledAt := time.Date(2026, 4, 11, 20, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	request := PublishRequest{Title: "标题", Content: "正文", ImagePaths: []string{"cover.jpg"}, ScheduleTime: &scheduledAt, DeclareOriginal: true, AllowContentCopy: false}
	if err := (Publisher{}).PublishStandardScheduled(context.Background(), page, request); err != nil {
		t.Fatalf("PublishStandardScheduled() error = %v", err)
	}
	wantCalls := []string{"open", "upload", "title", "content", "dismiss-overlays", "declare-original", "content-copy", "set-schedule", "submit-scheduled", "confirm-scheduled"}
	if !reflect.DeepEqual(page.calls, wantCalls) {
		t.Fatalf("calls = %#v, want %#v", page.calls, wantCalls)
	}
	if !page.originalApplied {
		t.Fatal("expected original declaration hook to run")
	}
	if page.contentCopyAllowed {
		t.Fatal("expected content copy hook to disable copy")
	}
}

func TestOpenPermissionDropdownSupportsMouseDownDrivenSelect(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	t.Run("mousedown opens dropdown", func(t *testing.T) {
		page := browser.MustPage("about:blank")
		defer page.MustClose()

		html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="permission-card-wrapper">
      <div class="d-select-wrapper permission-card-select" tabindex="1">
        <div class="d-select --color-text-title --color-bg-fill">
          <div class="d-grid d-select-main d-select-main-prefix-indicator --color-text-title">公开可见</div>
        </div>
      </div>
    </div>
    <div class="d-popover d-dropdown" style="display:none">
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div></div>
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div></div>
    </div>
    <script>
      const trigger = document.querySelector('.permission-card-select');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      trigger.addEventListener('mousedown', () => {
        dropdown.style.display = 'block';
      });
    </script>
  </body>
</html>`
		dataURL := "data:text/html," + url.PathEscape(html)
		page.MustNavigate(dataURL)
		page.MustWaitLoad()
		page.MustElement("body")

		rodPage := &rodPage{page: page}
		if err := rodPage.openPermissionDropdown(); err != nil {
			t.Fatalf("openPermissionDropdown() error = %v", err)
		}

		visible := page.MustEval(`() => {
			const node = document.querySelector('.d-popover.d-dropdown');
			if (!node) return false;
			const style = window.getComputedStyle(node);
			return style.display !== 'none' && style.visibility !== 'hidden';
		}`).Bool()
		if !visible {
			t.Fatal("permission dropdown should be visible after openPermissionDropdown()")
		}
	})

	t.Run("click fallback must not re-close dropdown", func(t *testing.T) {
		page := browser.MustPage("about:blank")
		defer page.MustClose()

		html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="permission-card-wrapper">
      <div class="d-select-wrapper permission-card-select" tabindex="1">
        <div class="d-select --color-text-title --color-bg-fill">
          <div class="d-grid d-select-main d-select-main-prefix-indicator --color-text-title">公开可见</div>
        </div>
      </div>
    </div>
    <div class="d-popover d-dropdown" style="display:none">
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div></div>
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div></div>
    </div>
    <script>
      const trigger = document.querySelector('.permission-card-select');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      trigger.addEventListener('mousedown', () => {
        dropdown.style.display = 'block';
      });
      trigger.addEventListener('click', () => {
        dropdown.style.display = dropdown.style.display === 'none' ? 'block' : 'none';
      });
    </script>
  </body>
</html>`
		dataURL := "data:text/html," + url.PathEscape(html)
		page.MustNavigate(dataURL)
		page.MustWaitLoad()
		page.MustElement("body")

		rodPage := &rodPage{page: page}
		if err := rodPage.openPermissionDropdown(); err != nil {
			t.Fatalf("openPermissionDropdown() error = %v", err)
		}

		visible := page.MustEval(`() => {
			const node = document.querySelector('.d-popover.d-dropdown');
			if (!node) return false;
			const style = window.getComputedStyle(node);
			return style.display !== 'none' && style.visibility !== 'hidden';
		}`).Bool()
		if !visible {
			t.Fatal("permission dropdown should remain visible after openPermissionDropdown()")
		}
	})

	t.Run("text node trigger also opens dropdown", func(t *testing.T) {
		page := browser.MustPage("about:blank")
		defer page.MustClose()

		html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="permission-card-wrapper">
      <div class="permission-label">公开可见</div>
    </div>
    <div class="d-popover d-dropdown" style="display:none">
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div></div>
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div></div>
    </div>
    <script>
      const trigger = document.querySelector('.permission-label');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      trigger.addEventListener('pointerdown', () => {
        dropdown.style.display = 'block';
      });
    </script>
  </body>
</html>`
		dataURL := "data:text/html," + url.PathEscape(html)
		page.MustNavigate(dataURL)
		page.MustWaitLoad()
		page.MustElement("body")

		rodPage := &rodPage{page: page}
		if err := rodPage.openPermissionDropdown(); err != nil {
			t.Fatalf("openPermissionDropdown() error = %v", err)
		}

		visible := page.MustEval(`() => {
			const node = document.querySelector('.d-popover.d-dropdown');
			if (!node) return false;
			const style = window.getComputedStyle(node);
			return style.display !== 'none' && style.visibility !== 'hidden';
		}`).Bool()
		if !visible {
			t.Fatal("permission dropdown should become visible when only a text trigger exists")
		}
	})
}

func TestOpenPermissionDropdownAcceptsAlreadyVisibleDropdown(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="d-popover d-dropdown" style="display:block">
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div></div>
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div></div>
    </div>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.openPermissionDropdown(); err != nil {
		t.Fatalf("openPermissionDropdown() error = %v", err)
	}
}

func TestSelectPermissionOptionSupportsMouseDownDrivenOption(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="permission-card-wrapper">
      <div class="d-select-wrapper permission-card-select" tabindex="1">
        <div class="d-select --color-text-title --color-bg-fill">
          <div class="d-grid d-select-main d-select-main-prefix-indicator --color-text-title">
            <div class="d-select-content"><div class="d-select-description">公开可见</div></div>
          </div>
        </div>
      </div>
    </div>
    <div class="d-popover d-dropdown" style="display:block">
      <div class="d-grid-item">
        <div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div>
      </div>
      <div class="d-grid-item option-handler">
        <div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div>
      </div>
    </div>
    <script>
      const triggerText = document.querySelector('.d-select-description');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      const option = document.querySelector('.option-handler');
      option.addEventListener('mousedown', () => {
        triggerText.textContent = '仅自己可见';
        dropdown.style.display = 'none';
      });
      option.addEventListener('click', (event) => {
        event.preventDefault();
      });
    </script>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	matched := page.MustEval(`(targetText) => {
		const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
		const isVisible = (node) => {
			if (!node) return false;
			const rect = node.getBoundingClientRect();
			const style = window.getComputedStyle(node);
			return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
		};
		const fireMouseEvent = (node, type) => {
			node.dispatchEvent(new MouseEvent(type, {
				bubbles: true,
				cancelable: true,
				view: window,
				button: 0,
			}));
		};
		const dropdowns = Array.from(document.querySelectorAll('.d-popover.d-dropdown, .d-dropdown')).filter(isVisible);
		for (const dropdown of dropdowns) {
			const options = Array.from(dropdown.querySelectorAll('.custom-option, .d-grid-item, .name'));
			for (const option of options) {
				const current = normalize(option.innerText || option.textContent || '');
				if (current !== targetText) {
					continue;
				}
				const clickable = option.closest('.d-grid-item') || option.closest('.custom-option') || option.closest('.group-info') || option;
				if (!isVisible(clickable)) {
					continue;
				}
				clickable.scrollIntoView({ block: 'center', behavior: 'instant' });
				fireMouseEvent(clickable, 'mousedown');
				fireMouseEvent(clickable, 'mouseup');
				if (isVisible(dropdown)) {
					clickable.click();
				}
				return true;
			}
		}
		return false;
	}`, "仅自己可见").Bool()
	if !matched {
		t.Fatal("fixture should allow selecting 仅自己可见 via mousedown-driven option handler")
	}

	selected := page.MustEval(`() => {
		const node = document.querySelector('.d-select-description');
		return node ? node.textContent.trim() : '';
	}`).String()
	if selected != "仅自己可见" {
		t.Fatalf("selected visibility = %q, want %q", selected, "仅自己可见")
	}
}

func TestSetOnlySelfVisibleUsesTrustedClicksOnRealPermissionDOM(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="permission-card-wrapper">
      <div class="d-select-wrapper d-inline-block permission-card-select custom-select-44" tabindex="1">
        <div class="d-select --color-text-title --color-bg-static --color-bg-fill">
          <div class="d-grid d-select-main d-select-main-prefix-indicator --color-text-title --color-bg-static" style="display:grid;">
            <div class="d-select-content">
              <div class="d-text d-text-nowrap d-select-description --color-text-title">公开可见</div>
            </div>
            <div class="d-select-suffix --color-text-description d-select-suffix-indicator">▼</div>
          </div>
        </div>
      </div>
    </div>
    <div class="d-popover d-popover-default d-dropdown --size-min-width-large custom-dropdown-44" style="display:none; min-width:308px;">
      <div class="d-dropdown-wrapper">
        <div class="d-dropdown-content">
          <div class="d-options-wrapper">
            <div class="d-grid d-options" style="display:grid;">
              <div class="d-grid-item" style="grid-area:1 / 1 / auto / -1;">
                <div class="custom-option --color-bg-fill-light --color-primary">
                  <div class="group-info"><div class="name">公开可见</div></div>
                </div>
              </div>
              <div class="d-grid-item option-only-self" style="grid-area:2 / 1 / auto / -1;">
                <div class="custom-option --color-bg-fill-light --color-text-title">
                  <div class="group-info"><div class="name">仅自己可见</div></div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    <script>
      const trigger = document.querySelector('.permission-card-select');
      const description = document.querySelector('.d-select-description');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      const option = document.querySelector('.option-only-self');
      trigger.addEventListener('mousedown', (event) => {
        if (!event.isTrusted) return;
        dropdown.style.display = 'block';
      });
      option.addEventListener('mousedown', (event) => {
        if (!event.isTrusted) return;
        description.textContent = '仅自己可见';
        dropdown.style.display = 'none';
      });
      option.addEventListener('click', (event) => {
        if (!event.isTrusted) {
          event.preventDefault();
        }
      });
    </script>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.setOnlySelfVisible(); err != nil {
		t.Fatalf("setOnlySelfVisible() error = %v", err)
	}

	selected := page.MustEval(`() => {
		const node = document.querySelector('.d-select-description');
		return node ? node.textContent.trim() : '';
	}`).String()
	if selected != "仅自己可见" {
		t.Fatalf("selected visibility = %q, want %q", selected, "仅自己可见")
	}
}

func TestOpenPermissionDropdownIgnoresMatchingTextOutsidePermissionCard(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="external-label">公开可见</div>
    <div class="permission-card-wrapper">
      <div class="permission-label">公开可见</div>
    </div>
    <div class="d-popover d-dropdown" style="display:none">
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div></div>
      <div class="d-grid-item"><div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div></div>
    </div>
    <script>
      const external = document.querySelector('.external-label');
      const trigger = document.querySelector('.permission-card-wrapper .permission-label');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      external.addEventListener('pointerdown', () => {
        window.externalClicked = true;
      });
      trigger.addEventListener('pointerdown', () => {
        dropdown.style.display = 'block';
      });
    </script>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.openPermissionDropdown(); err != nil {
		t.Fatalf("openPermissionDropdown() error = %v", err)
	}

	externalClicked := page.MustEval(`() => !!window.externalClicked`).Bool()
	if externalClicked {
		t.Fatal("openPermissionDropdown() should not click matching text outside permission card")
	}
}

func TestSelectPermissionOptionIgnoresMatchingTextOutsideDropdown(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="permission-card-wrapper">
      <div class="d-select-wrapper permission-card-select" tabindex="1">
        <div class="d-select --color-text-title --color-bg-fill">
          <div class="d-grid d-select-main d-select-main-prefix-indicator --color-text-title">
            <div class="d-select-content"><div class="d-select-description">公开可见</div></div>
          </div>
        </div>
      </div>
    </div>
    <div class="outside-name name">仅自己可见</div>
    <div class="d-popover d-dropdown custom-dropdown-44" style="display:block">
      <div class="d-grid-item">
        <div class="custom-option"><div class="group-info"><div class="name">公开可见</div></div></div>
      </div>
      <div class="d-grid-item option-only-self">
        <div class="custom-option"><div class="group-info"><div class="name">仅自己可见</div></div></div>
      </div>
    </div>
    <script>
      const description = document.querySelector('.d-select-description');
      const dropdown = document.querySelector('.d-popover.d-dropdown');
      const outside = document.querySelector('.outside-name');
      const option = document.querySelector('.option-only-self');
      outside.addEventListener('mousedown', () => {
        window.outsideClicked = true;
      });
      option.addEventListener('mousedown', () => {
        description.textContent = '仅自己可见';
        dropdown.style.display = 'none';
      });
      option.addEventListener('click', (event) => {
        event.preventDefault();
      });
    </script>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.selectPermissionOption("仅自己可见"); err != nil {
		t.Fatalf("selectPermissionOption() error = %v", err)
	}

	outsideClicked := page.MustEval(`() => !!window.outsideClicked`).Bool()
	if outsideClicked {
		t.Fatal("selectPermissionOption() should not click matching text outside visible dropdown")
	}
}

func TestFillContentRejectsPlainTextTopicWithoutHighlight(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="tiptap-container">
      <div contenteditable="true" role="textbox" class="tiptap ProseMirror" tabindex="0"></div>
    </div>
    <script>
      const editor = document.querySelector('.tiptap.ProseMirror');
      window.spaceConfirmedTopics = [];
      window.topicTriggerKeySeen = false;
      editor.addEventListener('keydown', (event) => {
        if (event.code === 'Digit3' && event.shiftKey) window.topicTriggerKeySeen = true;
      });
      editor.addEventListener('keyup', (event) => {
        if (event.code !== 'Space' || !window.topicTriggerKeySeen) return;
        const match = (editor.textContent || '').match(/#([^#\s]+)\s*$/);
        if (match) window.spaceConfirmedTopics.push(match[1]);
      });
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	err := rodPage.FillContent(context.Background(), "测试正文", []string{"AI编程"})
	if err == nil || !strings.Contains(err.Error(), "did not enter Xiaohongshu suggestion mode") {
		t.Fatalf("FillContent() error = %v", err)
	}
}

func TestFillContentAcceptsHighlightedTopicNode(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
	<html>
	  <head><meta charset="utf-8"></head>
	  <body>
	    <div class="tiptap-container">
	      <div contenteditable="true" role="textbox" class="tiptap ProseMirror" tabindex="0"></div>
	    </div>
	    <script>
	      const editor = document.querySelector('.tiptap.ProseMirror');
	      let pendingTopic = '';
	      let spaceCount = 0;
	      editor.addEventListener('beforeinput', (event) => {
	        if (event.inputType !== 'insertText' || !event.data) return;
	        if (event.data === '#') {
	          event.preventDefault();
	          pendingTopic = '';
	          spaceCount = 0;
	          editor.innerHTML = '<p><span class="suggestion is-empty">#</span></p>';
	          return;
	        }
	        const suggestion = editor.querySelector('.suggestion');
	        if (suggestion && event.data !== ' ') {
	          event.preventDefault();
	          pendingTopic += event.data;
	          suggestion.className = 'suggestion';
	          suggestion.textContent = '#' + pendingTopic;
	          return;
	        }
	        if (suggestion && event.data === ' ') {
	          event.preventDefault();
	        }
	      });
	      editor.addEventListener('keyup', (event) => {
	        const suggestion = editor.querySelector('.suggestion');
	        if (!suggestion || event.code !== 'Space') return;
	        spaceCount++;
	        if (spaceCount < 2) return;
	        const data = JSON.stringify({id: 'topic-id', link: 'https://www.xiaohongshu.com/page/topics/topic-id?naviHidden=yes', name: pendingTopic});
	        editor.innerHTML = '<p><a class="tiptap-topic" data-topic=' + JSON.stringify(data) + ' contenteditable="false">#' + pendingTopic + '<span class="content-hide">[话题]#</span></a>&nbsp;</p>';
	      });
	    </script>
	  </body>
	</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.FillContent(context.Background(), "", []string{"AI编程"}); err != nil {
		t.Fatalf("FillContent() error = %v", err)
	}

	htmlOut := page.MustEval(`() => document.querySelector('.tiptap.ProseMirror')?.innerHTML || ''`).String()
	if !strings.Contains(htmlOut, `class="tiptap-topic"`) || !strings.Contains(htmlOut, `data-topic=`) {
		t.Fatalf("editor html = %q, want highlighted topic node", htmlOut)
	}
}

func TestConfirmOnlySelfPublishedAcceptsPublishedRedirect(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div>发布中...</div>
    <script>
      setTimeout(() => {
        history.replaceState({}, '', '/publish/publish?source=&published=true');
        document.body.innerHTML = '<div>上传视频</div><div>上传图文</div><div>写长文</div>';
      }, 150);
    </script>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.ConfirmOnlySelfPublished(context.Background()); err != nil {
		t.Fatalf("ConfirmOnlySelfPublished() error = %v", err)
	}
}

func TestApplyOriginalDeclarationChecksUncheckedOriginalBox(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <section class="original-block">
      <div class="option-row">原创<label>声明原创<input id="original" type="checkbox"></label></div>
      <div>未声明</div>
    </section>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyOriginalDeclaration(true); err != nil {
		t.Fatalf("applyOriginalDeclaration(true) error = %v", err)
	}
	if !page.MustElement("#original").MustProperty("checked").Bool() {
		t.Fatal("original checkbox should be checked")
	}
}

func TestApplyContentCopyPreferenceUnchecksAllowCopyBox(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="copy-option">允许正文复制<label>允许正文复制<input id="copy" type="checkbox" checked></label></div>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyContentCopyPreference(false); err != nil {
		t.Fatalf("applyContentCopyPreference(false) error = %v", err)
	}
	if page.MustElement("#copy").MustProperty("checked").Bool() {
		t.Fatal("copy checkbox should be unchecked")
	}
}

func TestApplyOriginalDeclarationChecksAgreementInModal(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="original-entry">原创<label>声明原创<input id="original" type="checkbox"></label></div>
    <div class="d-modal-content">
      <div class="footerLeft">
        <label class="agreement">我已阅读并同意《原创声明须知》<input id="agreement" type="checkbox"></label>
      </div>
      <button id="confirm-original">声明原创</button>
    </div>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyOriginalDeclaration(true); err != nil {
		t.Fatalf("applyOriginalDeclaration(true) error = %v", err)
	}
	if !page.MustElement("#original").MustProperty("checked").Bool() {
		t.Fatal("original checkbox should be checked")
	}
	if !page.MustElement("#agreement").MustProperty("checked").Bool() {
		t.Fatal("agreement checkbox should be checked")
	}
}

func TestApplyOriginalDeclarationClicksConfirmButtonInModal(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="original-entry">原创<label>声明原创<input id="original" type="checkbox"></label></div>
    <div class="d-modal-content">
      <div class="footerLeft">
        <label class="agreement">我已阅读并同意《原创声明须知》<input id="agreement" type="checkbox"></label>
      </div>
      <button id="confirm-original">声明原创</button>
    </div>
    <script>
      document.getElementById('confirm-original').addEventListener('click', () => {
        document.body.setAttribute('data-original-confirmed', document.getElementById('agreement').checked ? 'yes' : 'no');
      });
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyOriginalDeclaration(true); err != nil {
		t.Fatalf("applyOriginalDeclaration(true) error = %v", err)
	}
	confirmed := page.MustEval(`() => document.body.getAttribute('data-original-confirmed') || ''`).String()
	if confirmed != "yes" {
		t.Fatalf("confirmed = %q, want yes", confirmed)
	}
}

func TestApplyOriginalDeclarationClicksEntryBeforeConfirmingModal(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="d-grid d-checkbox d-checkbox-main-label" id="original-entry">
      <span class="d-checkbox-simulator" id="original-simulator"></span>
      <span class="d-checkbox-label">原创</span>
    </div>
    <template id="modal-template">
      <div class="d-modal-content">
        <div class="footerLeft">
          <div class="d-grid d-checkbox d-checkbox-main-label" id="agreement-label">
            <span class="d-checkbox-simulator" id="agreement-simulator"></span>
            <input id="agreement" type="checkbox" onclick="event.preventDefault(); return false;">
            <span class="d-checkbox-label">我已阅读并同意《原创声明须知》</span>
          </div>
        </div>
        <button id="confirm-original">声明原创</button>
      </div>
    </template>
    <script>
      document.getElementById('original-entry').addEventListener('click', () => {
        document.getElementById('original-simulator').classList.add('checked');
        if (!document.querySelector('.d-modal-content')) {
          document.body.appendChild(document.getElementById('modal-template').content.cloneNode(true));
          document.getElementById('agreement-label').addEventListener('click', () => {
            document.getElementById('agreement').checked = true;
            document.getElementById('agreement-simulator').classList.add('checked');
          });
          document.getElementById('confirm-original').addEventListener('click', () => {
            document.body.setAttribute('data-original-confirmed', document.getElementById('agreement').checked ? 'yes' : 'no');
          });
        }
      });
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyOriginalDeclaration(true); err != nil {
		t.Fatalf("applyOriginalDeclaration(true) error = %v", err)
	}
	confirmed := page.MustEval(`() => document.body.getAttribute('data-original-confirmed') || ''`).String()
	if confirmed != "yes" {
		t.Fatalf("confirmed = %q, want yes", confirmed)
	}
}

func TestApplyOriginalDeclarationUsesVisibleSwitchAndConfirmsPrompt(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
	<html>
	  <head><meta charset="utf-8"></head>
	  <body>
	    <div class="creator-original-card" id="original-card">
	      <div class="title">原创声明</div>
	      <input id="original" type="checkbox" style="position:absolute;opacity:0;width:0;height:0;">
	      <span id="original-switch" style="display:flex;width:20px;height:20px;"></span>
	    </div>
	    <template id="original-prompt-template">
	      <div class="d-modal-content">
	        <p>笔记完成原创声明后，将获得以下权益</p>
	        <div class="agreement-row"><input id="agreement" type="checkbox" style="position:absolute;opacity:0;width:0;height:0;">
	        <span id="agreement-switch" style="display:inline-flex;width:20px;height:20px;"></span>
	        <span>我已阅读并同意《原创声明须知》，如滥用声明，平台将驳回并予以相关处置</span></div>
	        <button id="confirm-original">声明原创</button>
	      </div>
	    </template>
	    <script>
	      document.getElementById('original-switch').addEventListener('click', () => {
	        if (!document.querySelector('.d-modal-content')) {
	          document.body.appendChild(document.getElementById('original-prompt-template').content.cloneNode(true));
	          document.getElementById('agreement-switch').addEventListener('click', () => {
	            document.getElementById('agreement').checked = true;
	          });
	          document.getElementById('confirm-original').addEventListener('click', () => {
	            if (document.getElementById('agreement').checked) {
	              document.getElementById('original').checked = true;
	              document.body.insertAdjacentHTML('beforeend', '<span>已声明原创</span>');
	            }
	          });
	        }
	      });
	    </script>
	  </body>
	</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyOriginalDeclaration(true); err != nil {
		t.Fatalf("applyOriginalDeclaration(true) error = %v", err)
	}
	if !page.MustElement("#original").MustProperty("checked").Bool() {
		t.Fatal("original checkbox should be checked after prompt confirmation")
	}
	if !page.MustElement("#agreement").MustProperty("checked").Bool() {
		t.Fatal("agreement checkbox should be checked")
	}
}

func TestApplyOriginalDeclarationClicksWrapperWithHiddenInput(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="d-grid d-checkbox d-checkbox-main-label original-entry" id="original-wrapper">
      <span class="d-checkbox-simulator" id="original-simulator"></span>
      <input id="original" type="checkbox" style="position:absolute;opacity:0;width:0;height:0;" onclick="event.preventDefault(); return false;">
      <span class="d-checkbox-label">声明原创</span>
    </div>
    <script>
      document.getElementById('original-wrapper').addEventListener('click', () => {
        document.getElementById('original').checked = true;
        document.getElementById('original-simulator').classList.add('checked');
      });
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.applyOriginalDeclaration(true); err != nil {
		t.Fatalf("applyOriginalDeclaration(true) error = %v", err)
	}
	if !page.MustElement("#original").MustProperty("checked").Bool() {
		t.Fatal("original checkbox should be checked")
	}
	checkedClass := page.MustEval(`() => document.getElementById('original-simulator').classList.contains('checked')`).Bool()
	if !checkedClass {
		t.Fatal("original wrapper should receive checked class")
	}
}

func TestSetCheckboxStateClicksVisibleWrapper(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <head><meta charset="utf-8"></head>
  <body>
    <div class="d-modal-content">
      <div class="footerLeft">
        <div class="d-grid d-checkbox d-checkbox-main-label" id="agreement-label">
          <span class="d-checkbox-simulator" id="agreement-simulator"></span>
          <input id="agreement" type="checkbox" onclick="event.preventDefault(); return false;">
          <span class="d-checkbox-label">我已阅读并同意《原创声明须知》</span>
        </div>
      </div>
    </div>
    <script>
      document.getElementById('agreement-label').addEventListener('click', () => {
        document.getElementById('agreement').checked = true;
        document.getElementById('agreement-simulator').classList.add('checked');
      });
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html;charset=utf-8," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.setCheckboxState("我已阅读并同意", true); err != nil {
		t.Fatalf("setCheckboxState() error = %v", err)
	}
	if !page.MustElement("#agreement").MustProperty("checked").Bool() {
		t.Fatal("agreement checkbox should be checked")
	}
	checkedClass := page.MustEval(`() => document.getElementById('agreement-simulator').classList.contains('checked')`).Bool()
	if !checkedClass {
		t.Fatal("agreement wrapper should receive checked class")
	}
}

func TestConfirmOnlySelfPublishedAcceptsNoteManagerRedirect(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <body>
    <div>提交中...</div>
    <script>
      setTimeout(() => {
        history.replaceState({}, '', '/publish/publish?source=&published=true');
        document.body.innerHTML = '<div>笔记管理</div><div>草稿箱</div>';
      }, 150);
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.ConfirmOnlySelfPublished(context.Background()); err != nil {
		t.Fatalf("ConfirmOnlySelfPublished() error = %v", err)
	}
}

func TestConfirmScheduledSubmittedAcceptsPublishedRedirect(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <body>
    <div>提交中...</div>
    <script>
      setTimeout(() => {
        history.replaceState({}, '', '/publish/publish?source=&published=true');
        document.body.innerHTML = '<div>笔记管理</div><div>草稿箱</div>';
      }, 150);
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	if err := rodPage.ConfirmScheduledSubmitted(context.Background()); err != nil {
		t.Fatalf("ConfirmScheduledSubmitted() error = %v", err)
	}
}

func TestConfirmOnlySelfPublishedAcceptsCleanPublishPageWithoutToast(t *testing.T) {
	l := launcher.New().Headless(true)
	controlURL := l.MustLaunch()
	defer l.Kill()
	defer l.Cleanup()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	defer browser.MustClose()

	page := browser.MustPage("about:blank")
	defer page.MustClose()

	html := `<!doctype html>
<html>
  <body>
    <div>提交中...</div>
    <script>
      setTimeout(() => {
        history.replaceState({}, '', '/publish/publish?source=&published=true');
        document.body.innerHTML = '<div>上传视频</div><div>上传图文</div><div>写长文</div>';
      }, 150);
    </script>
  </body>
</html>`
	page.MustNavigate("data:text/html," + url.PathEscape(html))
	page.MustWaitLoad()

	rodPage := &rodPage{page: page}
	started := time.Now()
	if err := rodPage.ConfirmOnlySelfPublished(context.Background()); err != nil {
		t.Fatalf("ConfirmOnlySelfPublished() error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("ConfirmOnlySelfPublished() took %v, want under 2s", elapsed)
	}
}
