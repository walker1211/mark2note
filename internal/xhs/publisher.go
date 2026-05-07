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
	SaveDraft(ctx context.Context) error
	ConfirmDraftSaved(ctx context.Context) error
	SetSchedule(ctx context.Context, at time.Time) error
	SubmitScheduled(ctx context.Context) error
	ConfirmScheduledSubmitted(ctx context.Context) error
}

type Publisher struct{}

func (Publisher) PublishStandardDraft(ctx context.Context, page PublishPage, request PublishRequest) error {
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
	if err := page.SaveDraft(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrSubmitFailed, err)
	}
	if err := page.ConfirmDraftSaved(ctx); err != nil {
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
	if len(tags) > 0 {
		tagParts := make([]string, 0, len(tags))
		for _, tag := range tags {
			trimmed := strings.TrimSpace(tag)
			if trimmed == "" {
				continue
			}
			tagParts = append(tagParts, "#"+trimmed)
		}
		if len(tagParts) > 0 {
			text = text + "\n" + strings.Join(tagParts, " ")
		}
	}
	return rodTry(func() {
		field.MustInput(text)
	})
}

func (p *rodPage) SaveDraft(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish dismiss editor overlays")
	if err := p.dismissEditorOverlays(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish disable content copy")
	if err := p.disableContentCopy(); err != nil {
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

func (p *rodPage) ConfirmDraftSaved(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	defaultXHSLogger("only-self publish wait confirmation")
	if err := p.waitForBodyText(ctx, `发布成功|提交成功|保存成功|笔记保存成功|笔记发布成功`, 15*time.Second); err != nil {
		p.debugPublishConfirmationState()
		return fmt.Errorf("only-self publish confirmation not observed")
	}
	return nil
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
		p.page.MustElementR("button", "定时发布|确认发布").MustClick()
		p.page.MustWaitStable()
	})
}

func (p *rodPage) ConfirmScheduledSubmitted(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.waitForBodyText(ctx, `发布成功|提交成功|预约成功`, 15*time.Second); err != nil {
		return fmt.Errorf("scheduled publish confirmation not observed")
	}
	return nil
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

func (p *rodPage) openPermissionDropdown() error {
	opened := false
	err := rodTry(func() {
		opened = p.page.MustEval(`() => {
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const settings = document.querySelector('.publish-page-content-settings, .publish-page-content-settings-content');
			if (settings) {
				settings.scrollIntoView({ block: 'center', behavior: 'instant' });
			}
			if (document.activeElement && typeof document.activeElement.blur === 'function') {
				document.activeElement.blur();
			}
			document.body && document.body.click();
			const trigger = document.querySelector('.permission-card-wrapper .d-select-wrapper.permission-card-select') || document.querySelector('.permission-card-select');
			if (!isVisible(trigger)) {
				return false;
			}
			trigger.scrollIntoView({ block: 'center', behavior: 'instant' });
			trigger.click();
			return true;
		}`).Bool()
		if opened {
			p.page.MustWaitStable()
		}
	})
	if err != nil {
		return err
	}
	if !opened {
		p.debugPermissionState()
		return fmt.Errorf("permission dropdown trigger not found")
	}
	if err := p.waitForPermissionDropdown(5 * time.Second); err != nil {
		p.debugPermissionState()
		return err
	}
	return nil
}

func (p *rodPage) waitForPermissionDropdown(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		visible := false
		_ = rodTry(func() {
			visible = p.page.MustEval(`() => {
				const isVisible = (node) => {
					if (!node) return false;
					const rect = node.getBoundingClientRect();
					const style = window.getComputedStyle(node);
					return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
				};
				return Array.from(document.querySelectorAll('.d-popover.d-dropdown, .d-dropdown')).some(isVisible);
			}`).Bool()
		})
		if visible {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("permission dropdown did not become visible")
}

func (p *rodPage) selectPermissionOption(text string) error {
	clicked := false
	err := rodTry(func() {
		clicked = p.page.MustEval(`(targetText) => {
			const normalize = (value) => (value || '').replace(/\s+/g, ' ').trim();
			const isVisible = (node) => {
				if (!node) return false;
				const rect = node.getBoundingClientRect();
				const style = window.getComputedStyle(node);
				return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
			};
			const dropdowns = Array.from(document.querySelectorAll('.d-popover.d-dropdown, .d-dropdown')).filter(isVisible);
			for (const dropdown of dropdowns) {
				const options = Array.from(dropdown.querySelectorAll('.custom-option, .d-grid-item, .name'));
				for (const option of options) {
					const current = normalize(option.innerText || option.textContent || '');
					if (current !== targetText) {
						continue;
					}
					const clickable = option.closest('.custom-option') || option.closest('.d-grid-item') || option;
					if (!isVisible(clickable)) {
						continue;
					}
					clickable.scrollIntoView({ block: 'center', behavior: 'instant' });
					clickable.click();
					return true;
				}
			}
			return false;
		}`, text).Bool()
		if clicked {
			p.page.MustWaitStable()
		}
	})
	if err != nil {
		return err
	}
	if !clicked {
		return fmt.Errorf("permission option %q not found in visible dropdown", text)
	}
	return nil
}

func (p *rodPage) disableContentCopy() error {
	return rodTry(func() {
		nodes := p.page.MustElements("input[type='checkbox']")
		for _, node := range nodes {
			label := strings.TrimSpace(node.MustEval(`() => {
				let current = this;
				for (let i = 0; i < 4 && current; i++) {
					const text = (current.innerText || current.textContent || '').replace(/\s+/g, ' ').trim();
					if (text) {
						return text;
					}
					current = current.parentElement;
				}
				return '';
			}`).String())
			if !strings.Contains(label, "允许正文复制") {
				continue
			}
			node.MustEval(`() => this.scrollIntoView({ block: 'center', behavior: 'instant' })`)
			p.page.MustWaitStable()
			checked := node.MustProperty("checked").Bool()
			if checked {
				node.MustEval(`() => this.click()`)
				p.page.MustWaitStable()
			}
			return
		}
	})
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
