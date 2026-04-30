package render

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLivePairScannerBuildsCandidatePairs(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(root, "cover.mov"), "mov")

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := liveScanResult{
		Pairs: []liveAssetPair{{
			BaseName:  "cover",
			PhotoPath: filepath.Join(root, "cover.jpg"),
			VideoPath: filepath.Join(root, "cover.mov"),
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() = %#v, want %#v", got, want)
	}
}

func TestLivePairScannerMarksMissingMOVAsSkipped(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cover.jpg"), "jpg")

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := liveScanResult{
		SkippedItems: []SkippedItem{{
			BaseName:  "cover",
			PhotoPath: filepath.Join(root, "cover.jpg"),
			VideoPath: "",
			Message:   "missing .mov pair",
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() = %#v, want %#v", got, want)
	}
}

func TestLivePairScannerMarksMissingJPGAsSkipped(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cover.mov"), "mov")

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	want := liveScanResult{
		SkippedItems: []SkippedItem{{
			BaseName:  "cover",
			PhotoPath: "",
			VideoPath: filepath.Join(root, "cover.mov"),
			Message:   "missing .jpg pair",
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan() = %#v, want %#v", got, want)
	}
}

func TestLivePairScannerFailsOnDuplicateBasename(t *testing.T) {
	var acc livePairAccumulation
	if err := acc.addPhoto("cover", "/tmp/cover-a.jpg"); err != nil {
		t.Fatalf("addPhoto() error = %v", err)
	}

	err := acc.addPhoto("cover", "/tmp/cover-b.jpg")
	if err == nil || !strings.Contains(err.Error(), "cover") {
		t.Fatalf("addPhoto() error = %v", err)
	}
}

func TestLivePairScannerIgnoresUppercaseExtensions(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cover.JPG"), "jpg")
	writeTestFile(t, filepath.Join(root, "cover.MOV"), "mov")

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got.Pairs) != 0 || len(got.SkippedItems) != 0 {
		t.Fatalf("Scan() = %#v, want empty result", got)
	}
}

func TestLivePairScannerIgnoresSubdirectoriesAndSymlinks(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "nested")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestFile(t, filepath.Join(subdir, "nested.jpg"), "jpg")
	writeTestFile(t, filepath.Join(subdir, "nested.mov"), "mov")

	target := filepath.Join(root, "target.jpg")
	writeTestFile(t, target, "jpg")
	if err := os.Symlink(target, filepath.Join(root, "linked.jpg")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got.Pairs) != 0 || len(got.SkippedItems) != 1 {
		t.Fatalf("Scan() = %#v", got)
	}
	if got.SkippedItems[0] != (SkippedItem{BaseName: "target", PhotoPath: target, VideoPath: "", Message: "missing .mov pair"}) {
		t.Fatalf("SkippedItems[0] = %#v", got.SkippedItems[0])
	}
}

func TestLivePairScannerReturnsEmptyResultForEmptyDirectory(t *testing.T) {
	root := t.TempDir()

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(got.Pairs) != 0 || len(got.SkippedItems) != 0 {
		t.Fatalf("Scan() = %#v, want empty result", got)
	}
}

func TestLivePairScannerReturnsDeterministicSortedOutput(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "zeta.mov"), "mov")
	writeTestFile(t, filepath.Join(root, "beta.jpg"), "jpg")
	writeTestFile(t, filepath.Join(root, "alpha.mov"), "mov")
	writeTestFile(t, filepath.Join(root, "beta.mov"), "mov")
	writeTestFile(t, filepath.Join(root, "alpha.jpg"), "jpg")
	writeTestFile(t, filepath.Join(root, "gamma.jpg"), "jpg")

	got, err := (livePairScanner{}).Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	wantPairs := []liveAssetPair{
		{BaseName: "alpha", PhotoPath: filepath.Join(root, "alpha.jpg"), VideoPath: filepath.Join(root, "alpha.mov")},
		{BaseName: "beta", PhotoPath: filepath.Join(root, "beta.jpg"), VideoPath: filepath.Join(root, "beta.mov")},
	}
	wantSkipped := []SkippedItem{
		{BaseName: "gamma", PhotoPath: filepath.Join(root, "gamma.jpg"), VideoPath: "", Message: "missing .mov pair"},
		{BaseName: "zeta", PhotoPath: "", VideoPath: filepath.Join(root, "zeta.mov"), Message: "missing .jpg pair"},
	}
	if !reflect.DeepEqual(got.Pairs, wantPairs) {
		t.Fatalf("Pairs = %#v, want %#v", got.Pairs, wantPairs)
	}
	if !reflect.DeepEqual(got.SkippedItems, wantSkipped) {
		t.Fatalf("SkippedItems = %#v, want %#v", got.SkippedItems, wantSkipped)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
