package xhs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

var (
	ErrUploadFailed       = errors.New("upload failed")
	ErrUploadInputMissing = errors.New("upload input not found")
	ErrFillFailed         = errors.New("fill failed")
	ErrScheduleFailed     = errors.New("schedule failed")
	ErrSubmitFailed       = errors.New("submit failed")
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
		return fmt.Errorf("%w: %w", ErrUploadFailed, err)
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
		field.MustInput(NormalizePublishTitle(title))
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
	text, topicTags := composePublishContent(content, tags)
	if text != "" {
		if err := rodTry(func() {
			field.MustInput(text)
		}); err != nil {
			return err
		}
	}
	if text != "" && len(topicTags) > 0 {
		if err := rodTry(func() {
			field.MustInput("\n")
		}); err != nil {
			return err
		}
	}
	for _, tag := range topicTags {
		if err := p.inputTopicByKeyboard(field, tag); err != nil {
			return fmt.Errorf("input topic %q: %w", tag, err)
		}
	}
	return nil
}

func (p *rodPage) inputTopicByKeyboard(field *rod.Element, tag string) error {
	if err := rodTry(func() {
		field.MustClick()
	}); err != nil {
		return fmt.Errorf("click editor: %w", err)
	}
	if err := p.typeTopicTrigger(); err != nil {
		return fmt.Errorf("type topic trigger: %w", err)
	}
	if err := p.waitForTopicSuggestion("", 2*time.Second); err != nil {
		return err
	}
	if err := rodTry(func() {
		p.page.MustInsertText(tag)
	}); err != nil {
		return fmt.Errorf("type topic text: %w", err)
	}
	if err := p.waitForTopicSuggestion(tag, 2*time.Second); err != nil {
		return err
	}

	var lastErr error
	for i := 0; i < 2; i++ {
		if err := p.pressSpaceKey(); err != nil {
			return fmt.Errorf("press space: %w", err)
		}
		if err := p.waitForTopicConfirmation(tag, 1200*time.Millisecond); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i == 0 && p.waitForTopicSuggestion(tag, 300*time.Millisecond) != nil {
			break
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("topic %q suggestion disappeared before Xiaohongshu converted it into a highlighted topic", tag)
}

func (p *rodPage) typeTopicTrigger() error {
	if err := (&proto.InputDispatchKeyEvent{
		Type:                  proto.InputDispatchKeyEventTypeKeyDown,
		Modifiers:             input.ModifierShift,
		WindowsVirtualKeyCode: 51,
		Code:                  "Digit3",
		Key:                   "#",
		Text:                  "#",
		UnmodifiedText:        "#",
	}).Call(p.page); err != nil {
		return err
	}
	return (&proto.InputDispatchKeyEvent{
		Type:                  proto.InputDispatchKeyEventTypeKeyUp,
		Modifiers:             input.ModifierShift,
		WindowsVirtualKeyCode: 51,
		Code:                  "Digit3",
		Key:                   "#",
	}).Call(p.page)
}

func (p *rodPage) pressSpaceKey() error {
	if err := p.page.Keyboard.Press(input.Space); err != nil {
		return err
	}
	time.Sleep(80 * time.Millisecond)
	return p.page.Keyboard.Release(input.Space)
}

func (p *rodPage) waitForTopicSuggestion(tag string, timeout time.Duration) error {
	want := "#" + tag
	if tag == "" {
		want = "#"
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		matched := false
		if err := rodTry(func() {
			matched = p.page.MustEval(`(want) => {
				const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
				const editors = Array.from(document.querySelectorAll('div.tiptap.ProseMirror[contenteditable="true"], div.ProseMirror[contenteditable="true"], div[contenteditable="true"][role="textbox"]'));
				return editors.some((editor) => Array.from(editor.querySelectorAll('.suggestion')).some((node) => normalize(node.textContent) === want));
			}`, want).Bool()
		}); err == nil && matched {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("topic %q did not enter Xiaohongshu suggestion mode", tag)
}

func (p *rodPage) waitForTopicConfirmation(tag string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		confirmed := false
		if err := rodTry(func() {
			confirmed = p.page.MustEval(`(tag) => {
				const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
				const editors = Array.from(document.querySelectorAll('div.tiptap.ProseMirror[contenteditable="true"], div.ProseMirror[contenteditable="true"], div[contenteditable="true"][role="textbox"]'));
				return editors.some((editor) => Array.from(editor.querySelectorAll('a.tiptap-topic[data-topic]')).some((node) => {
					const text = normalize(node.innerText || node.textContent || '').replace(/\[话题\]#$/, '');
					if (text !== '#' + tag) return false;
					try {
						const data = JSON.parse(node.getAttribute('data-topic') || '{}');
						return data.name === tag;
					} catch (_) {
						return true;
					}
				}));
			}`, tag).Bool()
		}); err == nil && confirmed {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("topic %q was typed as plain text but Xiaohongshu did not convert it into a highlighted topic", tag)
}

func composePublishContent(content string, tags []string) (string, []string) {
	text := strings.TrimSpace(content)
	topicTags := make([]string, 0, len(tags))
	existingTags := existingHashtagTokens(text)
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "#")
		if trimmed == "" || existingTags[trimmed] {
			continue
		}
		topicTags = append(topicTags, trimmed)
		existingTags[trimmed] = true
	}
	return text, topicTags
}

func existingHashtagTokens(text string) map[string]bool {
	result := map[string]bool{}
	for _, field := range strings.Fields(text) {
		trimmed := strings.Trim(field, "，。；;：:、,.!?！？()（）[]【】{}《》\"'")
		if strings.HasPrefix(trimmed, "#") && len([]rune(trimmed)) > 1 {
			result[strings.TrimPrefix(trimmed, "#")] = true
		}
	}
	return result
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
	if err := p.waitForPublishConfirmation(ctx, `发布成功|提交成功|笔记发布成功`, 15*time.Second); err == nil {
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
		return fmt.Errorf("%w: %w", ErrUploadFailed, err)
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
	if err := p.waitForPublishConfirmation(ctx, `发布成功|提交成功|预约成功`, 15*time.Second); err == nil {
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
	if applied, err := p.clickVisibleCheckboxControl(labelNeedle, checked); err != nil {
		return err
	} else if applied {
		return nil
	}

	applied := false
	err := rodTry(func() {
		applied = p.page.MustEval(`(labelNeedle, checked) => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const wantChecked = checked === true || checked === 'true';
			const isVisible = (node) => {
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const labelFor = (input) => {
				const explicitLabel = input.closest('label');
				const explicitText = normalize(explicitLabel && (explicitLabel.innerText || explicitLabel.textContent || ''));
				if (explicitText) return explicitText;
				const nextText = normalize(input.nextElementSibling && (input.nextElementSibling.innerText || input.nextElementSibling.textContent || ''));
				if (nextText) return nextText;
				const previousText = normalize(input.previousElementSibling && (input.previousElementSibling.innerText || input.previousElementSibling.textContent || ''));
				if (previousText) return previousText;
				let current = input.parentElement;
				for (let i = 0; i < 8 && current && current !== document.body && current !== document.documentElement; i++) {
					const text = normalize(current.innerText || current.textContent || '');
					if (text) return text;
					current = current.parentElement;
				}
				return '';
			};
			const setNativeChecked = (node) => {
				const descriptor = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'checked');
				if (descriptor && descriptor.set) {
					descriptor.set.call(node, wantChecked);
				} else {
					node.checked = wantChecked;
				}
				node.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window, button: 0 }));
				node.dispatchEvent(new Event('input', { bubbles: true }));
				node.dispatchEvent(new Event('change', { bubbles: true }));
			};
			const findClickable = (input) => {
				if (input.nextElementSibling && isVisible(input.nextElementSibling)) return input.nextElementSibling;
				if (input.previousElementSibling && isVisible(input.previousElementSibling)) return input.previousElementSibling;
				let current = input;
				for (let i = 0; i < 8 && current; i++) {
					if (isVisible(current) && /checkbox|d-checkbox|original|原创|声明|同意|允许/.test((current.className || '') + ' ' + normalize(current.innerText || current.textContent || ''))) {
						return current;
					}
					current = current.parentElement;
				}
				return input;
			};
			const nodes = Array.from(document.querySelectorAll("input[type='checkbox']"));
			for (const node of nodes) {
				const label = labelFor(node);
				if (!label.includes(labelNeedle)) {
					continue;
				}
				node.scrollIntoView({ block: 'center', behavior: 'instant' });
				if (!!node.checked !== wantChecked) {
					findClickable(node).click();
				}
				if (!!node.checked !== wantChecked) {
					setNativeChecked(node);
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

func (p *rodPage) clickVisibleCheckboxControl(labelNeedle string, checked bool) (bool, error) {
	const attr = "data-mark2note-checkbox-target"
	status := ""
	err := rodTry(func() {
		status = p.page.MustEval(`(labelNeedle, checked, attr) => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const wantChecked = checked === true || checked === 'true';
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			document.querySelectorAll('[' + attr + ']').forEach((node) => node.removeAttribute(attr));
			const labelFor = (input) => {
				const explicitLabel = input.closest('label');
				const explicitText = normalize(explicitLabel && (explicitLabel.innerText || explicitLabel.textContent || ''));
				if (explicitText) return explicitText;
				const nextText = normalize(input.nextElementSibling && (input.nextElementSibling.innerText || input.nextElementSibling.textContent || ''));
				if (nextText) return nextText;
				const previousText = normalize(input.previousElementSibling && (input.previousElementSibling.innerText || input.previousElementSibling.textContent || ''));
				if (previousText) return previousText;
				let current = input.parentElement;
				for (let i = 0; i < 8 && current && current !== document.body && current !== document.documentElement; i++) {
					const text = normalize(current.innerText || current.textContent || '');
					if (text) return text;
					current = current.parentElement;
				}
				return '';
			};
			const targetFor = (input) => {
				if (isVisible(input.nextElementSibling)) return input.nextElementSibling;
				if (isVisible(input.previousElementSibling)) return input.previousElementSibling;
				let current = input.parentElement;
				for (let i = 0; i < 8 && current; i++) {
					if (isVisible(current)) return current;
					current = current.parentElement;
				}
				return input;
			};
			const nodes = Array.from(document.querySelectorAll("input[type='checkbox']"));
			for (const node of nodes) {
				if (!labelFor(node).includes(labelNeedle)) continue;
				if (!!node.checked === wantChecked) return 'already';
				const target = targetFor(node);
				target.setAttribute(attr, 'true');
				return 'click';
			}
			return 'missing';
		}`, labelNeedle, checked, attr).String()
	})
	if err != nil {
		return false, err
	}
	if status == "already" {
		return true, nil
	}
	if status != "click" {
		return false, nil
	}
	if err := rodTry(func() {
		p.page.MustElement("[" + attr + `="true"]`).MustScrollIntoView().MustWaitVisible().MustClick()
		p.page.MustWaitStable()
	}); err != nil {
		return false, err
	}
	if p.checkboxStateMatches(labelNeedle, checked) {
		return true, nil
	}
	if isOriginalDeclarationNeedle(labelNeedle) && p.hasOriginalDeclarationPrompt() {
		return true, nil
	}
	return false, nil
}

func (p *rodPage) checkboxStateMatches(labelNeedle string, checked bool) bool {
	matched := false
	_ = rodTry(func() {
		matched = p.page.MustEval(`(labelNeedle, checked) => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const wantChecked = checked === true || checked === 'true';
			const labelFor = (input) => {
				const explicitLabel = input.closest('label');
				const explicitText = normalize(explicitLabel && (explicitLabel.innerText || explicitLabel.textContent || ''));
				if (explicitText) return explicitText;
				const nextText = normalize(input.nextElementSibling && (input.nextElementSibling.innerText || input.nextElementSibling.textContent || ''));
				if (nextText) return nextText;
				const previousText = normalize(input.previousElementSibling && (input.previousElementSibling.innerText || input.previousElementSibling.textContent || ''));
				if (previousText) return previousText;
				let current = input.parentElement;
				for (let i = 0; i < 8 && current && current !== document.body && current !== document.documentElement; i++) {
					const text = normalize(current.innerText || current.textContent || '');
					if (text) return text;
					current = current.parentElement;
				}
				return '';
			};
			const nodes = Array.from(document.querySelectorAll("input[type='checkbox']"));
			for (const node of nodes) {
				if (labelFor(node).includes(labelNeedle)) {
					return !!node.checked === wantChecked;
				}
			}
			return false;
		}`, labelNeedle, checked).Bool()
	})
	return matched
}

func (p *rodPage) hasOriginalDeclarationPrompt() bool {
	matched := false
	_ = rodTry(func() {
		matched = p.page.MustEval(`() => {
			const text = document.body ? document.body.innerText : '';
			return /笔记完成原创声明后|我已阅读并同意/.test(text);
		}`).Bool()
	})
	return matched
}

func isOriginalDeclarationNeedle(labelNeedle string) bool {
	return strings.Contains(labelNeedle, "原创") || strings.Contains(labelNeedle, "声明")
}

func (p *rodPage) applyOriginalDeclaration(enabled bool) error {
	if !enabled {
		return nil
	}
	if p.isOriginalDeclared() {
		return nil
	}
	if err := p.setCheckboxState("声明原创", true); err != nil {
		if err := p.setCheckboxState("原创", true); err != nil {
			if _, clickErr := p.clickVisibleNodeByText(".d-checkbox-main-label, .d-checkbox-label, .original-entry, div, span, label, button", "^声明原创$|^原创声明$|^原创$"); clickErr != nil {
				p.debugOriginalDeclarationState()
				return clickErr
			}
		}
	}
	if err := p.setCheckboxState("我已阅读并同意", true); err != nil {
		if !strings.Contains(err.Error(), `checkbox containing "我已阅读并同意" not found`) {
			return err
		}
	}
	if clicked, err := p.clickByText("button", "声明原创"); err != nil {
		return err
	} else if clicked {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if p.isOriginalDeclared() {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if !p.isOriginalDeclared() {
		p.debugOriginalDeclarationState()
		return fmt.Errorf("original declaration was requested but not selected")
	}
	return nil
}

func (p *rodPage) isOriginalDeclared() bool {
	declared := false
	_ = rodTry(func() {
		declared = p.page.MustEval(`() => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const nodes = Array.from(document.querySelectorAll('input[type="checkbox"]'));
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
				if (/声明原创|原创声明|原创/.test(label) && node.checked === true) {
					return true;
				}
			}
			return Array.from(document.querySelectorAll('.checked, .is-checked, .d-checkbox-checked, [aria-checked="true"]')).some((node) => /声明原创|原创声明|原创/.test(normalize(node.innerText || node.textContent || '')));
		}`).Bool()
	})
	return declared
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

func (p *rodPage) waitForPublishConfirmation(ctx context.Context, pattern string, timeout time.Duration) error {
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
		if p.isPublishSuccessState() {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("publish confirmation not observed")
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
	return nil, fmt.Errorf("%w: element not found for selectors: %s", ErrUploadInputMissing, strings.Join(uploadInputSelectors, ", "))
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

func (p *rodPage) debugOriginalDeclarationState() {
	_ = rodTry(func() {
		text := p.page.MustEval(`() => {
			const body = document.body ? document.body.innerText : '';
			return body.replace(/\s+/g, ' ').trim().slice(0, 1500);
		}`).String()
		defaultXHSLogger("original declaration state body=%s", text)
	})
	_ = rodTry(func() {
		nodes := p.page.MustEval(`() => JSON.stringify(Array.from(document.querySelectorAll('input[type="checkbox"], .d-checkbox, .d-checkbox-main-label, .d-checkbox-label, .original-entry, label, button')).map((el) => {
			const text = (el.innerText || el.textContent || el.getAttribute('aria-label') || '').replace(/\s+/g, ' ').trim();
			const rect = el.getBoundingClientRect();
			const style = window.getComputedStyle(el);
			return {
				tag: el.tagName,
				text,
				className: el.className || '',
				checked: el.checked === true,
				visible: rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden',
			};
		}).filter((item) => /原创|声明|同意|阅读/.test(item.text) || item.tag === 'INPUT'))`).String()
		defaultXHSLogger("original declaration candidates=%s", nodes)
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
		if err := rodTry(func() {
			p.page.Timeout(2 * time.Second).MustElement(selector)
		}); err != nil {
			continue
		}
		var element *rod.Element
		if err := rodTry(func() {
			element = p.page.MustElement(selector)
		}); err == nil {
			return element, nil
		}
	}
	return nil, fmt.Errorf("element not found for selectors: %s", strings.Join(selectors, ", "))
}
