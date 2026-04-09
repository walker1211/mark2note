package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportResultWriterWritesImportResultJSONBesideSourceDir(t *testing.T) {
	root := t.TempDir()
	report := DeliveryReport{
		SourceDir:      root,
		AlbumName:      "mark2note-live",
		CandidatePairs: 2,
		SkippedPairs:   2,
		Status:         deliveryStatusPartial,
		Message:        "import command submitted to Photos; manual verification required",
		SkippedItems: []SkippedItem{
			{
				BaseName:  "tail",
				PhotoPath: filepath.Join(root, "tail.jpg"),
				VideoPath: "",
				Message:   "missing .mov pair",
			},
			{
				BaseName:  "head",
				PhotoPath: "",
				VideoPath: filepath.Join(root, "head.mov"),
				Message:   "missing .jpg pair",
			},
		},
	}

	path, err := (importResultWriter{}).Write(report)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	wantPath := filepath.Join(root, "import-result.json")
	if path != wantPath {
		t.Fatalf("Write() path = %q, want %q", path, wantPath)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var got DeliveryReport
	if err := json.Unmarshal(content, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got.SourceDir != report.SourceDir || got.AlbumName != report.AlbumName || got.CandidatePairs != report.CandidatePairs || got.SkippedPairs != report.SkippedPairs || got.Status != report.Status || got.Message != report.Message {
		t.Fatalf("decoded report = %#v, want %#v", got, report)
	}
	if len(got.SkippedItems) != len(report.SkippedItems) || got.SkippedItems[0] != report.SkippedItems[0] || got.SkippedItems[1] != report.SkippedItems[1] {
		t.Fatalf("decoded skipped_items = %#v, want %#v", got.SkippedItems, report.SkippedItems)
	}
}

func TestImportResultWriterUsesSnakeCaseFields(t *testing.T) {
	root := t.TempDir()
	report := DeliveryReport{
		SourceDir:      root,
		AlbumName:      "album",
		CandidatePairs: 1,
		SkippedPairs:   0,
		Status:         deliveryStatusPartial,
		Message:        "ok",
		SkippedItems: []SkippedItem{{
			BaseName:  "cover",
			PhotoPath: filepath.Join(root, "cover.jpg"),
			VideoPath: filepath.Join(root, "cover.mov"),
			Message:   "paired",
		}},
	}

	path, err := (importResultWriter{}).Write(report)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	text := string(content)
	for _, want := range []string{"source_dir", "album_name", "candidate_pairs", "skipped_pairs", "status", "message", "skipped_items", "base_name", "photo_path", "video_path"} {
		if !strings.Contains(text, want) {
			t.Fatalf("json output missing %q: %s", want, text)
		}
	}
	for _, unexpected := range []string{"SourceDir", "AlbumName", "CandidatePairs", "SkippedItems", "BaseName", "PhotoPath", "VideoPath"} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("json output unexpectedly contains %q: %s", unexpected, text)
		}
	}
}

func TestImportResultWriterRequiresAbsoluteSourceDirInOutput(t *testing.T) {
	for _, sourceDir := range []string{"", "relative/apple-live"} {
		_, err := (importResultWriter{}).Write(DeliveryReport{SourceDir: sourceDir})
		if err == nil {
			t.Fatalf("Write(%q) error = nil, want non-nil", sourceDir)
		}
		if !strings.Contains(err.Error(), "absolute") {
			t.Fatalf("Write(%q) error = %v, want absolute-path error", sourceDir, err)
		}
	}
}

func TestImportResultWriterReturnsWriteFailure(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", filePath, err)
	}

	_, err := (importResultWriter{}).Write(DeliveryReport{SourceDir: filePath})
	if err == nil {
		t.Fatalf("Write() error = nil, want non-nil")
	}
}
