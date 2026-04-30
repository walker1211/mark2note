package render

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakePhotosImporter struct {
	checkErr error

	ensureResults []EnsureAlbumResult
	ensureErrs    []error
	ensureCalls   []string

	importResult RawImportResult
	importErr    error
	importCalls  []ImportPhotosRequest
}

func (f *fakePhotosImporter) CheckAvailable(ctx context.Context) error {
	return f.checkErr
}

func (f *fakePhotosImporter) EnsureAlbum(ctx context.Context, name string) (EnsureAlbumResult, error) {
	f.ensureCalls = append(f.ensureCalls, name)
	if len(f.ensureErrs) > 0 {
		err := f.ensureErrs[0]
		f.ensureErrs = f.ensureErrs[1:]
		if err != nil {
			return EnsureAlbumResult{}, err
		}
	}
	if len(f.ensureResults) == 0 {
		return EnsureAlbumResult{}, nil
	}
	result := f.ensureResults[0]
	f.ensureResults = f.ensureResults[1:]
	return result, nil
}

func (f *fakePhotosImporter) ImportDirectory(ctx context.Context, req ImportPhotosRequest) (RawImportResult, error) {
	f.importCalls = append(f.importCalls, req)
	if f.importErr != nil {
		return RawImportResult{}, f.importErr
	}
	return f.importResult, nil
}

type recordingImportResultWriter struct {
	writeErr error
	reports  []DeliveryReport
	paths    []string
}

func (w *recordingImportResultWriter) Write(report DeliveryReport) (string, error) {
	w.reports = append(w.reports, report)
	if w.writeErr != nil {
		return "", w.writeErr
	}
	path := filepath.Join(report.SourceDir, "import-result.json")
	w.paths = append(w.paths, path)
	return path, nil
}

func TestLiveDeliveryOrchestratorFailsWhenNoCandidatePairs(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "tail.jpg"), "jpg")
	writer := &recordingImportResultWriter{}
	orchestrator := LiveDeliveryOrchestrator{
		Scanner: livePairScanner{},
		Writer:  writer,
		Now:     fixedDeliveryTime,
	}

	_, err := orchestrator.Deliver(liveDeliveryRequest{SourceDir: sourceDir, ImportTimeout: 2 * time.Second})
	if err == nil {
		t.Fatalf("Deliver() error = nil, want non-nil")
	}
	if len(writer.reports) != 1 {
		t.Fatalf("len(writer.reports) = %d, want 1", len(writer.reports))
	}
	report := writer.reports[0]
	if report.Status != deliveryStatusFailed {
		t.Fatalf("Status = %q, want failed", report.Status)
	}
	if report.CandidatePairs != 0 || report.SkippedPairs != 1 {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(report.Message, "no candidate live photo pairs") {
		t.Fatalf("Message = %q", report.Message)
	}
}

func TestLiveDeliveryOrchestratorWritesFailedReportWhenCheckAvailableFails(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "cover.mov"), "mov")
	importer := &fakePhotosImporter{checkErr: ErrUnsupportedPlatform}
	writer := &recordingImportResultWriter{}
	orchestrator := LiveDeliveryOrchestrator{
		Scanner:  livePairScanner{},
		Importer: importer,
		Writer:   writer,
		Now:      fixedDeliveryTime,
	}

	result, err := orchestrator.Deliver(liveDeliveryRequest{SourceDir: sourceDir, ImportTimeout: 2 * time.Second})
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("Deliver() error = %v, want ErrUnsupportedPlatform", err)
	}
	if result.ReportPath != filepath.Join(sourceDir, "import-result.json") {
		t.Fatalf("ReportPath = %q", result.ReportPath)
	}
	if result.Report.Status != deliveryStatusFailed {
		t.Fatalf("Status = %q, want failed", result.Report.Status)
	}
	if !strings.Contains(result.Report.Message, ErrUnsupportedPlatform.Error()) {
		t.Fatalf("Message = %q", result.Report.Message)
	}
}

func TestFinalizeAlbumNameRetriesDefaultGeneratedCollisions(t *testing.T) {
	importer := &fakePhotosImporter{ensureResults: []EnsureAlbumResult{{AlbumName: "mark2note-live-20260409-123456", Created: false}, {AlbumName: "mark2note-live-20260409-123456-2", Created: false}, {AlbumName: "mark2note-live-20260409-123456-3", Created: true}}}

	got, err := finalizeAlbumName(context.Background(), importer, "", fixedDeliveryTime())
	if err != nil {
		t.Fatalf("finalizeAlbumName() error = %v", err)
	}
	if got != "mark2note-live-20260409-123456-3" {
		t.Fatalf("finalizeAlbumName() = %q", got)
	}
	wantCalls := []string{"mark2note-live-20260409-123456", "mark2note-live-20260409-123456-2", "mark2note-live-20260409-123456-3"}
	if !reflect.DeepEqual(importer.ensureCalls, wantCalls) {
		t.Fatalf("ensureCalls = %#v, want %#v", importer.ensureCalls, wantCalls)
	}
}

func TestFinalizeAlbumNameTrimsExplicitAlbumAndReusesUniqueMatch(t *testing.T) {
	importer := &fakePhotosImporter{ensureResults: []EnsureAlbumResult{{AlbumName: "mark2note-live", Created: false}}}

	got, err := finalizeAlbumName(context.Background(), importer, "  mark2note-live  ", fixedDeliveryTime())
	if err != nil {
		t.Fatalf("finalizeAlbumName() error = %v", err)
	}
	if got != "mark2note-live" {
		t.Fatalf("finalizeAlbumName() = %q", got)
	}
	if !reflect.DeepEqual(importer.ensureCalls, []string{"mark2note-live"}) {
		t.Fatalf("ensureCalls = %#v", importer.ensureCalls)
	}
}

func TestFinalizeAlbumNameFailsOnExplicitAlbumAmbiguity(t *testing.T) {
	importer := &fakePhotosImporter{ensureErrs: []error{ErrAlbumAmbiguous}}

	_, err := finalizeAlbumName(context.Background(), importer, "mark2note-live", fixedDeliveryTime())
	if !errors.Is(err, ErrAlbumAmbiguous) {
		t.Fatalf("finalizeAlbumName() error = %v, want ErrAlbumAmbiguous", err)
	}
}

func TestLiveDeliveryOrchestratorReturnsPartialAfterImportSubmission(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "cover.mov"), "mov")
	writeTestFile(t, filepath.Join(sourceDir, "tail.jpg"), "jpg")
	importer := &fakePhotosImporter{
		ensureResults: []EnsureAlbumResult{{AlbumName: "mark2note-live-20260409-123456", Created: true}},
		importResult:  RawImportResult{Executed: true, Message: photosImportSubmittedMessage},
	}
	writer := &recordingImportResultWriter{}
	orchestrator := LiveDeliveryOrchestrator{
		Scanner:  livePairScanner{},
		Importer: importer,
		Writer:   writer,
		Now:      fixedDeliveryTime,
	}

	result, err := orchestrator.Deliver(liveDeliveryRequest{SourceDir: sourceDir, ImportTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	if result.Report.Status != deliveryStatusPartial {
		t.Fatalf("Status = %q, want partial", result.Report.Status)
	}
	if result.Report.AlbumName != "mark2note-live-20260409-123456" {
		t.Fatalf("AlbumName = %q", result.Report.AlbumName)
	}
	if result.Report.CandidatePairs != 1 || result.Report.SkippedPairs != 1 {
		t.Fatalf("report = %#v", result.Report)
	}
	if result.ReportPath != filepath.Join(sourceDir, "import-result.json") {
		t.Fatalf("ReportPath = %q", result.ReportPath)
	}
	if len(importer.importCalls) != 1 {
		t.Fatalf("len(importCalls) = %d, want 1", len(importer.importCalls))
	}
	if importer.importCalls[0] != (ImportPhotosRequest{SourceDir: sourceDir, AlbumName: "mark2note-live-20260409-123456"}) {
		t.Fatalf("importCalls[0] = %#v", importer.importCalls[0])
	}
}

func TestLiveDeliveryOrchestratorReturnsErrorWhenReportWriteFails(t *testing.T) {
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "cover.jpg"), "jpg")
	writeTestFile(t, filepath.Join(sourceDir, "cover.mov"), "mov")
	importer := &fakePhotosImporter{checkErr: ErrUnsupportedPlatform}
	writer := &recordingImportResultWriter{writeErr: errors.New("disk full")}
	orchestrator := LiveDeliveryOrchestrator{
		Scanner:  livePairScanner{},
		Importer: importer,
		Writer:   writer,
		Now:      fixedDeliveryTime,
	}

	_, err := orchestrator.Deliver(liveDeliveryRequest{SourceDir: sourceDir, ImportTimeout: 2 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "write delivery report") {
		t.Fatalf("Deliver() error = %v, want report-write failure", err)
	}
}

func fixedDeliveryTime() time.Time {
	return time.Date(2026, 4, 9, 12, 34, 56, 0, time.Local)
}
