package render

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
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

func TestRendererImportsOnlyGeneratedLivePairs(t *testing.T) {
	outDir := t.TempDir()
	liveOutDir := filepath.Join(outDir, "apple-live")
	if err := os.MkdirAll(liveOutDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeTestFile(t, filepath.Join(liveOutDir, "stale.jpg"), "stale jpg")
	writeTestFile(t, filepath.Join(liveOutDir, "stale.mov"), "stale mov")
	writeTestFile(t, filepath.Join(liveOutDir, "notes.txt"), "notes")
	importer := &fakePhotosImporter{
		ensureResults: []EnsureAlbumResult{{AlbumName: "mark2note-live-20260409-123456", Created: true}},
		importResult:  RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
	}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             &fakeRunner{},
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		PhotosImporter:     importer,
		ImportResultWriter: &recordingImportResultWriter{},
		Now:                fixedDeliveryTime,
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: liveOutDir, ImportPhotos: true, ImportTimeout: 2 * time.Second},
	}

	result, err := r.Render(sampleDeck(outDir))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.DeliveryReport == nil || result.DeliveryReport.CandidatePairs != 3 {
		t.Fatalf("DeliveryReport = %#v", result.DeliveryReport)
	}
	if len(importer.importCalls) != 1 {
		t.Fatalf("importCalls = %#v", importer.importCalls)
	}
	importCall := importer.importCalls[0]
	if importCall.SourceDir == liveOutDir {
		t.Fatalf("ImportDirectory SourceDir = live output dir %q, want staging dir", importCall.SourceDir)
	}
	entries, err := os.ReadDir(importCall.SourceDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", importCall.SourceDir, err)
	}
	gotNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		gotNames = append(gotNames, entry.Name())
	}
	wantNames := []string{"p01-cover.jpg", "p01-cover.mov", "p02-bullets.jpg", "p02-bullets.mov", "p03-ending.jpg", "p03-ending.mov"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("staged live files = %#v, want %#v", gotNames, wantNames)
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
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("restore cwd error = %v", chdirErr)
		}
	}()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
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

func readDirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

type writingScreenshotRunner struct{}

func (writingScreenshotRunner) Run(name string, args ...string) error {
	for _, arg := range args {
		path, ok := strings.CutPrefix(arg, "--screenshot=")
		if ok {
			return os.WriteFile(path, []byte("png"), 0o644)
		}
	}
	return nil
}

func TestRendererKeepsPNGAndLiveImportStagingSeparateWhenOutputDirIsShared(t *testing.T) {
	outDir := t.TempDir()
	importer := &fakePhotosImporter{
		ensureResults: []EnsureAlbumResult{
			{AlbumName: "png album", Created: true},
			{AlbumName: "live album", Created: true},
		},
		importResult: RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
	}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             writingScreenshotRunner{},
		PhotosImporter:     importer,
		ImportResultWriter: &recordingImportResultWriter{},
		Now:                fixedDeliveryTime,
		ImportPhotos:       true,
		ImportAlbum:        "png album",
		ImportTimeout:      2 * time.Second,
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: outDir, ImportPhotos: true, ImportAlbum: "live album", ImportTimeout: 2 * time.Second},
	}

	_, err := r.Render(sampleDeck(outDir))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(importer.importCalls) != 2 {
		t.Fatalf("importCalls = %#v, want 2 calls", importer.importCalls)
	}
	pngImportDir := importer.importCalls[0].SourceDir
	liveImportDir := importer.importCalls[1].SourceDir
	if pngImportDir == liveImportDir {
		t.Fatalf("PNG and Live import dirs both = %q, want separate staging dirs", pngImportDir)
	}
	pngNames := readDirNames(t, pngImportDir)
	liveNames := readDirNames(t, liveImportDir)
	wantPNGNames := []string{"p01-cover.png", "p02-bullets.png", "p03-ending.png"}
	wantLiveNames := []string{"p01-cover.jpg", "p01-cover.mov", "p02-bullets.jpg", "p02-bullets.mov", "p03-ending.jpg", "p03-ending.mov"}
	if !reflect.DeepEqual(pngNames, wantPNGNames) {
		t.Fatalf("PNG staging files = %#v, want %#v", pngNames, wantPNGNames)
	}
	if !reflect.DeepEqual(liveNames, wantLiveNames) {
		t.Fatalf("Live staging files = %#v, want %#v", liveNames, wantLiveNames)
	}
}

func TestRendererKeepsPNGAndLiveImportReportsSeparateWhenOutputDirIsShared(t *testing.T) {
	outDir := t.TempDir()
	importer := &fakePhotosImporter{
		ensureResults: []EnsureAlbumResult{
			{AlbumName: "png album", Created: true},
			{AlbumName: "live album", Created: true},
		},
		importResult: RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
	}
	r := Renderer{
		OutDir:             outDir,
		ChromePath:         "chrome",
		Jobs:               1,
		Runner:             writingScreenshotRunner{},
		PhotosImporter:     importer,
		Now:                fixedDeliveryTime,
		ImportPhotos:       true,
		ImportAlbum:        "png album",
		ImportTimeout:      2 * time.Second,
		LivePackageBuilder: &fakeLivePackageBuilder{writeOutput: true},
		LivePhotoAssembler: &fakeLivePhotoAssembler{},
		Animated:           animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
		Live:               liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "middle", Assemble: true, OutputDir: outDir, ImportPhotos: true, ImportAlbum: "live album", ImportTimeout: 2 * time.Second},
	}

	result, err := r.Render(sampleDeck(outDir))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.ImportReportPath == "" || result.DeliveryReportPath == "" {
		t.Fatalf("report paths = %q, %q", result.ImportReportPath, result.DeliveryReportPath)
	}
	if result.ImportReportPath != filepath.Join(outDir, photosImportResultFilename) {
		t.Fatalf("ImportReportPath = %q, want photos import report", result.ImportReportPath)
	}
	if result.DeliveryReportPath != filepath.Join(outDir, liveImportResultFilename) {
		t.Fatalf("DeliveryReportPath = %q, want live import report", result.DeliveryReportPath)
	}
	pngContent, err := os.ReadFile(result.ImportReportPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", result.ImportReportPath, err)
	}
	liveContent, err := os.ReadFile(result.DeliveryReportPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", result.DeliveryReportPath, err)
	}
	if !strings.Contains(string(pngContent), `"album_name": "png album"`) {
		t.Fatalf("PNG report content = %s", pngContent)
	}
	if !strings.Contains(string(liveContent), `"album_name": "live album"`) {
		t.Fatalf("Live report content = %s", liveContent)
	}
}

func TestRendererImportsGeneratedPNGOutputDirectory(t *testing.T) {
	outDir := t.TempDir()
	importer := &fakePhotosImporter{
		ensureResults: []EnsureAlbumResult{{AlbumName: "png album", Created: false}},
		importResult:  RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
	}
	if err := os.WriteFile(filepath.Join(outDir, "preview.mp4"), []byte("mp4"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "stale.png"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	r := Renderer{
		OutDir:         outDir,
		ChromePath:     "chrome",
		Jobs:           1,
		Runner:         writingScreenshotRunner{},
		PhotosImporter: importer,
		Now:            fixedDeliveryTime,
		ImportPhotos:   true,
		ImportAlbum:    "  png album  ",
		ImportTimeout:  2 * time.Second,
		Animated:       animatedOptions{Enabled: false, Format: "webp", DurationMS: 2400, FPS: 8},
	}

	result, err := r.Render(sampleDeck(outDir))
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if result.ImportReport == nil {
		t.Fatalf("ImportReport = nil")
	}
	if result.ImportReport.SourceDir != outDir {
		t.Fatalf("SourceDir = %q, want %q", result.ImportReport.SourceDir, outDir)
	}
	if result.ImportReport.CandidatePairs != 3 {
		t.Fatalf("CandidatePairs = %d, want generated PNG count 3", result.ImportReport.CandidatePairs)
	}
	if result.ImportReport.AlbumName != "png album" || result.ImportReport.Status != deliveryStatusPartial {
		t.Fatalf("ImportReport = %#v", result.ImportReport)
	}
	if len(importer.importCalls) != 1 {
		t.Fatalf("importCalls = %#v", importer.importCalls)
	}
	importCall := importer.importCalls[0]
	if importCall.SourceDir == outDir {
		t.Fatalf("ImportDirectory SourceDir = output dir %q, want PNG-only staging dir", importCall.SourceDir)
	}
	if importCall.AlbumName != "png album" {
		t.Fatalf("ImportDirectory AlbumName = %q", importCall.AlbumName)
	}
	entries, err := os.ReadDir(importCall.SourceDir)
	if err != nil {
		t.Fatalf("ReadDir(%q) error = %v", importCall.SourceDir, err)
	}
	gotNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		gotNames = append(gotNames, entry.Name())
	}
	wantNames := []string{"p01-cover.png", "p02-bullets.png", "p03-ending.png"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("staged files = %#v, want %#v", gotNames, wantNames)
	}
	if result.ImportReportPath != filepath.Join(outDir, photosImportResultFilename) {
		t.Fatalf("ImportReportPath = %q", result.ImportReportPath)
	}
}
