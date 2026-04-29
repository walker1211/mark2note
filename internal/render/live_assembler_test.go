package render

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMakeliveAssemblerCheckAvailableUsesBinary(t *testing.T) {
	lookedUp := ""
	assembler := makeliveAssembler{
		LookPath: func(name string) (string, error) {
			lookedUp = name
			return "/opt/homebrew/bin/makelive", nil
		},
	}
	if err := assembler.CheckAvailable(); err != nil {
		t.Fatalf("CheckAvailable() error = %v", err)
	}
	if lookedUp != "makelive" {
		t.Fatalf("lookedUp = %q, want makelive", lookedUp)
	}
}

func TestMakeliveAssemblerCheckAvailableWrapsLookupError(t *testing.T) {
	assembler := makeliveAssembler{LookPath: func(string) (string, error) { return "", errors.New("not found") }}
	if err := assembler.CheckAvailable(); err == nil || !strings.Contains(err.Error(), "makelive not available") {
		t.Fatalf("CheckAvailable() error = %v", err)
	}
}

func TestResolvedAppleLiveOutputDirDefaultsBesidePackage(t *testing.T) {
	task := appleLiveTask{PackageDir: filepath.Join("/tmp", "output", "p01-cover.live")}
	got := resolvedAppleLiveOutputDir(task)
	want := filepath.Join("/tmp", "output", defaultAppleLiveDirName)
	if got != want {
		t.Fatalf("resolvedAppleLiveOutputDir() = %q, want %q", got, want)
	}
}

func TestMakeliveAssemblerCopiesFilesAndRunsCommand(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "p01-cover.live")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	photoPath := filepath.Join(packageDir, liveCoverFilename)
	videoPath := filepath.Join(packageDir, liveMotionFilename)
	if err := os.WriteFile(photoPath, []byte("jpg"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(videoPath, []byte("mov"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner := &fakeRunner{}
	assembler := makeliveAssembler{Runner: runner, Binary: "makelive"}
	task := appleLiveTask{PageName: "p01-cover", PackageDir: packageDir, PhotoPath: photoPath, VideoPath: videoPath, OutputDir: filepath.Join(root, "apple-live")}
	if err := assembler.Assemble(task); err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}

	call := runner.snapshotCalls()[0]
	want := []string{"makelive", filepath.Join(root, "apple-live", "p01-cover.jpg"), filepath.Join(root, "apple-live", "p01-cover.mov")}
	if !reflect.DeepEqual(call, want) {
		t.Fatalf("call = %#v, want %#v", call, want)
	}
	if _, err := os.Stat(want[1]); err != nil {
		t.Fatalf("photo output missing: %v", err)
	}
	if _, err := os.Stat(want[2]); err != nil {
		t.Fatalf("video output missing: %v", err)
	}
}

func TestMakeliveAssemblerWrapsRunnerError(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "p01-cover.live")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	photoPath := filepath.Join(packageDir, liveCoverFilename)
	videoPath := filepath.Join(packageDir, liveMotionFilename)
	if err := os.WriteFile(photoPath, []byte("jpg"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(videoPath, []byte("mov"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner := &failRunner{failName: "makelive", failErr: errors.New("boom")}
	assembler := makeliveAssembler{Runner: runner, Binary: "makelive"}
	err := assembler.Assemble(appleLiveTask{PageName: "p01-cover", PackageDir: packageDir, PhotoPath: photoPath, VideoPath: videoPath})
	if err == nil || !strings.Contains(err.Error(), "assemble live photo p01-cover") {
		t.Fatalf("Assemble() error = %v", err)
	}
}
