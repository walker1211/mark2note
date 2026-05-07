package poster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct {
	name   string
	result map[string][]Candidate
	err    error
}

func (p *fakeProvider) Name() string { return p.name }

func (p *fakeProvider) Search(_ context.Context, title string) ([]Candidate, error) {
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

func TestEnrichMarkdownStartsTenTitleSearchesByDefault(t *testing.T) {
	provider := newBlockingProvider()
	markdown := titlesMarkdown(12)
	done := make(chan error, 1)

	go func() {
		_, _, err := EnrichMarkdown(context.Background(), markdown, EnrichOptions{Providers: []Provider{provider}})
		done <- err
	}()

	for i := 0; i < 10; i++ {
		select {
		case <-provider.started:
		case <-time.After(300 * time.Millisecond):
			provider.releaseAll()
			t.Fatalf("started %d title searches, want 10 concurrent searches by default", i)
		}
	}
	if got := provider.maxActive(); got != 10 {
		provider.releaseAll()
		t.Fatalf("max active searches = %d, want 10", got)
	}

	provider.releaseAll()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("EnrichMarkdown() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("EnrichMarkdown() did not finish after releasing provider")
	}
}

func TestEnrichMarkdownTimesOutOneTitleWithoutCancelingOthers(t *testing.T) {
	provider := timeoutProvider{}
	markdown := "推荐《快作品一》《慢作品》《快作品二》。"

	manifest, report, err := EnrichMarkdown(context.Background(), markdown, EnrichOptions{Providers: []Provider{provider}, Concurrency: 2, TitleTimeout: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("EnrichMarkdown() error = %v", err)
	}
	if _, ok := manifest.Find("快作品一"); !ok {
		t.Fatalf("快作品一 asset missing: %#v", manifest)
	}
	if _, ok := manifest.Find("快作品二"); !ok {
		t.Fatalf("快作品二 asset missing: %#v", manifest)
	}
	if !reflect.DeepEqual(report.Missing, []string{"慢作品"}) {
		t.Fatalf("missing = %#v, want 慢作品 only", report.Missing)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want one timeout warning", report.Warnings)
	}
}

func titlesMarkdown(n int) string {
	var b strings.Builder
	b.WriteString("# 片单\n\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "- **作品%02d**：介绍。\n", i)
	}
	return b.String()
}

type blockingProvider struct {
	started chan string
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	active  int
	max     int
}

func newBlockingProvider() *blockingProvider {
	return &blockingProvider{started: make(chan string, 20), release: make(chan struct{})}
}

func (p *blockingProvider) Name() string { return "blocking" }

func (p *blockingProvider) Search(ctx context.Context, title string) ([]Candidate, error) {
	p.mu.Lock()
	p.active++
	if p.active > p.max {
		p.max = p.active
	}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.active--
		p.mu.Unlock()
	}()
	p.started <- title
	select {
	case <-p.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return []Candidate{{Title: title, ImageURL: "https://img.example/" + title + ".jpg", Source: p.Name(), Confidence: "high"}}, nil
}

func (p *blockingProvider) maxActive() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.max
}

func (p *blockingProvider) releaseAll() {
	p.once.Do(func() { close(p.release) })
}

type timeoutProvider struct{}

func (p timeoutProvider) Name() string { return "timeout" }

func (p timeoutProvider) Search(ctx context.Context, title string) ([]Candidate, error) {
	if title == "慢作品" {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return []Candidate{{Title: title, ImageURL: "https://img.example/" + title + ".jpg", Source: p.Name(), Confidence: "high"}}, nil
}
