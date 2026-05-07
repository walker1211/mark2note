package render

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/walker1211/mark2note/internal/deck"
)

const (
	fixtureDir    = "testdata/fixtures"
	goldenHTMLDir = "testdata/golden-html"
	goldenPNGDir  = "testdata/golden-png"
)

type regressionFixture struct {
	Author         string `json:"author"`
	AuthorOverride string `json:"authorOverride"`
}

type loadedFixture struct {
	Name string
	Raw  string
	Meta regressionFixture
}

func TestMustLoadFixtureDeckIncludesDefaultWatermarkRuntimeFields(t *testing.T) {
	fixtures := mustLoadRegressionFixtures(t)
	d := mustLoadFixtureDeck(t, fixtures[0], t.TempDir())

	if !d.ShowWatermark {
		t.Fatalf("ShowWatermark = false, want true")
	}
	if d.WatermarkText != "walker1211/mark2note" {
		t.Fatalf("WatermarkText = %q, want %q", d.WatermarkText, "walker1211/mark2note")
	}
	if d.WatermarkPosition != "bottom-right" {
		t.Fatalf("WatermarkPosition = %q, want %q", d.WatermarkPosition, "bottom-right")
	}
}

func TestRenderFixtureHTMLSnapshots(t *testing.T) {
	fixtures := mustLoadRegressionFixtures(t)
	for i := range fixtures {
		fixture := fixtures[i]
		t.Run(fixture.Name, func(t *testing.T) {
			got := renderFixtureHTMLDigest(t, fixture)
			assertGoldenDigest(t, filepath.Join(goldenHTMLDir, fixture.Name+".sha256"), got)
		})
	}
}

func TestRenderFixturePNGBaselines(t *testing.T) {
	if os.Getenv("MARK2NOTE_E2E_CHROME") != "1" {
		t.Skip("set MARK2NOTE_E2E_CHROME=1 to enable Chrome PNG baseline regression")
	}

	chromePath := strings.TrimSpace(os.Getenv("MARK2NOTE_CHROME"))
	if chromePath == "" {
		chromePath = defaultChromePath
	}
	if _, err := os.Stat(chromePath); err != nil {
		t.Skipf("chrome binary not available at %q: %v", chromePath, err)
	}

	fixtures := mustLoadRegressionFixtures(t)
	for i := range fixtures {
		fixture := fixtures[i]
		goldenPath := filepath.Join(goldenPNGDir, fixture.Name+".sha256")
		if _, err := os.Stat(goldenPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("Stat(%q) error = %v", goldenPath, err)
		}
		t.Run(fixture.Name, func(t *testing.T) {
			got := renderFixturePNGDigest(t, fixture, chromePath)
			assertGoldenDigest(t, goldenPath, got)
		})
	}
}

func mustLoadRegressionFixtures(t *testing.T) []loadedFixture {
	t.Helper()

	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", fixtureDir, err)
	}

	fixtures := make([]loadedFixture, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(fixtureDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		var meta regressionFixture
		if err := json.Unmarshal(raw, &meta); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
		}
		fixtures = append(fixtures, loadedFixture{
			Name: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Raw:  string(raw),
			Meta: meta,
		})
	}

	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].Name < fixtures[j].Name
	})
	if len(fixtures) == 0 {
		t.Fatalf("no regression fixtures found in %q", fixtureDir)
	}
	return fixtures
}

func renderFixtureHTMLDigest(t *testing.T, fixture loadedFixture) string {
	t.Helper()

	d := mustLoadFixtureDeck(t, fixture, t.TempDir())
	hasher := sha256.New()
	for _, page := range d.Pages {
		html, err := RenderPageHTML(d, page)
		if err != nil {
			t.Fatalf("RenderPageHTML(%s) error = %v", page.Name, err)
		}
		writeDigestSection(hasher, page.Name)
		writeDigestSection(hasher, html)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func renderFixturePNGDigest(t *testing.T, fixture loadedFixture, chromePath string) string {
	t.Helper()

	outDir := t.TempDir()
	d := mustLoadFixtureDeck(t, fixture, outDir)
	r := Renderer{OutDir: outDir, ChromePath: chromePath, Jobs: 1}
	if _, err := r.Render(d); err != nil {
		t.Fatalf("Render(%s) error = %v", fixture.Name, err)
	}

	hasher := sha256.New()
	for _, page := range d.Pages {
		pngPath := filepath.Join(outDir, page.Name+".png")
		content, err := os.ReadFile(pngPath)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", pngPath, err)
		}
		writeDigestSection(hasher, page.Name)
		writeDigestSection(hasher, string(content))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func mustLoadFixtureDeck(t *testing.T, fixture loadedFixture, outDir string) deck.Deck {
	t.Helper()

	d, err := deck.FromJSON(fixture.Raw, outDir)
	if err != nil {
		t.Fatalf("FromJSON(%s) error = %v", fixture.Name, err)
	}
	author := deck.ResolveCoverAuthor(fixture.Meta.AuthorOverride, fixture.Meta.Author)
	d.ShowAuthor = author.Show
	d.AuthorText = author.Text
	d.ShowWatermark = true
	d.WatermarkText = "walker1211/mark2note"
	d.WatermarkPosition = "bottom-right"
	d.Themes = deck.RegisteredThemes()
	return d
}

func assertGoldenDigest(t *testing.T, path string, got string) {
	t.Helper()

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	want := strings.TrimSpace(string(wantBytes))
	if got != want {
		t.Fatalf("digest mismatch for %s\ngot:  %s\nwant: %s\nrun with UPDATE_GOLDEN=1 to refresh", filepath.Base(path), got, want)
	}
}

func writeDigestSection(hasher interface{ Write([]byte) (int, error) }, value string) {
	var buf bytes.Buffer
	if _, err := fmt.Fprintf(&buf, "%d:%s\n", len(value), value); err != nil {
		panic(err)
	}
	if _, err := hasher.Write(buf.Bytes()); err != nil {
		panic(err)
	}
}
