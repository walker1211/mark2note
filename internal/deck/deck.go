package deck

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type Theme struct {
	Name               string
	BG                 string
	Card               string
	Text               string
	Sub                string
	Accent             string
	AccentSoft         string
	Line               string
	Panel              string
	White              string
	AuthorColor        string
	AuthorWeight       string
	AuthorSize         string
	AuthorBottomOffset string
}

type PageMeta struct {
	Badge   string `json:"badge"`
	Counter string `json:"counter"`
	Theme   string `json:"theme"`
	CTA     string `json:"cta"`
}

type ImageBlock struct {
	Src     string `json:"src"`
	Alt     string `json:"alt"`
	Caption string `json:"caption,omitempty"`
}

type CompareRow struct {
	Left  string `json:"left"`
	Right string `json:"right"`
}

type CompareBlock struct {
	LeftLabel  string       `json:"leftLabel"`
	RightLabel string       `json:"rightLabel"`
	Rows       []CompareRow `json:"rows"`
}

type PageContent struct {
	Title    string        `json:"title,omitempty"`
	Subtitle string        `json:"subtitle,omitempty"`
	Body     string        `json:"body,omitempty"`
	Quote    string        `json:"quote,omitempty"`
	Note     string        `json:"note,omitempty"`
	Tip      string        `json:"tip,omitempty"`
	Items    []string      `json:"items,omitempty"`
	Compare  *CompareBlock `json:"compare,omitempty"`
	Steps    []string      `json:"steps,omitempty"`
	Images   []ImageBlock  `json:"images,omitempty"`
}

type Page struct {
	Name    string      `json:"name"`
	Variant string      `json:"variant"`
	Meta    PageMeta    `json:"meta"`
	Content PageContent `json:"content"`
}

type Deck struct {
	OutDir     string
	ThemeName  string           `json:"theme,omitempty"`
	ShowAuthor bool             `json:"-"`
	AuthorText string           `json:"-"`
	Pages      []Page           `json:"pages"`
	Themes     map[string]Theme `json:"-"`
}

type rawPage struct {
	Name    string          `json:"name"`
	Variant string          `json:"variant"`
	Meta    PageMeta        `json:"meta"`
	Content json.RawMessage `json:"content"`
}

type rawDeck struct {
	Theme string    `json:"theme"`
	Pages []rawPage `json:"pages"`
}

const (
	minPages = 3
	maxPages = 12
)

var supportedVariants = map[string]struct{}{
	"cover":         {},
	"quote":         {},
	"image-caption": {},
	"bullets":       {},
	"compare":       {},
	"gallery-steps": {},
	"ending":        {},
}

var allowedContentFieldsByVariant = map[string]map[string]struct{}{
	"cover": {
		"title":    {},
		"subtitle": {},
	},
	"quote": {
		"title": {},
		"quote": {},
		"note":  {},
		"tip":   {},
	},
	"image-caption": {
		"title":  {},
		"body":   {},
		"images": {},
	},
	"bullets": {
		"title": {},
		"items": {},
	},
	"compare": {
		"title":   {},
		"compare": {},
	},
	"gallery-steps": {
		"title":  {},
		"steps":  {},
		"images": {},
	},
	"ending": {
		"title": {},
		"body":  {},
	},
}

var allowedCompareFields = map[string]struct{}{
	"leftLabel":  {},
	"rightLabel": {},
	"rows":       {},
}

func defaultThemes() map[string]Theme {
	return RegisteredThemes()
}

func normalizePageNames(pages []Page) []Page {
	out := make([]Page, len(pages))
	copy(out, pages)
	for i := range out {
		out[i].Name = fmt.Sprintf("p%02d-%s", i+1, out[i].Variant)
	}
	return out
}

func DefaultDeck(outDir string) Deck {
	pages := []Page{
		{
			Name:    "p1-cover",
			Variant: "cover",
			Meta: PageMeta{
				Badge:   "第 1 页",
				Counter: "1/8",
				Theme:   "orange",
				CTA:     "先抓主线，再做分屏",
			},
			Content: PageContent{
				Title:    "把长文整理成可分享卡片",
				Subtitle: "从原始素材中提炼结构，再组织成清晰的多页表达",
			},
		},
		{
			Name:    "p2-quote",
			Variant: "quote",
			Meta: PageMeta{
				Badge:   "第 2 页",
				Counter: "2/8",
				Theme:   "orange",
				CTA:     "每一页只讲一个重点",
			},
			Content: PageContent{
				Title: "信息压缩的核心原则",
				Quote: "先保证结论清晰，再追求语言精炼。",
				Tip:   "一句话只能承载一个动作或判断。",
			},
		},
		{
			Name:    "p3-image-caption",
			Variant: "image-caption",
			Meta: PageMeta{
				Badge:   "第 3 页",
				Counter: "3/8",
				Theme:   "orange",
				CTA:     "图文页负责解释关键上下文",
			},
			Content: PageContent{
				Title: "先给结论，再补背景",
				Body:  "图文页适合承接复杂信息：用一段说明解释原因，再用配图帮助读者快速建立场景。",
			},
		},
		{
			Name:    "p4-bullets",
			Variant: "bullets",
			Meta: PageMeta{
				Badge:   "第 4 页",
				Counter: "4/8",
				Theme:   "orange",
				CTA:     "列表页用于稳定传达步骤",
			},
			Content: PageContent{
				Title: "整理内容时先做这三步",
				Items: []string{
					"先标出必须保留的结论",
					"合并重复表达，统一口径",
					"按阅读顺序重排页面",
				},
			},
		},
		{
			Name:    "p5-compare",
			Variant: "compare",
			Meta: PageMeta{
				Badge:   "第 5 页",
				Counter: "5/8",
				Theme:   "green",
				CTA:     "对比页帮助建立判断标准",
			},
			Content: PageContent{
				Title: "低质量输出与高质量输出",
				Compare: &CompareBlock{
					LeftLabel:  "常见问题",
					RightLabel: "改进方式",
					Rows: []CompareRow{
						{Left: "信息堆叠没有层次", Right: "按主题拆成独立页面"},
						{Left: "每页内容过多", Right: "每页只保留一个结论"},
						{Left: "标题抽象难理解", Right: "标题直接描述动作"},
					},
				},
			},
		},
		{
			Name:    "p6-image-caption",
			Variant: "image-caption",
			Meta: PageMeta{
				Badge:   "第 6 页",
				Counter: "6/8",
				Theme:   "green",
				CTA:     "示意图可以降低理解门槛",
			},
			Content: PageContent{
				Title: "给复杂步骤配一张示意图",
				Body:  "当流程较长时，用图示标注关键节点，能帮助读者快速把握输入、处理和输出关系。",
			},
		},
		{
			Name:    "p7-gallery-steps",
			Variant: "gallery-steps",
			Meta: PageMeta{
				Badge:   "第 7 页",
				Counter: "7/8",
				Theme:   "green",
				CTA:     "把方法沉淀为可复用流程",
			},
			Content: PageContent{
				Title: "拆解步骤示例",
				Steps: []string{
					"收集素材并标注主题",
					"提炼结论并确定页面顺序",
					"校对语句后统一视觉风格",
				},
			},
		},
		{
			Name:    "p8-ending",
			Variant: "ending",
			Meta: PageMeta{
				Badge:   "第 8 页",
				Counter: "8/8",
				Theme:   "green",
				CTA:     "先完成一版，再持续迭代",
			},
			Content: PageContent{
				Title: "先做成可读版本，再追求精致",
				Body:  "稳定的结构与清晰的表达，是复用和回归验证的基础；后续再逐步优化细节。",
			},
		},
	}

	pages = normalizePageNames(pages)

	return Deck{
		OutDir:    outDir,
		ThemeName: ThemeDefault,
		Pages:     pages,
		Themes:    defaultThemes(),
	}
}

func FromJSON(raw string, outDir string) (Deck, error) {
	var rd rawDeck
	if err := json.Unmarshal([]byte(raw), &rd); err != nil {
		return Deck{}, fmt.Errorf("parse deck json: %w", err)
	}

	pages := make([]Page, 0, len(rd.Pages))
	for _, rp := range rd.Pages {
		content, err := parsePageContent(rp)
		if err != nil {
			return Deck{}, err
		}
		pages = append(pages, Page{
			Name:    rp.Name,
			Variant: rp.Variant,
			Meta:    rp.Meta,
			Content: content,
		})
	}

	d := Deck{
		OutDir:    outDir,
		ThemeName: ResolveDeckTheme(rd.Theme),
		Pages:     pages,
		Themes:    defaultThemes(),
	}
	if err := d.Validate(); err != nil {
		return Deck{}, err
	}
	d.Pages = normalizePageNames(d.Pages)
	return d, nil
}

func (d Deck) Validate() error {
	if len(d.Pages) < minPages || len(d.Pages) > maxPages {
		return fmt.Errorf("deck must contain 3 to 12 pages")
	}
	if d.Pages[0].Variant != "cover" {
		return fmt.Errorf("first page must use cover variant")
	}
	if d.Pages[len(d.Pages)-1].Variant != "ending" {
		return fmt.Errorf("last page must use ending variant")
	}
	seenNames := make(map[string]struct{}, len(d.Pages))
	for _, page := range d.Pages {
		if page.Name == "" {
			return fmt.Errorf("page name is required")
		}
		if _, exists := seenNames[page.Name]; exists {
			return fmt.Errorf("duplicate page name %q", page.Name)
		}
		seenNames[page.Name] = struct{}{}
		if page.Variant == "" {
			return fmt.Errorf("page %q variant is required", page.Name)
		}
		if _, ok := supportedVariants[page.Variant]; !ok {
			return fmt.Errorf("unsupported variant %q", page.Variant)
		}
		if page.Meta.Badge == "" || page.Meta.Counter == "" || page.Meta.Theme == "" || page.Meta.CTA == "" {
			return fmt.Errorf("page %q meta fields are required", page.Name)
		}
		if page.Meta.Theme != "orange" && page.Meta.Theme != "green" {
			return fmt.Errorf("unknown theme %q", page.Meta.Theme)
		}
		if err := validateContent(page); err != nil {
			return err
		}
	}
	return nil
}

func parsePageContent(page rawPage) (PageContent, error) {
	if page.Variant == "" {
		return PageContent{}, nil
	}
	allowedFields, ok := allowedContentFieldsByVariant[page.Variant]
	if !ok {
		return PageContent{}, nil
	}

	contentRaw := page.Content
	if len(contentRaw) == 0 {
		contentRaw = json.RawMessage(`{}`)
	}

	var contentFields map[string]json.RawMessage
	if err := json.Unmarshal(contentRaw, &contentFields); err != nil {
		return PageContent{}, pageErr(page.Name, page.Variant, "content must be an object")
	}

	for field, value := range contentFields {
		if _, ok := allowedFields[field]; !ok {
			return PageContent{}, pageErr(page.Name, page.Variant, "unsupported content field %q", field)
		}
		if page.Variant == "compare" && field == "compare" {
			if err := validateCompareFields(page.Name, page.Variant, value); err != nil {
				return PageContent{}, err
			}
		}
	}

	var content PageContent
	decoder := json.NewDecoder(bytes.NewReader(contentRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil {
		return PageContent{}, pageErr(page.Name, page.Variant, "invalid content payload: %v", err)
	}

	return content, nil
}

func validateCompareFields(pageName string, variant string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}

	var compareFields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &compareFields); err != nil {
		return pageErr(pageName, variant, "compare must be an object")
	}

	for field := range compareFields {
		if _, ok := allowedCompareFields[field]; !ok {
			return pageErr(pageName, variant, "unsupported compare field %q", field)
		}
	}

	return nil
}

func validateContent(page Page) error {
	content := page.Content
	switch page.Variant {
	case "cover":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
	case "quote":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
		if content.Quote == "" {
			return pageErr(page.Name, page.Variant, "quote is required")
		}
	case "image-caption":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
		if len(content.Images) > 1 {
			return pageErr(page.Name, page.Variant, "images accepts at most 1 item")
		}
		for i, image := range content.Images {
			if image.Src == "" || image.Alt == "" {
				return pageErr(page.Name, page.Variant, "images[%d].src and images[%d].alt are required", i, i)
			}
		}
	case "bullets":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
		if len(content.Items) < 1 {
			return pageErr(page.Name, page.Variant, "items requires at least 1 item")
		}
	case "compare":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
		if content.Compare == nil {
			return pageErr(page.Name, page.Variant, "compare.leftLabel and compare.rightLabel are required")
		}
		if content.Compare.LeftLabel == "" || content.Compare.RightLabel == "" {
			return pageErr(page.Name, page.Variant, "compare.leftLabel and compare.rightLabel are required")
		}
		if len(content.Compare.Rows) < 1 {
			return pageErr(page.Name, page.Variant, "compare.rows requires at least 1 item")
		}
		for i, row := range content.Compare.Rows {
			if row.Left == "" || row.Right == "" {
				return pageErr(page.Name, page.Variant, "compare.rows[%d].left and compare.rows[%d].right are required", i, i)
			}
		}
	case "gallery-steps":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
		if len(content.Steps) < 2 {
			return pageErr(page.Name, page.Variant, "steps requires at least 2 items")
		}
		for i, image := range content.Images {
			if image.Src == "" || image.Alt == "" {
				return pageErr(page.Name, page.Variant, "images[%d].src and images[%d].alt are required", i, i)
			}
		}
	case "ending":
		if content.Title == "" {
			return pageErr(page.Name, page.Variant, "title is required")
		}
		if content.Body == "" {
			return pageErr(page.Name, page.Variant, "body is required")
		}
	}
	return nil
}

func pageErr(pageName string, variant string, format string, args ...any) error {
	return fmt.Errorf("page %q (%s): %s", pageName, variant, fmt.Sprintf(format, args...))
}
