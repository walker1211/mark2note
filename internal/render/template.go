package render

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
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

func compiledPageTemplate() (*template.Template, error) {
	pageTemplateOnce.Do(func() {
		pageTemplateCompiled, pageTemplateErr = template.New("page").Funcs(template.FuncMap{
			"imageSrc":       safeImageSrc,
			"add1":           func(i int) int { return i + 1 },
			"galleryCaption": galleryCaptionHTML,
			"renderRichText": renderRichTextHTML,
		}).Parse(pageTemplate)
	})
	if pageTemplateErr != nil {
		return nil, pageTemplateErr
	}
	return pageTemplateCompiled, nil
}

func RenderPageHTML(d deck.Deck, page deck.Page) (string, error) {
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

	data := struct {
		CSS                       template.CSS
		ThemeCSS                  template.CSS
		Page                      deck.Page
		Rich                      richPageContent
		ShowAuthor                bool
		ResolvedAuthorDisplayText string
		ResolvedAuthorFontSize    int
		HideAuthorBecauseOverflow bool
		ShowWatermark             bool
		WatermarkText             string
		WatermarkClass            string
	}{
		CSS:                       template.CSS(baseCSS),
		ThemeCSS:                  template.CSS(themeVarsCSS(theme)),
		Page:                      hydrated,
		Rich:                      buildRichPageContent(hydrated),
		ShowAuthor:                hydrated.Variant == "cover" && layout.Show,
		ResolvedAuthorDisplayText: layout.ResolvedAuthorDisplayText,
		ResolvedAuthorFontSize:    layout.ResolvedAuthorFontSize,
		HideAuthorBecauseOverflow: layout.HideAuthorBecauseOverflow,
		ShowWatermark:             d.ShowWatermark,
		WatermarkText:             d.WatermarkText,
		WatermarkClass:            watermarkPositionClass(d.WatermarkPosition),
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
