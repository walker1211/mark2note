package render

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"math"
	"strings"
	"sync"

	"github.com/walker1211/mark2note/internal/deck"
)

//go:embed assets/base.css
var baseCSS string

//go:embed assets/page.tmpl
var pageTemplate string

var (
	pageTemplateOnce     sync.Once
	pageTemplateCompiled *template.Template
	pageTemplateErr      error
)

type pageRenderMode struct {
	Animated   bool
	AnimatedMS int
	Preset     animatedPreset
}

type viewportLayout struct {
	Width   int
	Height  int
	Scale   float64
	OffsetX int
	OffsetY int
}

const animatedRuntimeCSS = `
body[data-animated="true"] .anim-fade-in,
body[data-animated="true"] .anim-fade-up,
body[data-animated="true"] .anim-scale-in,
body[data-animated="true"] .anim-reveal {
    will-change: opacity, transform;
}
`

const animatedRuntimeScript = `(function () {
  var body = document.body;
  if (!body) {
    return;
  }
  var params = new URLSearchParams(window.location.search);
  var bodyMS = Number(body.getAttribute("data-animated-ms") || "0");
  var queryMS = params.get("animated_ms");
  var animatedMS = queryMS === null ? bodyMS : Number(queryMS);
  if (!Number.isFinite(animatedMS) || animatedMS < 0) {
    animatedMS = 0;
  }
  body.setAttribute("data-animated", "true");
  body.setAttribute("data-animated-ms", String(animatedMS));
  body.setAttribute("data-animated-ready", "true");
  function clamp01(value) {
    if (value <= 0) {
      return 0;
    }
    if (value >= 1) {
      return 1;
    }
    return value;
  }
  function apply(selector, kind) {
    var nodes = document.querySelectorAll(selector);
    var nodeCount = nodes.length;
    var totalMS = bodyMS > 0 ? bodyMS : 1;
    var duration = nodeCount <= 1 ? totalMS : Math.max(Math.round(totalMS / nodeCount), 1);
    nodes.forEach(function (node, index) {
      var stagger = nodeCount <= 1 ? 0 : Math.round(((totalMS - duration) * index) / (nodeCount - 1));
      var progress = clamp01((animatedMS - stagger) / duration);
      node.style.opacity = String(progress);
      if (kind === "fade-up" || kind === "reveal") {
        node.style.transform = "translate3d(0," + ((1 - progress) * 24).toFixed(2) + "px,0)";
        return;
      }
      if (kind === "scale-in") {
        node.style.transform = "scale(" + (0.96 + progress * 0.04).toFixed(4) + ")";
        return;
      }
      node.style.transform = "translate3d(0,0,0)";
    });
  }
  apply(".anim-fade-in", "fade-in");
  apply(".anim-fade-up", "fade-up");
  apply(".anim-scale-in", "scale-in");
  apply(".anim-reveal", "reveal");
})();`

func compiledPageTemplate() (*template.Template, error) {
	pageTemplateOnce.Do(func() {
		pageTemplateCompiled, pageTemplateErr = template.New("page").Funcs(template.FuncMap{
			"imageSrc":       safeImageSrc,
			"add1":           func(i int) int { return i + 1 },
			"galleryCaption": galleryCaptionHTML,
			"renderRichText": renderRichTextHTML,
			"animClass": func(animated bool, preset string) string {
				if !animated {
					return ""
				}
				preset = strings.TrimSpace(preset)
				if preset == "" {
					return ""
				}
				return " anim-" + preset
			},
		}).Parse(pageTemplate)
	})
	if pageTemplateErr != nil {
		return nil, pageTemplateErr
	}
	return pageTemplateCompiled, nil
}

func RenderPageHTML(d deck.Deck, page deck.Page) (string, error) {
	return renderPageHTML(d, page, pageRenderMode{})
}

func RenderAnimatedPageHTML(d deck.Deck, page deck.Page, animatedMS int) (string, error) {
	preset, ok := defaultAnimatedPreset(page.Variant)
	if !ok {
		return "", fmt.Errorf("unsupported animated preset for variant %q", page.Variant)
	}
	return renderPageHTML(d, page, pageRenderMode{Animated: true, AnimatedMS: animatedMS, Preset: preset})
}

func renderPageHTML(d deck.Deck, page deck.Page, mode pageRenderMode) (string, error) {
	tmpl, err := compiledPageTemplate()
	if err != nil {
		return "", err
	}

	resolvedThemeKey := deck.ResolvePageTheme(d.ThemeName, page.Meta.Theme)
	if resolvedThemeKey == "" {
		return "", fmt.Errorf("unknown theme %q", page.Meta.Theme)
	}
	theme, ok := d.Themes[resolvedThemeKey]
	if !ok {
		return "", fmt.Errorf("unknown theme %q", page.Meta.Theme)
	}

	hydrated, err := withVariantData(page)
	if err != nil {
		return "", err
	}

	layout := coverAuthorLayout{}
	if hydrated.Variant == "cover" && d.ShowAuthor {
		layout = resolveCoverAuthorLayout(d.AuthorText)
	}

	layoutViewport := resolveViewportLayout(d.ViewportWidth, d.ViewportHeight)
	pageCSS := baseCSS + viewportCSS(layoutViewport)
	data := struct {
		CSS                       template.CSS
		ThemeCSS                  template.CSS
		AnimatedScriptTag         template.HTML
		Page                      deck.Page
		Rich                      richPageContent
		RenderMode                pageRenderMode
		ShowAuthor                bool
		ResolvedAuthorDisplayText string
		ResolvedAuthorFontSize    int
		HideAuthorBecauseOverflow bool
		ShowWatermark             bool
		WatermarkText             string
		WatermarkClass            string
		ViewportWidth             int
		ViewportHeight            int
	}{
		CSS:                       template.CSS(pageCSS),
		ThemeCSS:                  template.CSS(themeVarsCSS(theme)),
		Page:                      hydrated,
		Rich:                      buildRichPageContent(hydrated),
		RenderMode:                mode,
		ShowAuthor:                hydrated.Variant == "cover" && layout.Show,
		ResolvedAuthorDisplayText: layout.ResolvedAuthorDisplayText,
		ResolvedAuthorFontSize:    layout.ResolvedAuthorFontSize,
		HideAuthorBecauseOverflow: layout.HideAuthorBecauseOverflow,
		ShowWatermark:             d.ShowWatermark,
		WatermarkText:             d.WatermarkText,
		WatermarkClass:            watermarkPositionClass(d.WatermarkPosition),
		ViewportWidth:             layoutViewport.Width,
		ViewportHeight:            layoutViewport.Height,
	}
	if mode.Animated {
		data.CSS = template.CSS(pageCSS + animatedRuntimeCSS)
		data.AnimatedScriptTag = template.HTML("<script>" + animatedRuntimeScript + "</script>")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func withVariantData(page deck.Page) (deck.Page, error) {
	switch page.Variant {
	case "cover", "quote", "bullets", "ending":
		return page, nil
	case "compare":
		if page.Content.Compare == nil {
			return deck.Page{}, fmt.Errorf("variant %q requires compare content for page %q", page.Variant, page.Name)
		}
		return page, nil
	case "image-caption":
		return page, nil
	case "gallery-steps":
		target := len(page.Content.Steps)
		if target > 0 && len(page.Content.Images) > target {
			page.Content.Images = page.Content.Images[:target]
		}
		return page, nil
	default:
		return deck.Page{}, fmt.Errorf("unsupported variant %q", page.Variant)
	}
}

func galleryCaptionHTML(img deck.ImageBlock, steps []richText, idx int) template.HTML {
	if img.Caption != "" {
		return renderRichTextHTML(parseRichText(img.Caption, richTextOptions{}))
	}
	if idx >= 0 && idx < len(steps) {
		return renderRichTextHTML(steps[idx])
	}
	return ""
}

func safeImageSrc(src string) template.URL {
	if strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "data:image/") {
		return template.URL(src)
	}
	return template.URL(placeholderImageDataURI("图片占位"))
}

func viewportCSS(layout viewportLayout) string {
	return fmt.Sprintf(
		"html, body { width: %dpx; height: %dpx; }\n.page { transform-origin: top left; transform: translate(%dpx, %dpx) scale(%.6f); }\n",
		layout.Width,
		layout.Height,
		layout.OffsetX,
		layout.OffsetY,
		layout.Scale,
	)
}

func resolveViewportLayout(width int, height int) viewportLayout {
	resolvedWidth := resolveViewportWidth(width)
	resolvedHeight := resolveViewportHeight(height)
	scaleX := float64(resolvedWidth) / 1242.0
	scaleY := float64(resolvedHeight) / 1656.0
	scale := math.Min(scaleX, scaleY)
	contentWidth := int(math.Round(1242.0 * scale))
	contentHeight := int(math.Round(1656.0 * scale))
	return viewportLayout{
		Width:   resolvedWidth,
		Height:  resolvedHeight,
		Scale:   scale,
		OffsetX: maxInt((resolvedWidth-contentWidth)/2, 0),
		OffsetY: maxInt((resolvedHeight-contentHeight)/2, 0),
	}
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func resolveViewportWidth(width int) int {
	if width <= 0 {
		return 1242
	}
	return width
}

func resolveViewportHeight(height int) int {
	if height <= 0 {
		return 1656
	}
	return height
}

func watermarkPositionClass(position string) string {
	if position == "bottom-left" {
		return "watermark-bottom-left"
	}
	return "watermark-bottom-right"
}

func placeholderImageDataURI(label string) string {
	escaped := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	).Replace(label)
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="760" viewBox="0 0 1200 760"><rect width="1200" height="760" rx="36" fill="#F5EFE6"/><rect x="48" y="48" width="1104" height="664" rx="28" fill="#FFFDF9" stroke="#E8DED2" stroke-width="4"/><rect x="96" y="96" width="320" height="28" rx="14" fill="#FDE6DE"/><rect x="96" y="156" width="1008" height="20" rx="10" fill="#EFE7DC"/><rect x="96" y="196" width="864" height="20" rx="10" fill="#EFE7DC"/><rect x="96" y="236" width="920" height="20" rx="10" fill="#EFE7DC"/><rect x="96" y="308" width="1008" height="320" rx="24" fill="#F7F3EC" stroke="#E8DED2" stroke-width="4" stroke-dasharray="18 12"/><text x="600" y="470" text-anchor="middle" font-family="-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif" font-size="48" font-weight="700" fill="#5E5A56">%s</text><text x="600" y="536" text-anchor="middle" font-family="-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif" font-size="28" fill="#8A8178">后续将由 markdown 图片自动解析替换</text></svg>`, escaped)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}
