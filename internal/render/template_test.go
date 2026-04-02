package render

import (
	"strings"
	"testing"

	"github.com/walker1211/mark2note/internal/deck"
)

func TestCompiledPageTemplateCachesParsedTemplate(t *testing.T) {
	first, err := compiledPageTemplate()
	if err != nil {
		t.Fatalf("compiledPageTemplate() first call error = %v", err)
	}
	second, err := compiledPageTemplate()
	if err != nil {
		t.Fatalf("compiledPageTemplate() second call error = %v", err)
	}
	if first == nil || second == nil {
		t.Fatalf("compiledPageTemplate() returned nil template")
	}
	if first != second {
		t.Fatalf("compiledPageTemplate() should return cached template instance")
	}
}

func TestRenderHTMLIncludesBadgeCounterAndCTA(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	mustContain := []string{
		d.Pages[0].Meta.Badge,
		d.Pages[0].Meta.Counter,
		d.Pages[0].Meta.CTA,
		d.Pages[0].Content.Title,
		`class="header-row"`,
		`class="pill page-badge"`,
		`class="page-no page-counter"`,
		`class="cta-bar cover-cta"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLCoverUsesDynamicTitleAndSubtitle(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p01-cover",
		Variant: "cover",
		Meta:    deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "cta"},
		Content: deck.PageContent{Title: "自定义封面", Subtitle: "这是副标题"},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"自定义封面", "这是副标题", `class="cover-main"`, `class="title-xl cover-title"`, `class="subtitle cover-subtitle"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, "GitHub 官方邮箱") {
		t.Fatalf("html still contains hard-coded legacy title")
	}
}

func TestRenderHTMLCoverRendersResolvedAuthor(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ShowAuthor = true
	d.AuthorText = "@搁剑听风"

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{`class="cover-author"`, "@搁剑听风", `font-size: 24px;`, `class="cover-bottom-stack has-author"`, `class="divider cover-divider"`, `class="cta-bar cover-cta"`, `.cover-bottom-stack {`, `.cover-divider {`, `position: static;`, `width: 100%;`, `.cover-bottom-stack.has-author .cover-divider {`, `transform: translateY(-18px);`, `.cover-author {`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, ".cover-author {\n  position: absolute;") {
		t.Fatalf("html should not contain absolute-positioned cover author: %s", html)
	}
	dividerIndex := strings.Index(html, `class="divider cover-divider"`)
	ctaIndex := strings.Index(html, `class="cta-bar cover-cta"`)
	authorIndex := strings.Index(html, `class="cover-author"`)
	if !(dividerIndex >= 0 && ctaIndex > dividerIndex && authorIndex > ctaIndex) {
		t.Fatalf("cover bottom order should be divider -> cta -> author, got html: %s", html)
	}
}

func TestRenderHTMLCoverOmitsAuthorWhenShowAuthorFalse(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if strings.Contains(html, `class="cover-author"`) {
		t.Fatalf("html should not contain cover author: %s", html)
	}
	if strings.Contains(html, `class="cover-bottom-stack has-author"`) {
		t.Fatalf("html should not mark cover stack as has-author: %s", html)
	}
	if !strings.Contains(html, `.cover-bottom-stack.has-author .cover-divider {`) {
		t.Fatalf("html missing author-only divider rule: %s", html)
	}
}

func TestRenderHTMLUsesDeckLevelThemeForWarmPaper(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeWarmPaper
	d.Themes = deck.RegisteredThemes()

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, "--bg: #F7F1E8;") {
		t.Fatalf("html missing warm-paper vars")
	}
}

func TestRenderHTMLDefaultStillUsesOrangeGreenPerPageTokens(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	coverHTML, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() cover error = %v", err)
	}
	endingHTML, err := RenderPageHTML(d, d.Pages[len(d.Pages)-1])
	if err != nil {
		t.Fatalf("RenderPageHTML() ending error = %v", err)
	}
	if !strings.Contains(coverHTML, "--bg: #F7F3EC;") {
		t.Fatalf("cover html missing default orange vars")
	}
	if !strings.Contains(endingHTML, "--bg: #F2F7F2;") {
		t.Fatalf("ending html missing default green vars")
	}
}

func TestRenderHTMLQuoteUsesDynamicQuoteNoteAndTip(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p02-quote",
		Variant: "quote",
		Meta:    deck.PageMeta{Badge: "第 2 页", Counter: "2/3", Theme: "green", CTA: "cta"},
		Content: deck.PageContent{
			Title: "引用页",
			Quote: "真正重要的是动态内容。",
			Note:  "这里是补充说明。",
			Tip:   "这里是额外提示。",
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"引用页", "真正重要的是动态内容。", "这里是补充说明。", "这里是额外提示。", `class="cta-bar cta-strong"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	for _, legacy := range []string{"GitHub 官方邮箱", "以后这种“真外壳 + 假内容”只会更多", "这封邮件的 3 个破绽"} {
		if strings.Contains(html, legacy) {
			t.Fatalf("html still contains legacy hard-coded copy %q", legacy)
		}
	}
}

func TestRenderHTMLBulletsUsesDynamicItems(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p02-bullets",
		Variant: "bullets",
		Meta:    deck.PageMeta{Badge: "第 2 页", Counter: "2/3", Theme: "orange", CTA: "cta"},
		Content: deck.PageContent{Title: "三步法", Items: []string{"先筛重点", "再做分组", "最后压缩表达"}},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"先筛重点", "再做分组", "最后压缩表达"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, "这封邮件的 3 个破绽") {
		t.Fatalf("html still contains hard-coded legacy bullets title")
	}
}

func TestRenderHTMLCompareUsesDynamicLabelsAndRows(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p03-compare",
		Variant: "compare",
		Meta:    deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta"},
		Content: deck.PageContent{
			Title: "对比页",
			Compare: &deck.CompareBlock{
				LeftLabel:  "旧方式",
				RightLabel: "新方式",
				Rows: []deck.CompareRow{
					{Left: "先做视觉", Right: "先做结构"},
					{Left: "信息堆叠", Right: "结论先行"},
				},
			},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"旧方式", "新方式", "先做视觉", "先做结构", "信息堆叠", "结论先行"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, "表面看到的") || strings.Contains(html, "真正该看的") {
		t.Fatalf("html still contains hard-coded legacy compare labels")
	}
}

func TestRenderHTMLFailsWhenCompareContentMissing(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p03-compare",
		Variant: "compare",
		Meta:    deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta"},
		Content: deck.PageContent{Title: "对比页"},
	}

	_, err := RenderPageHTML(d, page)
	if err == nil {
		t.Fatalf("RenderPageHTML() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `variant "compare" requires compare content for page "p03-compare"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderHTMLGalleryStepsUsesDynamicStepsAndImages(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p04-gallery",
		Variant: "gallery-steps",
		Meta:    deck.PageMeta{Badge: "第 4 页", Counter: "4/4", Theme: "green", CTA: "cta"},
		Content: deck.PageContent{
			Title: "步骤演示",
			Steps: []string{"第一步：拆主题", "第二步：补图示"},
			Images: []deck.ImageBlock{
				{Src: "https://example.com/step1.png", Alt: "step1", Caption: "步骤一截图"},
				{Src: "https://example.com/step2.png", Alt: "step2", Caption: "步骤二截图"},
			},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"第一步：拆主题", "第二步：补图示", "步骤一截图", "步骤二截图", "https://example.com/step1.png", "https://example.com/step2.png", `alt="step1"`, `alt="step2"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, "遇到就反手三连") {
		t.Fatalf("html still contains hard-coded legacy gallery title")
	}
}

func TestRenderHTMLImageCaptionUsesDynamicContentAndPreservesProvidedImageAndBody(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p03-image-caption",
		Variant: "image-caption",
		Meta:    deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "orange", CTA: "cta"},
		Content: deck.PageContent{
			Title: "动态图文页",
			Body:  "这是一段自定义图注正文。",
			Images: []deck.ImageBlock{
				{Src: "https://example.com/custom-image.png", Alt: "custom-image", Caption: "自定义截图"},
			},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"动态图文页", "这是一段自定义图注正文。", "https://example.com/custom-image.png", `alt="custom-image"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	for _, unwanted := range []string{"data:image/svg", "这里将展示与正文内容相关的配图或截图。", "mail-shot.png", "domain-shot.png", "github-report-shot.png", "registrar-report-shot.png"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("html should preserve provided content without fallback %q", unwanted)
		}
	}
}

func TestRenderHTMLEndingUsesDynamicBody(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p03-ending",
		Variant: "ending",
		Meta:    deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "green", CTA: "cta"},
		Content: deck.PageContent{Title: "收尾", Body: "这是动态结尾正文"},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, "这是动态结尾正文") {
		t.Fatalf("html missing dynamic ending body")
	}
	if strings.Contains(html, "以后这种“真外壳 + 假内容”只会更多") {
		t.Fatalf("html still contains hard-coded legacy ending")
	}
}

func TestRenderHTMLIncludesWatermarkWhenEnabled(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ShowWatermark = true
	d.WatermarkText = "walker1211/mark2note"
	d.WatermarkPosition = "bottom-right"

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{
		`class="watermark watermark-bottom-right"`,
		">walker1211/mark2note<",
		`.watermark {`,
		`.watermark-bottom-right {`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Count(html, `class="watermark `) != 1 {
		t.Fatalf("html should render exactly one watermark, got %d", strings.Count(html, `class="watermark `))
	}
}

func TestRenderHTMLOmitsWatermarkWhenDisabled(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ShowWatermark = false
	d.WatermarkText = "walker1211/mark2note"
	d.WatermarkPosition = "bottom-right"

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if strings.Contains(html, `class="watermark `) {
		t.Fatalf("html should omit watermark when disabled: %s", html)
	}
}

func TestRenderHTMLUsesBottomLeftWatermarkClass(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ShowWatermark = true
	d.WatermarkText = "左下角水印"
	d.WatermarkPosition = "bottom-left"

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, `class="watermark watermark-bottom-left"`) {
		t.Fatalf("html missing bottom-left watermark class: %s", html)
	}
	if strings.Contains(html, `class="watermark watermark-bottom-right"`) {
		t.Fatalf("html should not use bottom-right class when bottom-left requested: %s", html)
	}
	if !strings.Contains(html, `.watermark-bottom-left {`) {
		t.Fatalf("html missing bottom-left css rule: %s", html)
	}
}

func TestRenderHTMLEmbedsUsableCSSInsteadOfZgotmplZ(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if strings.Contains(html, "ZgotmplZ") {
		t.Fatalf("html should not contain ZgotmplZ: %s", html)
	}
	mustContain := []string{
		":root {",
		"--bg: #F7F3EC;",
		"--author-color: #5E5A56;",
		".header-row {",
		".cta-bar {",
	}
	for _, want := range mustContain {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing css token %q", want)
		}
	}
}

func TestRenderHTMLUsesNeutralCTAShadowVariable(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[1])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"--cta-shadow: 0 18px 40px rgba(0, 0, 0, 0.12);", "box-shadow: var(--cta-shadow);"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing css token %q", want)
		}
	}
	if strings.Contains(html, "rgba(232, 91, 58, 0.28)") {
		t.Fatalf("html should not contain warm CTA shadow color: %s", html)
	}
}

func TestRenderHTMLSupportsAllCurrentVariants(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")

	markers := map[string]string{
		"cover":         `class="cover-main"`,
		"quote":         `class="card quote-card"`,
		"image-caption": `class="caption"`,
		"bullets":       `class="bullets"`,
		"compare":       `class="compare-rows"`,
		"gallery-steps": `class="steps`,
		"ending":        `class="ending-box"`,
	}

	for _, page := range d.Pages {
		html, err := RenderPageHTML(d, page)
		if err != nil {
			t.Fatalf("RenderPageHTML(%s) error = %v", page.Name, err)
		}
		marker, ok := markers[page.Variant]
		if !ok {
			t.Fatalf("unexpected variant %q", page.Variant)
		}
		if !strings.Contains(html, marker) {
			t.Fatalf("html for %s missing marker %q", page.Name, marker)
		}
	}
}

func TestRenderHTMLFailsOnUnknownTheme(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := d.Pages[0]
	page.Meta.Theme = "missing"

	_, err := RenderPageHTML(d, page)
	if err == nil {
		t.Fatalf("RenderPageHTML() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `unknown theme "missing"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderHTMLFailsOnUnknownVariant(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := d.Pages[0]
	page.Variant = "unknown"

	_, err := RenderPageHTML(d, page)
	if err == nil {
		t.Fatalf("RenderPageHTML() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `unsupported variant "unknown"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestRenderHTMLOmitsImageFrameWhenImageCaptionHasNoImages(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p09-extra-image",
		Variant: "image-caption",
		Meta: deck.PageMeta{
			Badge:   "第 9 页",
			Counter: "9/9",
			Theme:   "orange",
			CTA:     "cta",
		},
		Content: deck.PageContent{
			Title: "自定义图文页",
			Body:  "正文",
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if strings.Contains(html, `class="image-frame"`) {
		t.Fatalf("html should omit image frame when no images are provided: %s", html)
	}
	if strings.Contains(html, "data:image/svg") {
		t.Fatalf("html should not render placeholder image when no images are provided: %s", html)
	}
}

func TestRenderHTMLRejectsUnsafeImageSource(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p09-extra-image",
		Variant: "image-caption",
		Meta: deck.PageMeta{
			Badge:   "第 9 页",
			Counter: "9/9",
			Theme:   "orange",
			CTA:     "cta",
		},
		Content: deck.PageContent{
			Title:  "自定义图文页",
			Images: []deck.ImageBlock{{Src: "javascript:alert(1)", Alt: "bad"}},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if strings.Contains(html, `src="javascript:alert(1)"`) {
		t.Fatalf("html should not render unsafe image src: %s", html)
	}
	if !strings.Contains(html, "data:image/svg") {
		t.Fatalf("html should fall back to placeholder image, got %s", html)
	}
	if strings.Contains(html, `src="#ZgotmplZ"`) {
		t.Fatalf("html should use explicit placeholder instead of ZgotmplZ, got %s", html)
	}
}

func TestRenderHTMLGalleryStepsOmitsGalleryWhenImagesMissing(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p10-extra-gallery",
		Variant: "gallery-steps",
		Meta: deck.PageMeta{
			Badge:   "第 10 页",
			Counter: "10/10",
			Theme:   "green",
			CTA:     "cta",
		},
		Content: deck.PageContent{
			Title: "自定义步骤页",
			Steps: []string{"第一步", "第二步"},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{"第一步", "第二步", `class="steps steps-only"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, `class="gallery"`) {
		t.Fatalf("html should omit gallery when no images are provided: %s", html)
	}
	if strings.Contains(html, "data:image/svg") {
		t.Fatalf("html should not render placeholder gallery images: %s", html)
	}
}

func TestRenderHTMLDefaultDeckDoesNotUseLegacyPlaceholderAssets(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")

	for _, pageIndex := range []int{2, 5} {
		html, err := RenderPageHTML(d, d.Pages[pageIndex])
		if err != nil {
			t.Fatalf("RenderPageHTML(%s) error = %v", d.Pages[pageIndex].Name, err)
		}
		if strings.Contains(html, "data:image/svg") {
			t.Fatalf("html for %s should not render placeholder image when no images are provided", d.Pages[pageIndex].Name)
		}
		if strings.Contains(html, "mail-shot.png") || strings.Contains(html, "domain-shot.png") || strings.Contains(html, "github-report-shot.png") || strings.Contains(html, "registrar-report-shot.png") {
			t.Fatalf("html for %s should not contain legacy asset filenames", d.Pages[pageIndex].Name)
		}
	}
}

func TestRenderHTMLUses1656Viewport(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, `width=1242,height=1656,initial-scale=1`) {
		t.Fatalf("html missing 1656 viewport: %s", html)
	}
}
