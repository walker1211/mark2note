package poster

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeProvider struct {
	name   string
	result map[string][]Candidate
	err    error
	seen   []string
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) Search(_ context.Context, title string) ([]Candidate, error) {
	p.seen = append(p.seen, title)
	if p.err != nil {
		return nil, p.err
	}
	return p.result[title], nil
}

func TestEnrichMarkdownBuildsManifestFromProviderCandidates(t *testing.T) {
	provider := &fakeProvider{name: "manual", result: map[string][]Candidate{
		"噬谎者":  {{Title: "Usogui", ImageURL: "https://img.example/usogui.jpg", Source: "anilist", Confidence: "medium", Reason: "AniList media cover"}},
		"死亡笔记": {{Title: "死亡笔记", ImageURL: "https://img.example/death-note.jpg", Source: "anilist", Confidence: "high", Reason: "AniList media cover"}},
	}}
	markdown := `# 片单

- **噬谎者**：规则、胆量都算进局里。
- **死亡笔记**：夜神月和 L 的对抗。
`

	manifest, report, err := EnrichMarkdown(context.Background(), markdown, EnrichOptions{Providers: []Provider{provider}})
	if err != nil {
		t.Fatalf("EnrichMarkdown() error = %v", err)
	}
	if !reflect.DeepEqual(report.Titles, []string{"噬谎者", "死亡笔记"}) {
		t.Fatalf("report titles = %#v", report.Titles)
	}
	if len(report.Missing) != 0 || len(report.Warnings) != 0 {
		t.Fatalf("report = %#v", report)
	}
	asset, ok := manifest.Find("噬谎者")
	if !ok {
		t.Fatalf("噬谎者 asset missing: %#v", manifest)
	}
	if asset.Src != "https://img.example/usogui.jpg" || asset.Source != "anilist" || asset.MatchedTitle != "Usogui" || !asset.ReviewRequired {
		t.Fatalf("噬谎者 asset = %#v", asset)
	}
	asset, ok = manifest.Find("死亡笔记")
	if !ok || asset.ReviewRequired {
		t.Fatalf("死亡笔记 asset = %#v, ok = %v", asset, ok)
	}
}

func TestEnrichMarkdownContinuesAfterProviderError(t *testing.T) {
	broken := &fakeProvider{name: "broken", err: errors.New("rate limited")}
	fallback := &fakeProvider{name: "fallback", result: map[string][]Candidate{
		"死亡游戏": {{Title: "Society Game", ImageURL: "https://img.example/society.jpg", Source: "mydramalist", Confidence: "medium", Reason: "MyDramaList search result"}},
	}}

	manifest, report, err := EnrichMarkdown(context.Background(), "推荐《死亡游戏》。", EnrichOptions{Providers: []Provider{broken, fallback}})
	if err != nil {
		t.Fatalf("EnrichMarkdown() error = %v", err)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want one", report.Warnings)
	}
	asset, ok := manifest.Find("死亡游戏")
	if !ok || asset.Src != "https://img.example/society.jpg" {
		t.Fatalf("asset = %#v, ok = %v", asset, ok)
	}
}

func TestEnrichMarkdownReportsMissingTitles(t *testing.T) {
	provider := &fakeProvider{name: "empty", result: map[string][]Candidate{}}

	manifest, report, err := EnrichMarkdown(context.Background(), "推荐《朋友游戏》。", EnrichOptions{Providers: []Provider{provider}})
	if err != nil {
		t.Fatalf("EnrichMarkdown() error = %v", err)
	}
	if len(manifest.Posters) != 0 {
		t.Fatalf("manifest = %#v, want empty", manifest)
	}
	if !reflect.DeepEqual(report.Missing, []string{"朋友游戏"}) {
		t.Fatalf("missing = %#v", report.Missing)
	}
}
