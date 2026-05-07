package xhs

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
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
	publishOnlySelfErr error
	confirmOnlySelfErr error
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

func (f *fakePublishPage) PublishOnlySelf(context.Context) error {
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

func TestFillContentSelectsRecommendedTopicAndClosesPopup(t *testing.T) {
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
    <div class="recommend-topic-wrapper">
      <div class="tag-group">
        <span class="tag">#没想法随便发</span>
      </div>
    </div>
    <div data-tippy-root id="tippy-1" style="visibility:hidden; position:absolute; inset:0 auto auto 0; margin:0; transform:translate3d(0,0,0)">
      <div class="tippy-box" data-state="hidden" role="tooltip">
        <div class="tippy-content" data-state="hidden">
          <div id="creator-editor-topic-container" class="items" style="display:none">
            <div class="empty"><div>无结果</div></div>
          </div>
        </div>
      </div>
    </div>
    <script>
      const editor = document.querySelector('.tiptap.ProseMirror');
      const popupRoot = document.querySelector('#tippy-1');
      const popupBox = document.querySelector('.tippy-box');
      const popupContent = document.querySelector('.tippy-content');
      const popupItems = document.querySelector('#creator-editor-topic-container');
      const recommendation = document.querySelector('.recommend-topic-wrapper .tag');
      const rawTag = '#没想法随便发';
      const topicMarker = rawTag + '[话题]#';
      const setPopupVisible = (visible) => {
        popupRoot.style.visibility = visible ? 'visible' : 'hidden';
        popupBox.dataset.state = visible ? 'visible' : 'hidden';
        popupContent.dataset.state = visible ? 'visible' : 'hidden';
        popupItems.style.display = visible ? 'block' : 'none';
      };
      editor.addEventListener('input', () => {
        const text = editor.textContent || '';
        if (text.includes(rawTag) && !text.includes(topicMarker)) {
          editor.textContent = text.replace(rawTag, topicMarker);
          setPopupVisible(true);
        }
      });
      recommendation.addEventListener('mousedown', () => {
        const text = editor.textContent || '';
        editor.textContent = text.replace(topicMarker, rawTag);
        setPopupVisible(false);
      });
      recommendation.addEventListener('click', (event) => event.preventDefault());
    </script>
  </body>
</html>`
	dataURL := "data:text/html," + url.PathEscape(html)
	page.MustNavigate(dataURL)
	page.MustWaitLoad()
	page.MustElement("body")

	rodPage := &rodPage{page: page}
	if err := rodPage.FillContent(context.Background(), "测试正文", []string{"没想法随便发"}); err != nil {
		t.Fatalf("FillContent() error = %v", err)
	}

	bodyText := page.MustEval(`() => document.querySelector('.tiptap.ProseMirror')?.textContent || ''`).String()
	if strings.Contains(bodyText, "[话题]#") {
		t.Fatalf("editor text = %q, want popup marker removed", bodyText)
	}
	if !strings.Contains(bodyText, "#没想法随便发") {
		t.Fatalf("editor text = %q, want selected topic", bodyText)
	}
	popupVisible := page.MustEval(`() => {
		const root = document.querySelector('#tippy-1');
		const items = document.querySelector('#creator-editor-topic-container');
		return !!root && !!items && window.getComputedStyle(root).visibility !== 'hidden' && window.getComputedStyle(items).display !== 'none';
	}`).Bool()
	if popupVisible {
		t.Fatal("topic popup should be closed after selecting recommended topic")
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
