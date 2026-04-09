package render

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRendererReturnsDeliveryReportAfterLiveImport(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	liveAssembler := &fakeLivePhotoAssembler{}
	importer := &fakePhotosImporter{
		ensureResults: []EnsureAlbumResult{{AlbumName: "mark2note-live-20260409-123456", Created: true}},
		importResult:  RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
	}
	writer := &recordingImportResultWriter{}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		LivePackageBuilder: liveBuilder,
		LivePhotoAssembler: liveAssembler,
		PhotosImporter:     importer,
		ImportResultWriter: writer,
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: filepath.Join(outDir, "apple-live"), ImportPhotos: true, ImportTimeout: 2 * time.Second},
	}

	result, err := r.Render(sampleDeck(outDir))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.DeliveryReport == nil {
		t.Fatalf("DeliveryReport = nil")
	}
	if result.DeliveryReport.Status != deliveryStatusPartial {
		t.Fatalf("Status = %q, want partial", result.DeliveryReport.Status)
	}
	if result.DeliveryReportPath != filepath.Join(outDir, "apple-live", "import-result.json") {
		t.Fatalf("DeliveryReportPath = %q", result.DeliveryReportPath)
	}
}

func TestRendererReturnsFailedDeliveryReportWhenDeliveryStartedThenErrored(t *testing.T) {
	outDir := t.TempDir()
	runner := &fakeRunner{}
	liveBuilder := &fakeLivePackageBuilder{writeOutput: true}
	liveAssembler := &fakeLivePhotoAssembler{}
	importer := &fakePhotosImporter{checkErr: ErrUnsupportedPlatform}
	writer := &recordingImportResultWriter{}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             runner,
		LivePackageBuilder: liveBuilder,
		LivePhotoAssembler: liveAssembler,
		PhotosImporter:     importer,
		ImportResultWriter: writer,
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: filepath.Join(outDir, "apple-live"), ImportPhotos: true, ImportTimeout: 2 * time.Second},
	}

	result, err := r.Render(sampleDeck(outDir))
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("Render() error = %v, want ErrUnsupportedPlatform", err)
	}
	if result.DeliveryReport == nil {
		t.Fatalf("DeliveryReport = nil")
	}
	if result.DeliveryReport.Status != deliveryStatusFailed {
		t.Fatalf("Status = %q, want failed", result.DeliveryReport.Status)
	}
	if result.DeliveryReportPath != filepath.Join(outDir, "apple-live", "import-result.json") {
		t.Fatalf("DeliveryReportPath = %q", result.DeliveryReportPath)
	}
}

func TestRendererTrimsExplicitAlbumAndReusesUniqueMatch(t *testing.T) {
	outDir := t.TempDir()
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             &fakeRunner{},
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		PhotosImporter: &fakePhotosImporter{
			ensureResults: []EnsureAlbumResult{{AlbumName: "my album", Created: false}},
			importResult:  RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
		},
		ImportResultWriter: &recordingImportResultWriter{},
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: filepath.Join(outDir, "apple-live"), ImportPhotos: true, ImportAlbum: "  my album  ", ImportTimeout: 2 * time.Second},
	}

	result, err := r.Render(sampleDeck(outDir))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.DeliveryReport == nil || result.DeliveryReport.AlbumName != "my album" {
		t.Fatalf("DeliveryReport = %#v", result.DeliveryReport)
	}
}

func TestRendererFailsWhenExplicitAlbumIsAmbiguous(t *testing.T) {
	outDir := t.TempDir()
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             &fakeRunner{},
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		PhotosImporter:     &fakePhotosImporter{ensureErrs: []error{ErrAlbumAmbiguous}},
		ImportResultWriter: &recordingImportResultWriter{},
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: filepath.Join(outDir, "apple-live"), ImportPhotos: true, ImportAlbum: "dup", ImportTimeout: 2 * time.Second},
	}

	result, err := r.Render(sampleDeck(outDir))
	if !errors.Is(err, ErrAlbumAmbiguous) {
		t.Fatalf("Render() error = %v, want ErrAlbumAmbiguous", err)
	}
	if result.DeliveryReport == nil || result.DeliveryReport.Status != deliveryStatusFailed {
		t.Fatalf("DeliveryReport = %#v", result.DeliveryReport)
	}
}

func TestRendererReturnsErrorWhenDeliveryReportWriteFails(t *testing.T) {
	outDir := t.TempDir()
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             &fakeRunner{},
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		PhotosImporter:     &fakePhotosImporter{checkErr: ErrUnsupportedPlatform},
		ImportResultWriter: &recordingImportResultWriter{writeErr: errors.New("disk full")},
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: filepath.Join(outDir, "apple-live"), ImportPhotos: true, ImportTimeout: 2 * time.Second},
	}

	_, err := r.Render(sampleDeck(outDir))
	if err == nil || !strings.Contains(err.Error(), "write delivery report") {
		t.Fatalf("Render() error = %v, want report-write failure", err)
	}
}

func TestRendererResolvesRelativeDeliverySourceDir(t *testing.T) {
	relativeOutDir := filepath.Join("testdata", "relative-out")
	r := Renderer{
		OutDir:             relativeOutDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             &fakeRunner{},
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		PhotosImporter: &fakePhotosImporter{
			ensureResults: []EnsureAlbumResult{{AlbumName: "mark2note-live-20260409-123456", Created: true}},
			importResult:  RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
		},
		ImportResultWriter: &recordingImportResultWriter{},
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, ImportPhotos: true, ImportTimeout: 2 * time.Second},
	}
	deck := sampleDeck(relativeOutDir)

	result, err := r.Render(deck)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.DeliveryReport == nil {
		t.Fatalf("DeliveryReport = nil")
	}
	if !filepath.IsAbs(result.DeliveryReport.SourceDir) {
		t.Fatalf("SourceDir = %q, want absolute", result.DeliveryReport.SourceDir)
	}
}
