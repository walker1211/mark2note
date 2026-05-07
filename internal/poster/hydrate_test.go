package poster

import (
	"testing"

	"github.com/walker1211/mark2note/internal/deck"
)

func TestHydrateDeckConvertsComparePageToGallerySteps(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p02-compare",
			Variant: "compare",
			Meta:    deck.PageMeta{Badge: "漫画", Counter: "2/3", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{
				Title: "我心里的智斗门面",
				Compare: &deck.CompareBlock{
					LeftLabel:  "《噬谎者》",
					RightLabel: "《死亡笔记》",
					Rows:       []deck.CompareRow{{Left: "《噬谎者》：规则、胆量都算进局里。", Right: "《死亡笔记》：夜神月和 L 的对抗。"}},
				},
			},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{
		"噬谎者":  {Src: "https://example.com/usogui.jpg"},
		"死亡笔记": {Src: "https://example.com/death-note.jpg"},
	}}

	got, report, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	page := got.Pages[0]
	if page.Variant != "gallery-steps" {
		t.Fatalf("Variant = %q, want gallery-steps", page.Variant)
	}
	if len(page.Content.Images) != 2 || len(page.Content.Steps) != 2 {
		t.Fatalf("images/steps = %d/%d, want 2/2", len(page.Content.Images), len(page.Content.Steps))
	}
	if page.Content.Images[0].Src != "https://example.com/usogui.jpg" || page.Content.Images[1].Caption != "《死亡笔记》" {
		t.Fatalf("images = %#v", page.Content.Images)
	}
	if report.PagesChanged != 1 || report.ImagesAdded != 2 {
		t.Fatalf("report = %#v", report)
	}
}

func TestHydrateDeckPreservesAllBulletItemsWhenConverting(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p03-bullets",
			Variant: "bullets",
			Meta:    deck.PageMeta{Badge: "漫画", Counter: "3/3", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "人性局", Items: []string{"《朋友游戏》：选择比题目更好看。", "《真实账号》：社交网络那层皮。", "《赌博默示录》：每个选择都很重。"}},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{"朋友游戏": {Src: "https://example.com/tomodachi.jpg"}, "真实账号": {Src: "https://example.com/real-account.jpg"}}}

	got, _, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	page := got.Pages[0]
	if page.Variant != "gallery-steps" {
		t.Fatalf("Variant = %q, want gallery-steps", page.Variant)
	}
	if len(page.Content.Images) != 2 || len(page.Content.Steps) != 3 {
		t.Fatalf("images/steps = %d/%d, want 2/3", len(page.Content.Images), len(page.Content.Steps))
	}
	if page.Content.Steps[2] != "《赌博默示录》：每个选择都很重。" {
		t.Fatalf("steps = %#v", page.Content.Steps)
	}
}

func TestHydrateDeckPreservesAllCompareRowsWhenConverting(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p02-compare",
			Variant: "compare",
			Meta:    deck.PageMeta{Badge: "漫画", Counter: "2/3", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{
				Title: "我心里的智斗门面",
				Compare: &deck.CompareBlock{
					LeftLabel:  "《噬谎者》",
					RightLabel: "《死亡笔记》",
					Rows: []deck.CompareRow{
						{Left: "《噬谎者》：规则、胆量都算进局里。", Right: "《死亡笔记》：夜神月和 L 的对抗。"},
						{Left: "后期赌局更硬。", Right: "节奏更适合入门。"},
					},
				},
			},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{"噬谎者": {Src: "https://example.com/usogui.jpg"}, "死亡笔记": {Src: "https://example.com/death-note.jpg"}}}

	got, _, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	steps := got.Pages[0].Content.Steps
	if len(steps) != 4 {
		t.Fatalf("steps = %#v, want 4 entries", steps)
	}
	if steps[2] != "后期赌局更硬。" || steps[3] != "节奏更适合入门。" {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestHydrateDeckUsesBoldTitlesInsteadOfColonLead(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p04-gallery-steps",
			Variant: "gallery-steps",
			Meta:    deck.PageMeta{Badge: "动画", Counter: "4/4", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "怎么选", Steps: []string{
				"想轻一点、好入口：先看 **冰果**、**药屋少女的呢喃**。",
				"想看逻辑拆解和说服：把 **虚构推理**、**奇巧计程车** 放到同一组。",
			}},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{
		"冰果":      {Src: "https://example.com/hyouka.jpg"},
		"药屋少女的呢喃": {Src: "https://example.com/apothecary.jpg"},
		"虚构推理":    {Src: "https://example.com/kyokou.jpg"},
		"奇巧计程车":   {Src: "https://example.com/oddtaxi.jpg"},
	}}

	got, report, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	if len(got.Pages[0].Content.Images) != 2 {
		t.Fatalf("images = %#v, want two", got.Pages[0].Content.Images)
	}
	if got.Pages[0].Content.Images[0].Caption != "《冰果》" || got.Pages[0].Content.Images[1].Caption != "《药屋少女的呢喃》" {
		t.Fatalf("images = %#v", got.Pages[0].Content.Images)
	}
	if len(report.Missing) != 0 {
		t.Fatalf("missing = %#v, want none", report.Missing)
	}
}

func TestHydrateDeckDoesNotReportGenericCompareLabels(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p08-compare",
			Variant: "compare",
			Meta:    deck.PageMeta{Badge: "动画", Counter: "8/12", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{
				Title: "怎么选",
				Compare: &deck.CompareBlock{
					LeftLabel:  "你现在想看",
					RightLabel: "可以先放进清单",
					Rows:       []deck.CompareRow{{Left: "《冰果》：轻一点。", Right: "《药屋少女的呢喃》：也好入口。"}},
				},
			},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{"冰果": {Src: "https://example.com/hyouka.jpg"}, "药屋少女的呢喃": {Src: "https://example.com/apothecary.jpg"}}}

	got, report, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	if len(got.Pages[0].Content.Images) != 2 {
		t.Fatalf("images = %#v, want two", got.Pages[0].Content.Images)
	}
	if len(report.Missing) != 0 {
		t.Fatalf("missing = %#v, want none", report.Missing)
	}
}

func TestHydrateDeckReportsMissingTitlesEvenWhenPageCanHydrate(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p03-bullets",
			Variant: "bullets",
			Meta:    deck.PageMeta{Badge: "漫画", Counter: "3/3", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "人性局", Items: []string{"《朋友游戏》：选择比题目更好看。", "《真实账号》：社交网络那层皮。", "《赌博默示录》：每个选择都很重。"}},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{"朋友游戏": {Src: "https://example.com/tomodachi.jpg"}, "真实账号": {Src: "https://example.com/real-account.jpg"}}}

	_, report, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	if len(report.Missing) != 1 || report.Missing[0] != "赌博默示录" {
		t.Fatalf("missing = %#v, want [赌博默示录]", report.Missing)
	}
}

func TestHydrateDeckLeavesPageUnchangedWhenTwoPostersAreNotAvailable(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p03-bullets",
			Variant: "bullets",
			Meta:    deck.PageMeta{Badge: "漫画", Counter: "3/3", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "人性局", Items: []string{"《朋友游戏》：选择比题目更好看。", "《真实账号》：社交网络那层皮。"}},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{"朋友游戏": {Src: "https://example.com/tomodachi.jpg"}}}

	got, report, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	if got.Pages[0].Variant != "bullets" || len(got.Pages[0].Content.Images) != 0 {
		t.Fatalf("page changed unexpectedly: %#v", got.Pages[0])
	}
	if report.PagesChanged != 0 || report.ImagesAdded != 0 {
		t.Fatalf("report = %#v", report)
	}
}

func TestHydrateDeckAddsImagesToExistingGallerySteps(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p04-gallery-steps",
			Variant: "gallery-steps",
			Meta:    deck.PageMeta{Badge: "动画", Counter: "4/4", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "高概念", Steps: []string{"《游戏人生》：爽感直接。", "《忧国的莫里亚蒂》：布局操盘。"}},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{"游戏人生": {Src: "https://example.com/ngnl.jpg"}, "忧国的莫里亚蒂": {Src: "https://example.com/moriarty.jpg"}}}

	got, report, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	if got.Pages[0].Variant != "gallery-steps" || len(got.Pages[0].Content.Images) != 2 {
		t.Fatalf("page = %#v", got.Pages[0])
	}
	if report.ImagesAdded != 2 {
		t.Fatalf("report = %#v", report)
	}
}

func TestHydrateDeckPrefersUnusedPostersFromCurrentPageCandidates(t *testing.T) {
	d := deck.Deck{Pages: []deck.Page{
		{
			Name:    "p04-gallery-steps",
			Variant: "gallery-steps",
			Meta:    deck.PageMeta{Badge: "动画", Counter: "4/5", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "轻入口", Steps: []string{"先看 **冰果**、**药屋少女的呢喃**。"}},
		},
		{
			Name:    "p05-gallery-steps",
			Variant: "gallery-steps",
			Meta:    deck.PageMeta{Badge: "动画", Counter: "5/5", Theme: "default", CTA: "继续"},
			Content: deck.PageContent{Title: "路线选择", Steps: []string{"先看 **冰果**、**药屋少女的呢喃**，也可以补 **侦探学园Q**、**夏日重现**。"}},
		},
	}}
	manifest := Manifest{Posters: map[string]PosterAsset{
		"冰果":      {Src: "https://example.com/hyouka.jpg"},
		"药屋少女的呢喃": {Src: "https://example.com/apothecary.jpg"},
		"侦探学园Q":   {Src: "https://example.com/tantei-q.jpg"},
		"夏日重现":    {Src: "https://example.com/summer-time.jpg"},
	}}

	got, _, err := HydrateDeck(d, manifest, HydrateOptions{})
	if err != nil {
		t.Fatalf("HydrateDeck() error = %v", err)
	}
	secondImages := got.Pages[1].Content.Images
	if len(secondImages) != 2 {
		t.Fatalf("second page images = %#v, want two", secondImages)
	}
	if secondImages[0].Caption != "《侦探学园Q》" || secondImages[1].Caption != "《夏日重现》" {
		t.Fatalf("second page images = %#v", secondImages)
	}
}
