package poster

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestLoadYAMLFindsByTitleAndAlias(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "posters.yaml")
	content := []byte(`posters:
  噬谎者:
    src: https://example.com/usogui.jpg
    source: anilist
  死亡游戏:
    src: https://example.com/society.jpg
    alias: Society Game
    confidence: medium
  死亡笔记: https://example.com/death-note.jpg
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	asset, ok := manifest.Find("《噬谎者》")
	if !ok || asset.Src != "https://example.com/usogui.jpg" || asset.Source != "anilist" {
		t.Fatalf("Find(噬谎者) = %#v, %v", asset, ok)
	}
	asset, ok = manifest.Find("society game")
	if !ok || asset.Src != "https://example.com/society.jpg" || asset.Confidence != "medium" {
		t.Fatalf("Find(alias) = %#v, %v", asset, ok)
	}
	asset, ok = manifest.Find("死亡笔记")
	if !ok || asset.Src != "https://example.com/death-note.jpg" {
		t.Fatalf("Find(scalar) = %#v, %v", asset, ok)
	}
}

func TestManifestLoadJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "posters.json")
	content := []byte(`{"posters":{"死亡笔记":{"src":"https://example.com/death-note.jpg","source":"manual"}}}`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	asset, ok := manifest.Find("死亡笔记")
	if !ok || asset.Source != "manual" {
		t.Fatalf("Find() = %#v, %v", asset, ok)
	}
}

func TestManifestMergeOverlayWins(t *testing.T) {
	base := Manifest{Posters: map[string]PosterAsset{"噬谎者": {Src: "old"}, "死亡笔记": {Src: "death"}}}
	overlay := Manifest{Posters: map[string]PosterAsset{"噬谎者": {Src: "new"}}}
	merged := base.Merge(overlay)
	if got := merged.Posters["噬谎者"].Src; got != "new" {
		t.Fatalf("overlay src = %q, want new", got)
	}
	if got := merged.Posters["死亡笔记"].Src; got != "death" {
		t.Fatalf("base src = %q, want death", got)
	}
}

func TestMaterializeSrcPreservesRemoteAndDataURI(t *testing.T) {
	for _, src := range []string{"https://example.com/a.jpg", "http://example.com/a.jpg", "data:image/png;base64,abc"} {
		got, err := MaterializeSrc(src, t.TempDir())
		if err != nil {
			t.Fatalf("MaterializeSrc(%q) error = %v", src, err)
		}
		if got != src {
			t.Fatalf("MaterializeSrc(%q) = %q", src, got)
		}
	}
}

func TestMaterializeSrcConvertsLocalFileToDataURI(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "poster.png")
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := MaterializeSrc("poster.png", root)
	if err != nil {
		t.Fatalf("MaterializeSrc() error = %v", err)
	}
	if !strings.HasPrefix(got, "data:image/png;base64,") {
		t.Fatalf("MaterializeSrc() = %q", got)
	}
}
