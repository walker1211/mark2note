package render

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeScriptExecutor struct {
	output []byte
	err    error

	calls [][]string
}

func (f *fakeScriptExecutor) CombinedOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	f.calls = append(f.calls, call)
	return append([]byte(nil), f.output...), f.err
}

func TestOsaScriptPhotosImporterCheckAvailableRejectsNonDarwin(t *testing.T) {
	importer := osascriptPhotosImporter{GOOS: func() string { return "linux" }}

	err := importer.CheckAvailable(context.Background())
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("CheckAvailable() error = %v, want ErrUnsupportedPlatform", err)
	}
}

func TestOsaScriptPhotosImporterCheckAvailableMapsPermissionDenied(t *testing.T) {
	executor := &fakeScriptExecutor{
		output: []byte("Photos got an error: Not authorized to send Apple events to Photos. (-1743)"),
		err:    errors.New("exit status 1"),
	}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	err := importer.CheckAvailable(context.Background())
	if !errors.Is(err, ErrAutomationPermissionDenied) {
		t.Fatalf("CheckAvailable() error = %v, want ErrAutomationPermissionDenied", err)
	}
	assertSingleOsaScriptCall(t, executor.calls)
	assertCallContains(t, executor.calls[0], `tell application "Photos"`)
}

func TestOsaScriptPhotosImporterEnsureAlbumReturnsUniqueExactMatch(t *testing.T) {
	executor := &fakeScriptExecutor{output: []byte("FOUND:mark2note-live")}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	got, err := importer.EnsureAlbum(context.Background(), "  mark2note-live  ")
	if err != nil {
		t.Fatalf("EnsureAlbum() error = %v", err)
	}
	want := EnsureAlbumResult{AlbumName: "mark2note-live", Created: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnsureAlbum() = %#v, want %#v", got, want)
	}
	assertSingleOsaScriptCall(t, executor.calls)
	assertCallContains(t, executor.calls[0], `set targetName to "mark2note-live"`)
	assertCallContains(t, executor.calls[0], `FOUND:`)
}

func TestOsaScriptPhotosImporterEnsureAlbumReturnsCreatedAlbum(t *testing.T) {
	executor := &fakeScriptExecutor{output: []byte("CREATED:mark2note-live")}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	got, err := importer.EnsureAlbum(context.Background(), "mark2note-live")
	if err != nil {
		t.Fatalf("EnsureAlbum() error = %v", err)
	}
	want := EnsureAlbumResult{AlbumName: "mark2note-live", Created: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnsureAlbum() = %#v, want %#v", got, want)
	}
	assertSingleOsaScriptCall(t, executor.calls)
	assertCallContains(t, executor.calls[0], `make new album named targetName`)
}

func TestOsaScriptPhotosImporterEnsureAlbumRejectsEmptyResolvedName(t *testing.T) {
	executor := &fakeScriptExecutor{output: []byte("FOUND:")}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	_, err := importer.EnsureAlbum(context.Background(), "mark2note-live")
	if !errors.Is(err, ErrScriptExecution) {
		t.Fatalf("EnsureAlbum() error = %v, want ErrScriptExecution", err)
	}
}

func TestOsaScriptPhotosImporterEnsureAlbumReturnsAlbumAmbiguous(t *testing.T) {
	executor := &fakeScriptExecutor{output: []byte("AMBIGUOUS")}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	_, err := importer.EnsureAlbum(context.Background(), "mark2note-live")
	if !errors.Is(err, ErrAlbumAmbiguous) {
		t.Fatalf("EnsureAlbum() error = %v, want ErrAlbumAmbiguous", err)
	}
}

func TestOsaScriptPhotosImporterEnsureAlbumRejectsNonDarwin(t *testing.T) {
	importer := osascriptPhotosImporter{GOOS: func() string { return "linux" }}

	_, err := importer.EnsureAlbum(context.Background(), "mark2note-live")
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("EnsureAlbum() error = %v, want ErrUnsupportedPlatform", err)
	}
}

func TestOsaScriptPhotosImporterImportDirectoryReturnsExecuted(t *testing.T) {
	sourceDir := t.TempDir()
	executor := &fakeScriptExecutor{output: []byte("IMPORTED:mark2note-live")}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	got, err := importer.ImportDirectory(context.Background(), ImportPhotosRequest{SourceDir: sourceDir, AlbumName: "mark2note-live"})
	if err != nil {
		t.Fatalf("ImportDirectory() error = %v", err)
	}
	want := RawImportResult{Executed: true, Message: "import command submitted to Photos; manual verification required"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportDirectory() = %#v, want %#v", got, want)
	}
	assertSingleOsaScriptCall(t, executor.calls)
	assertCallContains(t, executor.calls[0], `set sourceDir to POSIX file `)
	assertCallContains(t, executor.calls[0], sourceDir)
	assertCallContains(t, executor.calls[0], `IMPORTED:`)
}

func TestOsaScriptPhotosImporterWrapsContextDeadline(t *testing.T) {
	sourceDir := t.TempDir()
	executor := &fakeScriptExecutor{err: context.DeadlineExceeded}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	_, err := importer.ImportDirectory(context.Background(), ImportPhotosRequest{SourceDir: sourceDir, AlbumName: "mark2note-live"})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("ImportDirectory() error = %v, want ErrTimeout", err)
	}
}

func TestOsaScriptPhotosImporterPreservesContextCanceled(t *testing.T) {
	sourceDir := t.TempDir()
	executor := &fakeScriptExecutor{err: context.Canceled}
	importer := osascriptPhotosImporter{Executor: executor, GOOS: func() string { return "darwin" }}

	_, err := importer.ImportDirectory(context.Background(), ImportPhotosRequest{SourceDir: sourceDir, AlbumName: "mark2note-live"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ImportDirectory() error = %v, want context.Canceled", err)
	}
}

func TestOsaScriptPhotosImporterImportDirectoryRejectsNonDarwin(t *testing.T) {
	importer := osascriptPhotosImporter{GOOS: func() string { return "linux" }}

	_, err := importer.ImportDirectory(context.Background(), ImportPhotosRequest{SourceDir: t.TempDir(), AlbumName: "mark2note-live"})
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("ImportDirectory() error = %v, want ErrUnsupportedPlatform", err)
	}
}

func TestOsaScriptPhotosImporterImportDirectoryRejectsRelativeSourceDir(t *testing.T) {
	importer := osascriptPhotosImporter{GOOS: func() string { return "darwin" }}

	_, err := importer.ImportDirectory(context.Background(), ImportPhotosRequest{SourceDir: "relative/apple-live", AlbumName: "mark2note-live"})
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("ImportDirectory() error = %v, want absolute-path error", err)
	}
}

func TestEscapeAppleScriptStringEscapesSpecialCharacters(t *testing.T) {
	got := escapeAppleScriptString("album \\\n\"quoted\"\rname")
	want := `album \\\n\"quoted\"\rname`
	if got != want {
		t.Fatalf("escapeAppleScriptString() = %q, want %q", got, want)
	}
}

func assertSingleOsaScriptCall(t *testing.T, calls [][]string) {
	t.Helper()
	if len(calls) != 1 {
		t.Fatalf("len(calls) = %d, want 1", len(calls))
	}
	if len(calls[0]) < 3 {
		t.Fatalf("call = %#v, want osascript -e <script>", calls[0])
	}
	if calls[0][0] != "osascript" || calls[0][1] != "-e" {
		t.Fatalf("call = %#v, want osascript -e <script>", calls[0])
	}
}

func assertCallContains(t *testing.T, call []string, want string) {
	t.Helper()
	got := strings.Join(call, "\n")
	if !strings.Contains(got, want) {
		t.Fatalf("call missing %q: %s", want, got)
	}
}
