package deck

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultDeckUsesNeutralContentSchema(t *testing.T) {
	d := DefaultDeck("/tmp/out")

	if len(d.Pages) < 6 || len(d.Pages) > 8 {
		t.Fatalf("len(d.Pages) = %d, want 6..8", len(d.Pages))
	}
	if d.Pages[0].Variant != "cover" {
		t.Fatalf("first variant = %q", d.Pages[0].Variant)
	}
	if d.Pages[len(d.Pages)-1].Variant != "ending" {
		t.Fatalf("last variant = %q", d.Pages[len(d.Pages)-1].Variant)
	}

	for i, page := range d.Pages {
		if page.Content.Title == "" {
			t.Fatalf("page %d (%s) title is empty", i+1, page.Name)
		}
	}

	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	text := string(raw)
	banned := []string{"GitHub", "钓鱼", ".xyz", "share.google", "ClawTeam"}
	for _, token := range banned {
		if strings.Contains(text, token) {
			t.Fatalf("default deck still contains old case token %q", token)
		}
	}
}

func TestDefaultDeckStillCoversAllSupportedVariants(t *testing.T) {
	d := DefaultDeck("/tmp/out")
	seen := map[string]bool{}
	for _, page := range d.Pages {
		seen[page.Variant] = true
	}

	for _, variant := range []string{"cover", "quote", "image-caption", "bullets", "compare", "gallery-steps", "ending"} {
		if !seen[variant] {
			t.Fatalf("missing variant %q in default deck", variant)
		}
	}
}

func TestDefaultDeckUsesNormalizedPageNames(t *testing.T) {
	d := DefaultDeck("/tmp/out")
	for i, page := range d.Pages {
		want := fmt.Sprintf("p%02d-%s", i+1, page.Variant)
		if page.Name != want {
			t.Fatalf("page %d name = %q, want %q", i+1, page.Name, want)
		}
	}
}

func TestDefaultDeckRoundTripViaJSON(t *testing.T) {
	d := DefaultDeck("/tmp/out")

	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	parsed, err := FromJSON(string(raw), "/tmp/out")
	if err != nil {
		t.Fatalf("FromJSON() error = %v\nraw=%s", err, string(raw))
	}

	if !reflect.DeepEqual(d.Pages, parsed.Pages) {
		t.Fatalf("round-trip pages mismatch\noriginal=%#v\nparsed=%#v\nraw=%s", d.Pages, parsed.Pages, string(raw))
	}

	for i := range d.Pages {
		want := d.Pages[i]
		got := parsed.Pages[i]
		if want.Variant != got.Variant {
			t.Fatalf("page %d variant = %q, want %q", i+1, got.Variant, want.Variant)
		}
		if !reflect.DeepEqual(want.Meta, got.Meta) {
			t.Fatalf("page %d meta mismatch\ngot=%#v\nwant=%#v", i+1, got.Meta, want.Meta)
		}
		if !reflect.DeepEqual(want.Content.Images, got.Content.Images) {
			t.Fatalf("page %d images mismatch\ngot=%#v\nwant=%#v", i+1, got.Content.Images, want.Content.Images)
		}
		if !reflect.DeepEqual(want.Content.Compare, got.Content.Compare) {
			t.Fatalf("page %d compare mismatch\ngot=%#v\nwant=%#v", i+1, got.Content.Compare, want.Content.Compare)
		}
	}
}

func TestDeckFromJSONNormalizesPageNamesByOrderAndVariant(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"1","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p1-weird","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"closing","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文"}}
  ]
}`
	d, err := FromJSON(raw, "/tmp/out")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	got := []string{d.Pages[0].Name, d.Pages[1].Name, d.Pages[2].Name}
	want := []string{"p01-cover", "p02-bullets", "p03-ending"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("page names = %#v, want %#v", got, want)
	}
}

func TestDeckFromJSONAcceptsThreePagesWithAnchors(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面","subtitle":"副标题"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间页","items":["要点"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	d, err := FromJSON(raw, "/tmp/out")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if len(d.Pages) != 3 {
		t.Fatalf("len(Pages) = %d, want 3", len(d.Pages))
	}
	if d.OutDir != "/tmp/out" {
		t.Fatalf("OutDir = %q, want %q", d.OutDir, "/tmp/out")
	}
}

func TestDeckFromJSONAcceptsTwelvePagesWithContentSchema(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/12","theme":"orange","cta":"cta"},"content":{"title":"封面","subtitle":"副标题"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/12","theme":"orange","cta":"cta"},"content":{"title":"列表页","items":["a","b"]}},
    {"name":"p03-quote","variant":"quote","meta":{"badge":"第 3 页","counter":"3/12","theme":"orange","cta":"cta"},"content":{"title":"引用页","quote":"一句话"}},
    {"name":"p04-image","variant":"image-caption","meta":{"badge":"第 4 页","counter":"4/12","theme":"orange","cta":"cta"},"content":{"title":"图文页"}},
    {"name":"p05-compare","variant":"compare","meta":{"badge":"第 5 页","counter":"5/12","theme":"green","cta":"cta"},"content":{"title":"对比页","compare":{"leftLabel":"旧","rightLabel":"新","rows":[{"left":"慢","right":"快"}]}}},
    {"name":"p06-gallery","variant":"gallery-steps","meta":{"badge":"第 6 页","counter":"6/12","theme":"green","cta":"cta"},"content":{"title":"步骤页","steps":["第一步","第二步"]}},
    {"name":"p07-bullets","variant":"bullets","meta":{"badge":"第 7 页","counter":"7/12","theme":"orange","cta":"cta"},"content":{"title":"列表页 2","items":["x"]}},
    {"name":"p08-quote","variant":"quote","meta":{"badge":"第 8 页","counter":"8/12","theme":"orange","cta":"cta"},"content":{"title":"引用页 2","quote":"第二句"}},
    {"name":"p09-image","variant":"image-caption","meta":{"badge":"第 9 页","counter":"9/12","theme":"green","cta":"cta"},"content":{"title":"图文页 2"}},
    {"name":"p10-compare","variant":"compare","meta":{"badge":"第 10 页","counter":"10/12","theme":"green","cta":"cta"},"content":{"title":"对比页 2","compare":{"leftLabel":"A","rightLabel":"B","rows":[{"left":"一","right":"二"}]}}},
    {"name":"p11-gallery","variant":"gallery-steps","meta":{"badge":"第 11 页","counter":"11/12","theme":"green","cta":"cta"},"content":{"title":"步骤页 2","steps":["一","二","三"]}},
    {"name":"p12-ending","variant":"ending","meta":{"badge":"第 12 页","counter":"12/12","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	if _, err := FromJSON(raw, "/tmp/out"); err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
}

func TestDeckFromJSONRejectsTooFewPages(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/2","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p02-ending","variant":"ending","meta":{"badge":"第 2 页","counter":"2/2","theme":"green","cta":"cta2"},"content":{"title":"结尾","body":"正文"}}
  ]
}`
	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "must contain 3 to 12 pages") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsThirteenPages(t *testing.T) {
	pages := make([]string, 0, 13)
	pages = append(pages, `{"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/13","theme":"orange","cta":"cta"},"content":{"title":"封面"}}`)
	for i := 2; i <= 12; i++ {
		pages = append(pages, fmt.Sprintf(`{"name":"p%02d-bullets","variant":"bullets","meta":{"badge":"第 %d 页","counter":"%d/13","theme":"orange","cta":"cta"},"content":{"title":"列表页 %d","items":["x"]}}`, i, i, i, i))
	}
	pages = append(pages, `{"name":"p13-ending","variant":"ending","meta":{"badge":"第 13 页","counter":"13/13","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}`)
	raw := `{"pages":[` + strings.Join(pages, ",") + `]}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "deck must contain 3 to 12 pages") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsWhenFirstPageIsNotCover(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-quote","variant":"quote","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"开头","quote":"正文1"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "first page must use cover variant") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsWhenLastPageIsNotEnding(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"列表","items":["a"]}},
    {"name":"p03-quote","variant":"quote","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾?","quote":"不是 ending"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "last page must use ending variant") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONAcceptsTopLevelTheme(t *testing.T) {
	raw := `{
  "theme": "warm-paper",
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	d, err := FromJSON(raw, "/tmp/out")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if d.ThemeName != ThemeWarmPaper {
		t.Fatalf("ThemeName = %q, want %q", d.ThemeName, ThemeWarmPaper)
	}
}

func TestDeckFromJSONFallsBackWhenTopLevelThemeIsInvalid(t *testing.T) {
	raw := `{
  "theme": "bad",
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	d, err := FromJSON(raw, "/tmp/out")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if d.ThemeName != ThemeDefault {
		t.Fatalf("ThemeName = %q, want %q", d.ThemeName, ThemeDefault)
	}
}

func TestDeckFromJSONAssignsConcretePageThemesForShuffleLight(t *testing.T) {
	raw := `{
  "theme": "shuffle-light",
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	d, err := FromJSON(raw, "/tmp/out")
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}
	if d.ThemeName != ThemeShuffleLight {
		t.Fatalf("ThemeName = %q, want %q", d.ThemeName, ThemeShuffleLight)
	}
	if len(d.PageThemeKeys) != len(d.Pages) {
		t.Fatalf("len(PageThemeKeys) = %d, want %d", len(d.PageThemeKeys), len(d.Pages))
	}
	for i, key := range d.PageThemeKeys {
		if key == "" {
			t.Fatalf("PageThemeKeys[%d] is empty", i)
		}
		if i > 0 && d.PageThemeKeys[i-1] == key {
			t.Fatalf("adjacent shuffle-light keys repeated %q", key)
		}
	}
}

func TestDefaultDeckDoesNotAssignPageThemeKeysForFixedThemes(t *testing.T) {
	d := DefaultDeck("/tmp/out")
	if len(d.PageThemeKeys) != 0 {
		t.Fatalf("len(PageThemeKeys) = %d, want 0", len(d.PageThemeKeys))
	}
}

func TestDeckFromJSONRejectsUnknownTheme(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"blue","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `unknown theme "blue"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsEmptyRawPageNameBeforeNormalization(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"second-page","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"third-page","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "page name is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsDuplicateRawPageNamesBeforeNormalization(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"same-raw-name","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"same-raw-name","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间","items":["a"]}},
    {"name":"different-raw-name","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `duplicate page name "same-raw-name"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsUnsupportedVariant(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta1"},"content":{"title":"封面"}},
    {"name":"p02-custom","variant":"custom","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta2"},"content":{"title":"中间"}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta3"},"content":{"title":"结尾","body":"正文3"}}
  ]
}`
	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unsupported variant") {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsImageCaptionWithMoreThanOneImage(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-image","variant":"image-caption","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"图文","images":[{"src":"a","alt":"a"},{"src":"b","alt":"b"}]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-image" (image-caption): images accepts at most 1 item`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsGalleryStepsWithOnlyOneStep(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-gallery","variant":"gallery-steps","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"步骤页","steps":["只有一步"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-gallery" (gallery-steps): steps requires at least 2 items`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsQuoteWithoutQuoteField(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-quote","variant":"quote","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"引用页"}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-quote" (quote): quote is required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsCompareWithoutLabels(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"对比页","compare":{"rows":[{"left":"旧","right":"新"}]}}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-compare" (compare): compare.leftLabel and compare.rightLabel are required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsEndingWithoutBody(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"列表","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p03-ending" (ending): body is required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsFieldsOutsideVariantWhitelist(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面","items":["not allowed"]}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"列表","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p01-cover" (cover): unsupported content field "items"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsUnknownContentField(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面","mystery":"x"}},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"列表","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p01-cover" (cover): unsupported content field "mystery"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsUnknownCompareField(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"对比页","compare":{"leftLabel":"旧","rightLabel":"新","rows":[{"left":"旧","right":"新"}],"extra":"x"}}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-compare" (compare): unsupported compare field "extra"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsNonObjectContent(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":"bad"},
    {"name":"p02-bullets","variant":"bullets","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"列表","items":["a"]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p01-cover" (cover): content must be an object`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsNonObjectCompare(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"对比页","compare":"bad"}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-compare" (compare): compare must be an object`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsCompareWithEmptyRows(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"对比页","compare":{"leftLabel":"旧","rightLabel":"新","rows":[]}}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-compare" (compare): compare.rows requires at least 1 item`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsCompareRowWithoutLeftOrRight(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-compare","variant":"compare","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"对比页","compare":{"leftLabel":"旧","rightLabel":"新","rows":[{"left":"只填左边"}]}}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-compare" (compare): compare.rows[0].left and compare.rows[0].right are required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsImageWithoutSrcOrAlt(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-image","variant":"image-caption","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"图文","images":[{"src":"a"}]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-image" (image-caption): images[0].src and images[0].alt are required`) {
		t.Fatalf("error = %v", err)
	}
}

func TestDeckFromJSONRejectsGalleryStepsImageWithoutSrcOrAlt(t *testing.T) {
	raw := `{
  "pages": [
    {"name":"p01-cover","variant":"cover","meta":{"badge":"第 1 页","counter":"1/3","theme":"orange","cta":"cta"},"content":{"title":"封面"}},
    {"name":"p02-gallery","variant":"gallery-steps","meta":{"badge":"第 2 页","counter":"2/3","theme":"orange","cta":"cta"},"content":{"title":"步骤页","steps":["第一步","第二步"],"images":[{"src":"a"}]}},
    {"name":"p03-ending","variant":"ending","meta":{"badge":"第 3 页","counter":"3/3","theme":"green","cta":"cta"},"content":{"title":"结尾","body":"总结"}}
  ]
}`

	_, err := FromJSON(raw, "/tmp/out")
	if err == nil {
		t.Fatalf("FromJSON() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `page "p02-gallery" (gallery-steps): images[0].src and images[0].alt are required`) {
		t.Fatalf("error = %v", err)
	}
}
