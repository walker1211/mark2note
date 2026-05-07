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

func TestRenderPageHTMLStaticModeOmitsAnimatedMarkers(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, unwanted := range []string{"data-animated=", "animated_ms", "anim-fade-up", "anim-reveal"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("html should omit animated marker %q: %s", unwanted, html)
		}
	}
}

func TestRenderAnimatedPageHTMLIncludesDeterministicTimeMarkers(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderAnimatedPageHTML(d, d.Pages[0], 700)
	if err != nil {
		t.Fatalf("RenderAnimatedPageHTML() error = %v", err)
	}
	for _, want := range []string{
		`data-animated="true"`,
		`data-animated-ms="700"`,
		`animated_ms`,
		`data-animated-ready`,
		`anim-fade-up`,
		`anim-fade-in`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderAnimatedPageHTMLUsesVariantDefaultPreset(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	bulletsPage := d.Pages[3]
	html, err := RenderAnimatedPageHTML(d, bulletsPage, 0)
	if err != nil {
		t.Fatalf("RenderAnimatedPageHTML() error = %v", err)
	}
	for _, want := range []string{`class="title-lg anim-fade-in"`, `class="bullet anim-reveal"`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderAnimatedPageHTMLDistributesAnimationTimingAcrossWholeDuration(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	bulletsPage := d.Pages[3]
	html, err := RenderAnimatedPageHTML(d, bulletsPage, 2400)
	if err != nil {
		t.Fatalf("RenderAnimatedPageHTML() error = %v", err)
	}
	for _, want := range []string{
		`var totalMS = bodyMS > 0 ? bodyMS : 1;`,
		`var duration = nodeCount <= 1 ? totalMS : Math.max(Math.round(totalMS / nodeCount), 1);`,
		`var stagger = nodeCount <= 1 ? 0 : Math.round(((totalMS - duration) * index) / (nodeCount - 1));`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing distributed timing token %q", want)
		}
	}
	for _, unwanted := range []string{`index * 120`, `kind === "reveal" ? 280 : 360`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("html should not contain fixed timing token %q", unwanted)
		}
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

func TestRenderHTMLDefaultIgnoresPageMetaTheme(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	coverHTML, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() cover error = %v", err)
	}
	endingHTML, err := RenderPageHTML(d, d.Pages[len(d.Pages)-1])
	if err != nil {
		t.Fatalf("RenderPageHTML() ending error = %v", err)
	}
	for name, html := range map[string]string{"cover": coverHTML, "ending": endingHTML} {
		if !strings.Contains(html, "--bg: #F4F1EB;") {
			t.Fatalf("%s html missing default bg vars", name)
		}
		if !strings.Contains(html, "--accent: #181818;") {
			t.Fatalf("%s html missing default accent vars", name)
		}
	}
}

func TestRenderHTMLFreshGreenUsesDeckLevelTheme(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeFreshGreen
	d.Themes = deck.RegisteredThemes()

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, "--bg: #F2F7F2;") {
		t.Fatalf("html missing fresh-green vars")
	}
}

func TestRenderHTMLRetiredShuffleLightFallsBackToDefaultAndIgnoresAssignments(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = "shuffle-light"
	d.Themes = deck.RegisteredThemes()
	d.PageThemeKeys = []string{deck.ThemeEditorialCool}

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, "--bg: #F4F1EB;") {
		t.Fatalf("html missing fallback default vars")
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

func mustExtractContainerHTML(t *testing.T, html string, marker string) string {
	t.Helper()
	start := strings.Index(html, marker)
	if start < 0 {
		t.Fatalf("html missing marker %q", marker)
	}
	segment := html[start:]
	depth := 0
	for i := 0; i < len(segment); {
		switch {
		case strings.HasPrefix(segment[i:], "<div"):
			depth++
			i += len("<div")
		case strings.HasPrefix(segment[i:], "</div>"):
			depth--
			i += len("</div>")
			if depth == 0 {
				return segment[:i]
			}
		default:
			i++
		}
	}

	t.Fatalf("html missing matching closing div for marker %q", marker)
	return ""
}

func TestRenderHTMLImageCaptionWrapsBodyAndCTAInContentColumn(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p05-image-caption",
		Variant: "image-caption",
		Meta: deck.PageMeta{
			Badge:   "第 5 页",
			Counter: "5/6",
			Theme:   "green",
			CTA:     "把风险拆开，工作流就更稳",
		},
		Content: deck.PageContent{
			Title:  "为啥行",
			Body:   "这套方式最重要的，不是“换模型”，而是把风险拆开。\n\n哪天真碰上验证、限制、地区、账号状态这些问题，至少你不会把 **工作流底座** 和模型能力一起丢掉。\n\n最近我还给 `ccs` 补了 browser MCP 支持，把浏览器启动、导航、点击、输入、截图这一套最小闭环先接起来了。对前端开发来说，这很重要，因为它开始补上“边看页面边改页面”的能力，而不只是停在终端里写静态代码。\n\n至少对我来说，现阶段 `ccs + codex` 是一条比较顺手的路：既保留 Claude Code 的底座体验，又能吃到 `gpt-5.4` 的模型能力。",
			Images: []deck.ImageBlock{{Src: "https://example.com/custom-image.png", Alt: "custom-image"}},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{
		`.image-caption-layout {`,
		`height: 1220px;`,
		`display: flex;`,
		`flex-direction: column;`,
		`.image-caption-image {`,
		`height: 760px;`,
		`.image-caption-cta {`,
		`position: static;`,
		`margin-top: auto;`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing image-caption layout css %q", want)
		}
	}
	container := mustExtractContainerHTML(t, html, `<div class="image-caption-layout">`)
	for _, want := range []string{
		`class="image-frame image-caption-image"`,
		`<div class="caption">`,
		`class="cta-bar image-caption-cta"`,
	} {
		if !strings.Contains(container, want) {
			t.Fatalf("image-caption container missing %q: %s", want, container)
		}
	}
	imageIndex := strings.Index(container, `class="image-frame image-caption-image`)
	captionIndex := strings.Index(container, `<div class="caption">`)
	ctaIndex := strings.Index(container, `class="cta-bar image-caption-cta"`)
	if !(imageIndex >= 0 && captionIndex > imageIndex && ctaIndex > captionIndex) {
		t.Fatalf("image-caption layout should render image, caption, then cta in order: %s", container)
	}
}

func TestRenderHTMLImageCaptionKeepsCTAInsideLayoutWithoutImage(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p05-image-caption",
		Variant: "image-caption",
		Meta: deck.PageMeta{
			Badge:   "第 5 页",
			Counter: "5/6",
			Theme:   "green",
			CTA:     "没有配图也要把行动项放到底部",
		},
		Content: deck.PageContent{
			Title: "纯文字图文页",
			Body:  "只有正文，也应该保持清晰的阅读顺序。",
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{`.image-caption-layout {`, `.image-caption-cta {`, `margin-top: auto;`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing image-caption layout css %q", want)
		}
	}
	container := mustExtractContainerHTML(t, html, `<div class="image-caption-layout">`)
	if strings.Contains(container, `class="image-frame image-caption-image`) {
		t.Fatalf("html should not render image block when no image is provided: %s", container)
	}
	captionIndex := strings.Index(container, `<div class="caption">`)
	ctaIndex := strings.Index(container, `class="cta-bar image-caption-cta"`)
	if !(captionIndex >= 0 && ctaIndex > captionIndex) {
		t.Fatalf("image-caption without image should render caption before cta: %s", container)
	}
}

func TestRenderHTMLImageCaptionRendersCTAAfterImageWhenBodyMissing(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := deck.Page{
		Name:    "p05-image-caption",
		Variant: "image-caption",
		Meta: deck.PageMeta{
			Badge:   "第 5 页",
			Counter: "5/6",
			Theme:   "green",
			CTA:     "只有配图也要保留 CTA",
		},
		Content: deck.PageContent{
			Title:  "纯图片图文页",
			Images: []deck.ImageBlock{{Src: "https://example.com/only-image.png", Alt: "only-image"}},
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{`.image-caption-layout {`, `.image-caption-image {`, `.image-caption-cta {`, `margin-top: auto;`} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing image-caption layout css %q", want)
		}
	}
	container := mustExtractContainerHTML(t, html, `<div class="image-caption-layout">`)
	if strings.Contains(container, `<div class="caption">`) {
		t.Fatalf("html should not render caption block when body is missing: %s", container)
	}
	imageIndex := strings.Index(container, `class="image-frame image-caption-image`)
	ctaIndex := strings.Index(container, `class="cta-bar image-caption-cta"`)
	if !(imageIndex >= 0 && ctaIndex > imageIndex) {
		t.Fatalf("image-caption without body should render cta after image: %s", container)
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
		"--bg: #F4F1EB;",
		"--author-color: #5F5A54;",
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

func TestRenderHTMLGalleryStepNumbersAndCompareResultCellsUseMetricValueClass(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeTechNoir
	d.Themes = deck.RegisteredThemes()

	galleryHTML, err := RenderPageHTML(d, d.Pages[5])
	if err != nil {
		t.Fatalf("RenderPageHTML() gallery error = %v", err)
	}
	if strings.Count(galleryHTML, `class="step-no metric-value"`) != len(d.Pages[5].Content.Steps) {
		t.Fatalf("gallery html should mark every step number as metric-value: %s", galleryHTML)
	}

	compareHTML, err := RenderPageHTML(d, d.Pages[4])
	if err != nil {
		t.Fatalf("RenderPageHTML() compare error = %v", err)
	}
	if !strings.Contains(compareHTML, `class="compare-cell right metric-value"`) {
		t.Fatalf("compare html should mark right result cells as metric-value: %s", compareHTML)
	}

	for _, html := range []string{galleryHTML, compareHTML} {
		for _, want := range []string{
			`.bullet-index.metric-value {`,
			`.step-no.metric-value {`,
			`.compare-cell.right.metric-value {`,
			`color: var(--number-color);`,
		} {
			if !strings.Contains(html, want) {
				t.Fatalf("html missing effective metric-value css %q", want)
			}
		}
	}
}

func TestRenderHTMLStepNumberUsesContrastForegroundOnAccentBackground(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeTechNoir
	d.Themes = deck.RegisteredThemes()

	galleryHTML, err := RenderPageHTML(d, d.Pages[5])
	if err != nil {
		t.Fatalf("RenderPageHTML() gallery error = %v", err)
	}
	want := ".step-no.metric-value {\n    color: var(--accent-foreground);\n}"
	if !strings.Contains(galleryHTML, want) {
		t.Fatalf("gallery html missing readable step number css %q", want)
	}
}

func TestRenderHTMLCoverRendersBoldAsSemanticEmphasis(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeTechNoir
	d.Themes = deck.RegisteredThemes()
	page := deck.Page{
		Name:    "p01-cover",
		Variant: "cover",
		Meta:    deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "先看 **抽卡逻辑**"},
		Content: deck.PageContent{Title: "这都 **OpenClaude** 了", Subtitle: "副标题"},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{`class="text-em"`, "OpenClaude", "抽卡逻辑"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLBodyRendersInlineCodeAndCodeBlock(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeTechNoir
	d.Themes = deck.RegisteredThemes()
	page := deck.Page{
		Name:    "p03-image",
		Variant: "image-caption",
		Meta:    deck.PageMeta{Badge: "第 3 页", Counter: "3/3", Theme: "orange", CTA: "cta"},
		Content: deck.PageContent{
			Title: "图文页",
			Body:  "先跑 `mark2note`\n```bash\nmark2note --input article.md --theme tech-noir\n```",
		},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{`class="inline-code"`, `class="code-block"`, "mark2note --input article.md --theme tech-noir"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLTitleDowngradesDisallowedCodeBlockToPlainText(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeTechNoir
	d.Themes = deck.RegisteredThemes()
	page := deck.Page{
		Name:    "p01-cover",
		Variant: "cover",
		Meta:    deck.PageMeta{Badge: "第 1 页", Counter: "1/3", Theme: "orange", CTA: "cta"},
		Content: deck.PageContent{Title: "```bash\nmark2note\n```", Subtitle: "副标题"},
	}

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if strings.Contains(html, `class="code-block"`) {
		t.Fatalf("html should not render code block in title: %s", html)
	}
	if !strings.Contains(html, "```bash") {
		t.Fatalf("html should keep downgraded fenced code as plain text: %s", html)
	}
}

func TestRenderHTMLExposesSemanticThemeVarsForAllUserFacingThemes(t *testing.T) {
	themes := deck.RegisteredThemes()
	cases := []string{
		deck.ThemeDefault,
		deck.ThemeFreshGreen,
		deck.ThemeWarmPaper,
		deck.ThemeEditorialCool,
		deck.ThemeTechNoir,
		deck.ThemePlumInk,
		deck.ThemeSageMist,
	}

	for _, themeName := range cases {
		t.Run(themeName, func(t *testing.T) {
			theme, ok := themes[themeName]
			if !ok {
				t.Fatalf("RegisteredThemes missing %q", themeName)
			}
			d := deck.DefaultDeck("/tmp/out")
			d.ThemeName = themeName
			d.Themes = themes

			html, err := RenderPageHTML(d, d.Pages[0])
			if err != nil {
				t.Fatalf("RenderPageHTML() error = %v", err)
			}

			for _, want := range []string{
				"--accent-foreground: " + theme.AccentForeground + ";",
				"--inverse-pill-color: " + theme.InversePillColor + ";",
				"--watermark-color: " + theme.WatermarkColor + ";",
				"--emphasis-color: " + theme.EmphasisColor + ";",
				"--number-color: " + theme.NumberColor + ";",
				"--inline-code-bg: " + theme.InlineCodeBG + ";",
				"--inline-code-border: " + theme.InlineCodeBorder + ";",
				"--inline-code-color: " + theme.InlineCodeColor + ";",
				"--code-block-bg: " + theme.CodeBlockBG + ";",
				"--code-block-border: " + theme.CodeBlockBorder + ";",
				"--code-block-color: " + theme.CodeBlockColor + ";",
				".text-em {",
				".inline-code {",
				".code-block {",
				".metric-value {",
			} {
				if !strings.Contains(html, want) {
					t.Fatalf("html missing semantic style token %q", want)
				}
			}
		})
	}
}

func TestRenderHTMLTechNoirCoverCTAUsesReadableInversePillForeground(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ThemeName = deck.ThemeTechNoir
	d.Themes = deck.RegisteredThemes()

	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{
		"--inverse-pill-color: #111315;",
		".pill.inverse {",
		"color: var(--inverse-pill-color);",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing readable tech-noir CTA token %q", want)
		}
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

func TestRenderHTMLIgnoresUnknownPageMetaTheme(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	page := d.Pages[0]
	page.Meta.Theme = "missing"

	html, err := RenderPageHTML(d, page)
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	if !strings.Contains(html, "--bg: #F4F1EB;") {
		t.Fatalf("html missing fallback default vars")
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

func TestRenderHTMLUsesDefaultViewport(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{
		`width=1242,height=1656,initial-scale=1`,
		`html, body { width: 1242px; height: 1656px; }`,
		`.page { transform-origin: top left; transform: translate(0px, 0px) scale(1.000000); }`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing default viewport token %q: %s", want, html)
		}
	}
}

func TestRenderHTMLUsesConfiguredViewport(t *testing.T) {
	d := deck.DefaultDeck("/tmp/out")
	d.ViewportWidth = 720
	d.ViewportHeight = 960
	html, err := RenderPageHTML(d, d.Pages[0])
	if err != nil {
		t.Fatalf("RenderPageHTML() error = %v", err)
	}
	for _, want := range []string{
		`width=720,height=960,initial-scale=1`,
		`html, body { width: 720px; height: 960px; }`,
		`.page { transform-origin: top left; transform: translate(0px, 0px) scale(0.579710); }`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing configured viewport token %q: %s", want, html)
		}
	}
}

func TestResolveViewportLayoutCentersWhenAspectRatioChanges(t *testing.T) {
	layout := resolveViewportLayout(800, 800)
	if layout.Width != 800 || layout.Height != 800 {
		t.Fatalf("layout size = %dx%d, want 800x800", layout.Width, layout.Height)
	}
	if layout.Scale <= 0 {
		t.Fatalf("layout scale = %f, want > 0", layout.Scale)
	}
	if layout.OffsetX <= 0 {
		t.Fatalf("layout OffsetX = %d, want > 0", layout.OffsetX)
	}
	if layout.OffsetY != 0 {
		t.Fatalf("layout OffsetY = %d, want 0", layout.OffsetY)
	}
}
