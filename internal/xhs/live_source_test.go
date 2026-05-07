package xhs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/walker1211/mark2note/internal/render"
)

func TestResolveLivePublishSourceLoadsDeliveryReport(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.mov"), "mov")
	reportPath := writeLiveReport(t, render.DeliveryReport{
		SourceDir:      sourceDir,
		AlbumName:      "mark2note-live",
		CandidatePairs: 1,
		Status:         "partial",
		Message:        "manual verification required",
	})
	got, err := ResolveLiveSource(reportPath, nil)
	if err != nil {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
	if got.Report.SourceDir != sourceDir || got.Report.AlbumName != "mark2note-live" {
		t.Fatalf("report = %#v", got.Report)
	}
	wantItems := []ResolvedLiveItem{{
		PageName:  "p01-cover",
		PhotoPath: filepath.Join(sourceDir, "p01-cover.jpg"),
		VideoPath: filepath.Join(sourceDir, "p01-cover.mov"),
	}}
	if !reflect.DeepEqual(got.Items, wantItems) {
		t.Fatalf("items = %#v, want %#v", got.Items, wantItems)
	}
}

func TestResolveLivePublishSourceRejectsFailedReport(t *testing.T) {
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: t.TempDir(), Status: "failed", Message: "permission denied"})
	_, err := ResolveLiveSource(reportPath, nil)
	if err == nil || !strings.Contains(err.Error(), "status must be partial") {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
}

func TestResolveLivePublishSourceRejectsEmptySourceDir(t *testing.T) {
	reportPath := writeLiveReport(t, render.DeliveryReport{Status: "partial", Message: "ok"})
	_, err := ResolveLiveSource(reportPath, nil)
	if err == nil || !strings.Contains(err.Error(), "source_dir is required") {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
}

func TestResolveLivePublishSourceRejectsRelativeSourceDir(t *testing.T) {
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: "relative/apple-live", Status: "partial", Message: "ok"})
	_, err := ResolveLiveSource(reportPath, nil)
	if err == nil || !strings.Contains(err.Error(), "source_dir must be absolute") {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
}

func TestResolveLivePublishSourceUsesFullOrderedRunSetByDefault(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "p02-bullets.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "p02-bullets.mov"), "mov")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.mov"), "mov")
	writeTestFile(t, filepath.Join(sourceDir, "p03-tail.jpg"), "jpg")
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: sourceDir, Status: "partial"})

	got, err := ResolveLiveSource(reportPath, nil)
	if err != nil {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
	want := []ResolvedLiveItem{
		{PageName: "p01-cover", PhotoPath: filepath.Join(sourceDir, "p01-cover.jpg"), VideoPath: filepath.Join(sourceDir, "p01-cover.mov")},
		{PageName: "p02-bullets", PhotoPath: filepath.Join(sourceDir, "p02-bullets.jpg"), VideoPath: filepath.Join(sourceDir, "p02-bullets.mov")},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("items = %#v, want %#v", got.Items, want)
	}
}

func TestResolveLivePublishSourceUsesRequestedSubsetInOrder(t *testing.T) {
	sourceDir := t.TempDir()
	for _, name := range []string{"p01-cover", "p02-bullets", "p03-tail"} {
		writeTestFile(t, filepath.Join(sourceDir, name+".jpg"), "jpg")
		writeTestFile(t, filepath.Join(sourceDir, name+".mov"), "mov")
	}
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: sourceDir, Status: "partial"})

	got, err := ResolveLiveSource(reportPath, []string{"p03-tail", "p01-cover"})
	if err != nil {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
	want := []ResolvedLiveItem{
		{PageName: "p03-tail", PhotoPath: filepath.Join(sourceDir, "p03-tail.jpg"), VideoPath: filepath.Join(sourceDir, "p03-tail.mov")},
		{PageName: "p01-cover", PhotoPath: filepath.Join(sourceDir, "p01-cover.jpg"), VideoPath: filepath.Join(sourceDir, "p01-cover.mov")},
	}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("items = %#v, want %#v", got.Items, want)
	}
}

func TestResolveLivePublishSourceRejectsDuplicateRequestedPage(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.mov"), "mov")
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: sourceDir, Status: "partial"})

	_, err := ResolveLiveSource(reportPath, []string{"p01-cover", "p01-cover"})
	if err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
}

func TestResolveLivePublishSourceFailsOnUnknownPage(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.mov"), "mov")
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: sourceDir, Status: "partial"})

	_, err := ResolveLiveSource(reportPath, []string{"p99-missing"})
	if err == nil || !strings.Contains(err.Error(), "must map to exactly one imported item") {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
}

func TestResolveLivePublishSourceIgnoresSubdirectories(t *testing.T) {
	sourceDir := t.TempDir()
	nested := filepath.Join(sourceDir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", nested, err)
	}
	writeTestFile(t, filepath.Join(nested, "p99-nested.jpg"), "jpg")
	writeTestFile(t, filepath.Join(nested, "p99-nested.mov"), "mov")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "p01-cover.mov"), "mov")
	reportPath := writeLiveReport(t, render.DeliveryReport{SourceDir: sourceDir, Status: "partial"})

	got, err := ResolveLiveSource(reportPath, nil)
	if err != nil {
		t.Fatalf("ResolveLiveSource() error = %v", err)
	}
	want := []ResolvedLiveItem{{PageName: "p01-cover", PhotoPath: filepath.Join(sourceDir, "p01-cover.jpg"), VideoPath: filepath.Join(sourceDir, "p01-cover.mov")}}
	if !reflect.DeepEqual(got.Items, want) {
		t.Fatalf("items = %#v, want %#v", got.Items, want)
	}
}

func writeLiveReport(t *testing.T, report render.DeliveryReport) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "import-result.json")
	content, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
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
