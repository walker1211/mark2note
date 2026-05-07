package xhs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

var (
	ErrUploadFailed   = fmt.Errorf("upload failed")
	ErrFillFailed     = fmt.Errorf("fill failed")
	ErrScheduleFailed = fmt.Errorf("schedule failed")
	ErrSubmitFailed   = fmt.Errorf("submit failed")
)

type PublishPage interface {
	Open(ctx context.Context) error
	UploadImages(ctx context.Context, paths []string) error
	FillTitle(ctx context.Context, title string) error
	FillContent(ctx context.Context, content string, tags []string) error
	PublishOnlySelf(ctx context.Context, request PublishRequest) error
	ConfirmOnlySelfPublished(ctx context.Context) error
	SetSchedule(ctx context.Context, at time.Time) error
	SubmitScheduled(ctx context.Context) error
	ConfirmScheduledSubmitted(ctx context.Context) error
}

type scheduledPreSubmitPage interface {
	dismissEditorOverlays() error
	applyOriginalDeclaration(bool) error
	applyContentCopyPreference(bool) error
}

type Publisher struct{}

func (Publisher) PublishStandardOnlySelf(ctx context.Context, page PublishPage, request PublishRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := page.Open(ctx); err != nil {
		return err
	}
	if err := page.UploadImages(ctx, request.ImagePaths); err != nil {
		return fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}
	if err := page.FillTitle(ctx, request.Title); err != nil {
		return fmt.Errorf("%w: %v", ErrFillFailed, err)
	}
	if err := page.FillContent(ctx, request.Content, request.Tags); err != nil {
		return fmt.Errorf("%w: %v", ErrFillFailed, err)
	}
	if err := page.PublishOnlySelf(ctx, request); err != nil {
		return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	if err := page.ConfirmOnlySelfPublished(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	return nil
}

var (
	uploadInputSelectors = []string{
		`input.upload-input`,
		`input[type="file"][accept*="image"]`,
		`input[type="file"][multiple]`,
		`input[type="file"]`,
		`input[type="file"][multiple][accept*=".jpg"]`,
		`input[type="file"][accept*=".jpg"]`,
	}
	titleSelectors = []string{
		`input[placeholder="填写标题会有更多赞哦"]`,
		`input[placeholder*="填写标题"]`,
		`input[placeholder*="标题"]`,
	}
	contentSelectors = []string{
		`div.tiptap.ProseMirror[contenteditable="true"]`,
		`div.ProseMirror[contenteditable="true"]`,
		`div[contenteditable="true"][role="textbox"]`,
	}
	permissionTriggerSelectors = []string{
		`.permission-card-select`,
		`.d-select-main`,
		`.d-select`,
	}
	permissionOptionSelectors = []string{
		`.name`,
		`.custom-option`,
		`.d-grid-item`,
	}
)

func (p *rodPage) Open(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.Navigate(xhsPublishURL)
}

func (p *rodPage) UploadImages(ctx context.Context, paths []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.Navigate(xhsImagePublishURL); err != nil {
		return err
	}
	if currentURL, err := p.URL(); err == nil {
		defaultXHSLogger("upload page url=%s", currentURL)
	}
	input, err := p.waitForUploadInput(ctx, 12*time.Second)
	if err != nil {
		return err
	}
	if err := rodTry(func() {
		input.MustSetFiles(paths...)
	}); err != nil {
		return err
	}
	return p.waitForEditorReady(ctx, 30*time.Second)
}

func (p *rodPage) FillTitle(ctx context.Context, title string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	field, err := p.elementBySelectors(titleSelectors)
	if err != nil {
		return err
	}
	return rodTry(func() {
		field.MustInput(title)
	})
}

func (p *rodPage) FillContent(ctx context.Context, content string, tags []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	field, err := p.elementBySelectors(contentSelectors)
	if err != nil {
		return err
	}
	text := strings.TrimSpace(content)
	trimmedTags := make([]string, 0, len(tags))
	if len(tags) > 0 {
		tagParts := make([]string, 0, len(tags))
		for _, tag := range tags {
			trimmed := strings.TrimSpace(tag)
			if trimmed == "" {
				continue
			}
			trimmedTags = append(trimmedTags, trimmed)
			tagParts = append(tagParts, "#"+trimmed)
		}
		if len(tagParts) > 0 {
			text = text + "\n" + strings.Join(tagParts, " ")
		}
	}
	if err := rodTry(func() {
		field.MustInput(text)
	}); err != nil {
		return err
	}
	if len(trimmedTags) == 0 {
		return nil
	}
	for _, tag := range trimmedTags {
		if err := p.selectRecommendedTopic(tag); err != nil {
			return err
		}
	}
	return nil
}

func (p *rodPage) PublishOnlySelf(ctx context.Context, request PublishRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish dismiss editor overlays")
	if err := p.dismissEditorOverlays(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish declare original")
	if err := p.applyOriginalDeclaration(request.DeclareOriginal); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish set content copy")
	if err := p.applyContentCopyPreference(request.AllowContentCopy); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish set visibility")
	if err := p.setOnlySelfVisible(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish click submit")
	clicked, err := p.clickByText("button", "^发布$")
	if err != nil {
		return err
	}
	if !clicked {
		return fmt.Errorf("only-self publish action not found")
	}
	return nil
}

func (p *rodPage) ConfirmOnlySelfPublished(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish wait confirmation")
	if err := p.waitForBodyText(ctx, `发布成功|提交成功|笔记发布成功`, 15*time.Second); err == nil {
		return nil
	}
	if err := p.waitForPublishedRedirect(ctx, 5*time.Second); err == nil {
		return nil
	}
	p.debugPublishConfirmationState()
	return fmt.Errorf("only-self publish confirmation not observed")
}

func (Publisher) PublishStandardScheduled(ctx context.Context, page PublishPage, request PublishRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if request.ScheduleTime == nil {
		return fmt.Errorf("%w: schedule time is required", ErrScheduleFailed)
	}
	if err := page.Open(ctx); err != nil {
		return err
	}
	if err := page.UploadImages(ctx, request.ImagePaths); err != nil {
		return fmt.Errorf("%w: %v", ErrUploadFailed, err)
	}
	if err := page.FillTitle(ctx, request.Title); err != nil {
		return fmt.Errorf("%w: %v", ErrFillFailed, err)
	}
	if err := page.FillContent(ctx, request.Content, request.Tags); err != nil {
		return fmt.Errorf("%w: %v", ErrFillFailed, err)
	}
	if err := runScheduledPreSubmitHooks(page, request); err != nil {
		return err
	}
	if err := page.SetSchedule(ctx, *request.ScheduleTime); err != nil {
		return fmt.Errorf("%w: %v", ErrScheduleFailed, err)
	}
	if err := page.SubmitScheduled(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	if err := page.ConfirmScheduledSubmitted(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	return nil
}

func runScheduledPreSubmitHooks(page PublishPage, request PublishRequest) error {
	preSubmitPage, ok := page.(scheduledPreSubmitPage)
	if !ok {
		return nil
	}
	if err := preSubmitPage.dismissEditorOverlays(); err != nil {
		return err
	}
	if err := preSubmitPage.applyOriginalDeclaration(request.DeclareOriginal); err != nil {
		return err
	}
	if err := preSubmitPage.applyContentCopyPreference(request.AllowContentCopy); err != nil {
		return err
	}
	return nil
}

func (p *rodPage) SetSchedule(ctx context.Context, at time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	formatted := at.In(shanghaiLocation()).Format("2006-01-02 15:04:05")
	return rodTry(func() {
		p.page.MustElementR("label,button,div,span", "定时发布|发布时间").MustClick()
		input := p.page.MustElement(`input[placeholder*="选择发布时间"]`)
		input.MustEval(`(value) => {
			this.focus();
			this.value = '';
			this.dispatchEvent(new Event('input', { bubbles: true }));
			this.dispatchEvent(new Event('change', { bubbles: true }));
			this.value = value;
			this.dispatchEvent(new Event('input', { bubbles: true }));
			this.dispatchEvent(new Event('change', { bubbles: true }));
		}`, formatted)
		p.page.MustWaitStable()
	})
}

func (p *rodPage) SubmitScheduled(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return rodTry(func() {
		if clicked, err := p.clickVisibleNodeByText("button", "^确认发布$|^定时发布$"); err != nil {
			panic(err)
		} else if !clicked {
			panic(fmt.Errorf("scheduled submit action not found"))
		}
		p.page.MustWaitStable()
	})
}

func (p *rodPage) ConfirmScheduledSubmitted(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.waitForBodyText(ctx, `发布成功|提交成功|预约成功`, 15*time.Second); err == nil {
		return nil
	}
	if err := p.waitForPublishedRedirect(ctx, 5*time.Second); err == nil {
		return nil
	}
	return fmt.Errorf("scheduled publish confirmation not observed")
}

func (p *rodPage) clickByText(selector string, pattern string) (bool, error) {
	clicked := false
	err := rodTry(func() {
		el, findErr := p.page.Timeout(2*time.Second).ElementR(selector, pattern)
		if findErr != nil {
			return
		}
		el.MustClick()
		p.page.MustWaitStable()
		clicked = true
	})
	if err != nil {
		return false, err
	}
	return clicked, nil
}

func (p *rodPage) clickVisibleNodeByText(selector string, pattern string) (bool, error) {
	clicked := false
	err := rodTry(func() {
		clicked = p.page.MustEval(`(selector, pattern) => {
			const regex = new RegExp(pattern);
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const fire = (node, type) => node.dispatchEvent(new MouseEvent(type, { bubbles: true, cancelable: true, view: window, button: 0 }));
			const nodes = Array.from(document.querySelectorAll(selector)).filter(isVisible);
			for (const node of nodes) {
				const text = (node.innerText || node.textContent || '').replace(/\s+/g, ' ').trim();
				if (!regex.test(text)) continue;
				node.scrollIntoView({ block: 'center', behavior: 'instant' });
				fire(node, 'mousedown');
				fire(node, 'mouseup');
				node.click();
				return true;
			}
			return false;
		}`, selector, pattern).Bool()
		if clicked {
			p.page.MustWaitStable()
		}
	})
	if err != nil {
		return false, err
	}
	return clicked, nil
}

func (p *rodPage) openPermissionDropdown() error {
	if p.isOnlySelfPermissionSelected() {
		return nil
	}
	if p.isPermissionDropdownVisible() {
		return nil
	}
	trigger, err := p.findPermissionTrigger()
	if err != nil {
		p.debugPermissionState()
		return fmt.Errorf("permission dropdown trigger not found")
	}
	if err := rodTry(func() {
		trigger.MustScrollIntoView().MustWaitVisible().MustClick()
		p.page.MustWaitStable()
	}); err != nil {
		return err
	}
	if p.isOnlySelfPermissionSelected() || p.isPermissionDropdownVisible() {
		return nil
	}
	if err := p.openPermissionDropdownFallback(trigger); err != nil {
		return err
	}
	if err := p.waitForPermissionDropdown(2 * time.Second); err != nil {
		p.debugPermissionState()
		return err
	}
	return nil
}

func (p *rodPage) isPermissionDropdownVisible() bool {
	visible := false
	_ = rodTry(func() {
		visible = p.page.MustEval(`() => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const overlayVisible = Array.from(document.querySelectorAll('.d-popover.d-dropdown, .d-dropdown, [role="listbox"], [data-popper-placement]')).some((node) => {
				if (!isVisible(node)) {
					return false;
				}
				const text = normalize(node.innerText || node.textContent || '');
				return /仅自己可见|公开可见/.test(text) || node.matches('[role="listbox"], [data-popper-placement]');
			});
			if (overlayVisible) {
				return true;
			}
			const visibleTexts = Array.from(document.querySelectorAll('div, span, label, button')).filter(isVisible).map((node) => normalize(node.innerText || node.textContent || ''));
			return visibleTexts.includes('公开可见') && visibleTexts.includes('仅自己可见');
		}`).Bool()
	})
	return visible
}

func (p *rodPage) isOnlySelfPermissionSelected() bool {
	selected := false
	_ = rodTry(func() {
		selected = p.page.MustEval(`() => {
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const nodes = Array.from(document.querySelectorAll('.permission-card-wrapper .d-select-description, .permission-card-wrapper .name, .permission-card-wrapper .d-text, .permission-card-select .d-select-description')).filter(isVisible);
			return nodes.some((node) => (node.innerText || node.textContent || '').replace(/\s+/g, ' ').trim() === '仅自己可见');
		}`).Bool()
	})
	return selected
}

func (p *rodPage) findPermissionTrigger() (*rod.Element, error) {
	card, err := p.permissionCard()
	if err != nil {
		return nil, err
	}
	for _, selector := range permissionTriggerSelectors {
		var element *rod.Element
		if err := rodTry(func() {
			elements := card.MustElements(selector)
			for _, candidate := range elements {
				if !candidate.MustVisible() {
					continue
				}
				element = candidate
				return
			}
		}); err != nil {
			return nil, err
		}
		if element != nil {
			return element, nil
		}
	}
	var element *rod.Element
	if err := rodTry(func() {
		elements := card.MustElements("div, span, label, button")
		for _, candidate := range elements {
			if !candidate.MustVisible() {
				continue
			}
			text := strings.TrimSpace(candidate.MustText())
			if text != "公开可见" && text != "仅自己可见" {
				continue
			}
			element = candidate
			return
		}
	}); err != nil {
		return nil, err
	}
	if element != nil {
		return element, nil
	}
	return nil, fmt.Errorf("permission dropdown trigger not found")
}

func (p *rodPage) permissionCard() (*rod.Element, error) {
	var card *rod.Element
	if err := rodTry(func() {
		cards := p.page.MustElements(".permission-card-wrapper")
		for _, candidate := range cards {
			if !candidate.MustVisible() {
				continue
			}
			card = candidate
			return
		}
	}); err != nil {
		return nil, err
	}
	if card == nil {
		return nil, fmt.Errorf("permission card not found")
	}
	return card, nil
}

func (p *rodPage) openPermissionDropdownFallback(trigger *rod.Element) error {
	return rodTry(func() {
		trigger.MustEval(`() => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const isDropdownVisible = () => {
				const overlayVisible = Array.from(document.querySelectorAll('.d-popover.d-dropdown, .d-dropdown, [role="listbox"], [data-popper-placement]')).some((node) => {
					if (!isVisible(node)) {
						return false;
					}
					const text = normalize(node.innerText || node.textContent || '');
					return /仅自己可见|公开可见/.test(text) || node.matches('[role="listbox"], [data-popper-placement]');
				});
				if (overlayVisible) {
					return true;
				}
				const visibleTexts = Array.from(document.querySelectorAll('div, span, label, button')).filter(isVisible).map((node) => normalize(node.innerText || node.textContent || ''));
				return visibleTexts.includes('公开可见') && visibleTexts.includes('仅自己可见');
			};
			const firePointer = (node, type) => {
				if (typeof PointerEvent === 'function') {
					node.dispatchEvent(new PointerEvent(type, {
						bubbles: true,
						cancelable: true,
						view: window,
						button: 0,
						pointerType: 'mouse',
						isPrimary: true,
					}));
				}
			};
			const fireMouseEvent = (node, type) => {
				node.dispatchEvent(new MouseEvent(type, {
					bubbles: true,
					cancelable: true,
					view: window,
					button: 0,
				}));
			};
			const node = this;
			node.scrollIntoView({ block: 'center', behavior: 'instant' });
			firePointer(node, 'pointerdown');
			if (!isDropdownVisible()) {
				fireMouseEvent(node, 'mousedown');
			}
			if (!isDropdownVisible()) {
				firePointer(node, 'pointerup');
				fireMouseEvent(node, 'mouseup');
			}
			if (!isDropdownVisible()) {
				fireMouseEvent(node, 'click');
			}
			if (!isDropdownVisible() && typeof node.focus === 'function') {
				node.focus();
			}
		}`)
		p.page.MustWaitStable()
	})
}

func (p *rodPage) waitForPermissionDropdown(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.isPermissionDropdownVisible() || p.isOnlySelfPermissionSelected() {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("permission dropdown did not become visible")
}

func (p *rodPage) selectPermissionOption(text string) error {
	dropdown, err := p.visiblePermissionDropdown()
	if err != nil {
		if p.isOnlySelfPermissionSelected() {
			return nil
		}
		return err
	}
	for _, selector := range permissionOptionSelectors {
		var found bool
		if err := rodTry(func() {
			elements := dropdown.MustElements(selector)
			for _, candidate := range elements {
				if !candidate.MustVisible() {
					continue
				}
				current := strings.TrimSpace(candidate.MustText())
				if current != text {
					continue
				}
				clickable := p.permissionOptionClickable(candidate, text)
				if clickable == nil {
					continue
				}
				clickable.MustScrollIntoView().MustWaitVisible().MustClick()
				p.page.MustWaitStable()
				found = true
				return
			}
		}); err != nil {
			return err
		}
		if found {
			if p.isOnlySelfPermissionSelected() {
				return nil
			}
			break
		}
	}
	if p.isOnlySelfPermissionSelected() {
		return nil
	}
	return fmt.Errorf("permission option %q not found in visible dropdown", text)
}

func (p *rodPage) visiblePermissionDropdown() (*rod.Element, error) {
	var dropdown *rod.Element
	if err := rodTry(func() {
		elements := p.page.MustElements(".d-popover.d-dropdown, .d-dropdown, [role='listbox'], [data-popper-placement]")
		for _, candidate := range elements {
			if !candidate.MustVisible() {
				continue
			}
			text := strings.TrimSpace(candidate.MustText())
			if !strings.Contains(text, "公开可见") && !strings.Contains(text, "仅自己可见") {
				continue
			}
			dropdown = candidate
			return
		}
	}); err != nil {
		return nil, err
	}
	if dropdown == nil {
		return nil, fmt.Errorf("permission dropdown not found")
	}
	return dropdown, nil
}

func (p *rodPage) permissionOptionClickable(candidate *rod.Element, text string) *rod.Element {
	current := candidate
	for i := 0; i < 4 && current != nil; i++ {
		if current.MustVisible() {
			currentText := strings.TrimSpace(current.MustText())
			if currentText == text || strings.Contains(currentText, text) {
				return current
			}
		}
		parent, err := current.Parent()
		if err != nil || parent == nil {
			break
		}
		current = parent
	}
	if candidate.MustVisible() {
		return candidate
	}
	return nil
}

func (p *rodPage) setCheckboxState(labelNeedle string, checked bool) error {
	applied := false
	err := rodTry(func() {
		applied = p.page.MustEval(`(labelNeedle, checked) => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const wantChecked = checked === true || checked === 'true';
			const nodes = Array.from(document.querySelectorAll("input[type='checkbox']"));
			for (const node of nodes) {
				let current = node;
				let label = '';
				for (let i = 0; i < 8 && current; i++) {
					const text = normalize(current.innerText || current.textContent || '');
					if (text) {
						label = text;
						break;
					}
					current = current.parentElement;
				}
				if (!label.includes(labelNeedle)) {
					continue;
				}
				node.scrollIntoView({ block: 'center', behavior: 'instant' });
				if (!!node.checked !== wantChecked) {
					node.click();
				}
				if (!!node.checked !== wantChecked) {
					node.checked = wantChecked;
					node.dispatchEvent(new Event('input', { bubbles: true }));
					node.dispatchEvent(new Event('change', { bubbles: true }));
				}
				return !!node.checked === wantChecked;
			}
			return false;
		}`, labelNeedle, checked).Bool()
	})
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("checkbox containing %q not found or not updated", labelNeedle)
	}
	p.page.MustWaitStable()
	return nil
}

func (p *rodPage) applyOriginalDeclaration(enabled bool) error {
	if !enabled {
		return nil
	}
	entryClicked, err := p.clickVisibleNodeByText(".d-checkbox-main-label, .d-checkbox-label, .original-entry, div, span, label, button", "^声明原创$|^原创声明$|^原创$")
	if err != nil {
		return err
	}
	if !entryClicked {
		if err := p.setCheckboxState("声明原创", true); err != nil {
			if err := p.setCheckboxState("原创", true); err != nil {
				return err
			}
		}
	}
	if err := p.setCheckboxState("我已阅读并同意", true); err != nil {
		if !strings.Contains(err.Error(), `checkbox containing "我已阅读并同意" not found`) {
			return err
		}
	}
	clicked, err := p.clickByText("button", "声明原创")
	if err != nil {
		return err
	}
	if !clicked {
		return nil
	}
	return nil
}

func (p *rodPage) applyContentCopyPreference(allow bool) error {
	return p.setCheckboxState("允许正文复制", allow)
}

func (p *rodPage) setOnlySelfVisible() error {
	if err := p.openPermissionDropdown(); err != nil {
		return err
	}
	if err := p.selectPermissionOption("仅自己可见"); err != nil {
		return err
	}
	return nil
}

func (p *rodPage) waitForBodyText(ctx context.Context, pattern string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		matched := false
		if err := rodTry(func() {
			matched = p.page.MustEval(`(pattern) => {
				const text = document.body ? document.body.innerText : '';
				return new RegExp(pattern).test(text);
			}`, pattern).Bool()
		}); err == nil && matched {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("body text did not match %q", pattern)
}

func (p *rodPage) isPublishSuccessState() bool {
	success := false
	_ = rodTry(func() {
		success = p.page.MustEval(`() => {
			const href = window.location.href || '';
			if (href.includes('published=true')) return true;
			if (href.includes('/new/note-manager')) return true;

			const text = document.body ? document.body.innerText : '';
			if (/笔记管理|草稿箱/.test(text)) return true;

			const hasPublishEntry = /上传视频|上传图文|写长文/.test(text);
			const hasTitleInput = !!document.querySelector('input[placeholder="填写标题会有更多赞哦"], input[placeholder*="填写标题"], input[placeholder*="标题"]');
			const hasContentEditor = !!document.querySelector('div.tiptap.ProseMirror[contenteditable="true"], div.ProseMirror[contenteditable="true"], div[contenteditable="true"][role="textbox"]');
			const buttons = Array.from(document.querySelectorAll('button')).map((el) => (el.innerText || el.textContent || '').trim());
			const hasPublishButton = buttons.some((text) => /发布/.test(text));

			return hasPublishEntry && !hasTitleInput && !hasContentEditor && !hasPublishButton;
		}`).Bool()
	})
	return success
}

func (p *rodPage) waitForPublishedRedirect(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if p.isPublishSuccessState() {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("published redirect not observed")
}

func (p *rodPage) selectRecommendedTopic(tag string) error {
	clicked := false
	err := rodTry(func() {
		clicked = p.page.MustEval(`(rawTag) => {
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
			const target = '#' + normalize(rawTag);
			const options = Array.from(document.querySelectorAll('.recommend-topic-wrapper .tag-group .tag')).filter(isVisible);
			for (const option of options) {
				const text = normalize(option.innerText || option.textContent || '');
				if (text !== target) {
					continue;
				}
				option.scrollIntoView({ block: 'center', behavior: 'instant' });
				fireMouseEvent(option, 'mousedown');
				fireMouseEvent(option, 'mouseup');
				option.click();
				return true;
			}
			return false;
		}`, tag).Bool()
		if clicked {
			p.page.MustWaitStable()
		}
	})
	if err != nil {
		return err
	}
	if !clicked {
		return fmt.Errorf("recommended topic %q not found", tag)
	}
	closed := false
	err = rodTry(func() {
		closed = p.page.MustEval(`() => {
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const roots = Array.from(document.querySelectorAll('#tippy-1, [data-tippy-root], .tippy-box, .tippy-content')).filter(isVisible);
			const items = Array.from(document.querySelectorAll('#creator-editor-topic-container, [id^="creator-editor-topic-container"]')).filter(isVisible);
			return roots.length === 0 || items.length === 0;
		}`).Bool()
	})
	if err != nil {
		return err
	}
	if !closed {
		clickedBody, clickErr := p.clickVisibleNodeByText("body", ".*")
		if clickErr != nil {
			return clickErr
		}
		if clickedBody {
			err = rodTry(func() {
				closed = p.page.MustEval(`() => {
					const isVisible = (node) => {
						if (!node) return false;
						const rect = node.getBoundingClientRect();
						const style = window.getComputedStyle(node);
						return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
					};
					const roots = Array.from(document.querySelectorAll('#tippy-1, [data-tippy-root], .tippy-box, .tippy-content')).filter(isVisible);
					const items = Array.from(document.querySelectorAll('#creator-editor-topic-container, [id^="creator-editor-topic-container"]')).filter(isVisible);
					return roots.length === 0 || items.length === 0;
				}`).Bool()
			})
			if err != nil {
				return err
			}
		}
	}
	if !closed {
		return fmt.Errorf("topic popup did not close after selecting %q", tag)
	}
	p.page.MustWaitStable()
	return nil
}

func (p *rodPage) waitForUploadInput(ctx context.Context, timeout time.Duration) (*rod.Element, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		input, err := p.elementBySelectors(uploadInputSelectors)
		if err == nil {
			return input, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	p.debugUploadInputState()
	return nil, fmt.Errorf("element not found for selectors: %s", strings.Join(uploadInputSelectors, ", "))
}

func (p *rodPage) debugUploadInputState() {
	_ = rodTry(func() {
		currentURL := p.page.MustEval(`() => location.href`).String()
		defaultXHSLogger("upload input missing url=%s", currentURL)
	})
	_ = rodTry(func() {
		count := p.page.MustEval(`() => document.querySelectorAll('input[type="file"]').length`).Int()
		defaultXHSLogger("upload input missing fileInputCount=%d", count)
	})
	_ = rodTry(func() {
		snippets := p.page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('input[type="file"]')).slice(0, 5).map((el) => el.outerHTML))`).String()
		defaultXHSLogger("upload input missing fileInputs=%s", snippets)
	})
	_ = rodTry(func() {
		text := p.page.MustEval(`() => {
			const body = document.body ? document.body.innerText : '';
			return body.replace(/\s+/g, ' ').trim().slice(0, 500);
		}`).String()
		defaultXHSLogger("upload input missing body=%s", text)
	})
}

func (p *rodPage) debugPermissionState() {
	_ = rodTry(func() {
		text := p.page.MustEval(`() => {
			const body = document.body ? document.body.innerText : '';
			return body.replace(/\s+/g, ' ').trim().slice(0, 1500);
		}`).String()
		defaultXHSLogger("permission state body=%s", text)
	})
	_ = rodTry(func() {
		nodes := p.page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('.permission-card-wrapper, .permission-card-select, .d-popover.d-dropdown, .d-dropdown, .custom-option, .group-info .name')).map((el) => {
			const text = (el.innerText || el.textContent || '').replace(/\s+/g, ' ').trim();
			const rect = el.getBoundingClientRect();
			const style = window.getComputedStyle(el);
			return {
				text,
				className: el.className || '',
				visible: rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden',
				outerHTML: (el.outerHTML || '').slice(0, 260),
			};
		})).slice(0, 40))`).String()
		defaultXHSLogger("permission state nodes=%s", nodes)
	})
}

func (p *rodPage) debugPublishConfirmationState() {
	_ = rodTry(func() {
		currentURL := p.page.MustEval(`() => location.href`).String()
		defaultXHSLogger("publish confirmation missing url=%s", currentURL)
	})
	_ = rodTry(func() {
		buttons := p.page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('button')).slice(0, 12).map((el) => (el.innerText || el.textContent || '').replace(/\s+/g, ' ').trim()).filter(Boolean))`).String()
		defaultXHSLogger("publish confirmation missing buttons=%s", buttons)
	})
	_ = rodTry(func() {
		text := p.page.MustEval(`() => {
			const body = document.body ? document.body.innerText : '';
			return body.replace(/\s+/g, ' ').trim().slice(0, 1000);
		}`).String()
		defaultXHSLogger("publish confirmation missing body=%s", text)
	})
}

func (p *rodPage) dismissEditorOverlays() error {
	return rodTry(func() {
		p.page.MustEval(`() => {
			if (document.activeElement && typeof document.activeElement.blur === 'function') {
				document.activeElement.blur();
			}
			window.scrollTo({ top: document.body.scrollHeight, behavior: 'instant' });
		}`)
		p.page.MustWaitStable()
		body := p.page.MustElement("body")
		body.MustClick()
		p.page.MustWaitStable()
	})
}

func (p *rodPage) waitForEditorReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		titleReady := false
		contentReady := false
		_ = rodTry(func() {
			titleReady = p.page.MustEval(`(selectors) => selectors.some((selector) => {
				const el = document.querySelector(selector);
				return !!el && !el.disabled;
			})`, titleSelectors).Bool()
		})
		_ = rodTry(func() {
			contentReady = p.page.MustEval(`(selectors) => selectors.some((selector) => {
				const el = document.querySelector(selector);
				return !!el && el.getAttribute('contenteditable') === 'true';
			})`, contentSelectors).Bool()
		})
		if titleReady && contentReady {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("editor not ready after upload")
}

func (p *rodPage) elementBySelectors(selectors []string) (*rod.Element, error) {
	for _, selector := range selectors {
		var element *rod.Element
		if err := rodTry(func() {
			element = p.page.Timeout(2 * time.Second).MustElement(selector)
		}); err == nil {
			return element, nil
		}
	}
	return nil, fmt.Errorf("element not found for selectors: %s", strings.Join(selectors, ", "))
}
