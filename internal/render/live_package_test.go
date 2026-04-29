package render

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type recordingRunner struct {
	mu           sync.Mutex
	calls        [][]string
	outputByName map[string][]string
}

func (r *recordingRunner) Run(name string, args ...string) error {
	call := append([]string{name}, args...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
	outputs := append([]string(nil), r.outputByName[name]...)
	r.mu.Unlock()
	for _, path := range outputs {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (r *recordingRunner) snapshotCalls() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]string, len(r.calls))
	for i := range r.calls {
		out[i] = append([]string(nil), r.calls[i]...)
	}
	return out
}

type recordingFailRunner struct {
	recordingRunner
	failName string
	failErr  error
}

func (r *recordingFailRunner) Run(name string, args ...string) error {
	call := append([]string{name}, args...)
	r.mu.Lock()
	r.calls = append(r.calls, call)
	outputs := append([]string(nil), r.outputByName[name]...)
	r.mu.Unlock()
	for _, path := range outputs {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			return err
		}
	}
	if name == r.failName {
		return r.failErr
	}
	return nil
}

func TestLiveCoverFrameIndexFirstPicksFirstFrame(t *testing.T) {
	got := liveCoverFrameIndex([]string{"a.png", "b.png", "c.png"}, liveCoverFrameFirst)
	if got != 0 {
		t.Fatalf("liveCoverFrameIndex() = %d, want 0", got)
	}
}

func TestLiveCoverFrameIndexMiddlePicksHalfIndex(t *testing.T) {
	got := liveCoverFrameIndex([]string{"a.png", "b.png", "c.png", "d.png"}, liveCoverFrameMiddle)
	if got != 2 {
		t.Fatalf("liveCoverFrameIndex() = %d, want 2", got)
	}
}

func TestLiveCoverFrameIndexLastPicksLastFrame(t *testing.T) {
	got := liveCoverFrameIndex([]string{"a.png", "b.png", "c.png", "d.png"}, liveCoverFrameLast)
	if got != 3 {
		t.Fatalf("liveCoverFrameIndex() = %d, want 3", got)
	}
}

func TestLivePackageBuilderWritesCoverMotionAndManifest(t *testing.T) {
	root := t.TempDir()
	frameDir := filepath.Join(root, "frames")
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	frames := []string{
		filepath.Join(frameDir, "frame-0000.png"),
		filepath.Join(frameDir, "frame-0001.png"),
		filepath.Join(frameDir, "frame-0002.png"),
	}
	for _, path := range frames {
		if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}

	runner := &recordingRunner{outputByName: map[string][]string{
		"ffmpeg": {
			filepath.Join(root, "p01-cover.live", "cover.jpg"),
			filepath.Join(root, "p01-cover.live", "motion.mov"),
		},
	}}
	builder := livePackageBuilder{
		Runner: runner,
		LookPath: func(name string) (string, error) {
			switch name {
			case "ffmpeg":
				return "/opt/homebrew/bin/ffmpeg", nil
			case "exiftool":
				return "/opt/homebrew/bin/exiftool", nil
			default:
				return "", errors.New("not found")
			}
		},
	}

	task := livePackageTask{
		PageName:     "p01-cover",
		OutputDir:    filepath.Join(root, "p01-cover.live"),
		FramePaths:   frames,
		FramePattern: filepath.Join(frameDir, "frame-%04d.png"),
		DurationMS:   2400,
		FPS:          8,
		PhotoFormat:  "jpeg",
		CoverFrame:   "middle",
	}
	if err := builder.Build(task); err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, name := range []string{"cover.jpg", "motion.mov", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(task.OutputDir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}

	manifestBytes, err := os.ReadFile(filepath.Join(task.OutputDir, "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}
	var manifest livePackageManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("Unmarshal(manifest.json) error = %v", err)
	}
	if manifest.CoverFrame != "middle" || manifest.PhotoFile != "cover.jpg" || manifest.MotionFile != "motion.mov" {
		t.Fatalf("manifest = %#v", manifest)
	}
	if manifest.PairingID == "" {
		t.Fatalf("PairingID = empty")
	}

	calls := runner.snapshotCalls()
	if len(calls) != 4 {
		t.Fatalf("len(calls) = %d, want 4", len(calls))
	}
	if !reflect.DeepEqual([]string{calls[0][0], calls[1][0], calls[2][0], calls[3][0]}, []string{"ffmpeg", "ffmpeg", "exiftool", "exiftool"}) {
		t.Fatalf("calls = %#v", calls)
	}
	wantMotionCall := []string{"ffmpeg", "-y", "-framerate", "0.833333", "-i", filepath.Join(frameDir, "frame-%04d.png"), "-frames:v", "2", "-c:v", "libx264", "-pix_fmt", "yuv420p", filepath.Join(root, "p01-cover.live", "motion.mov")}
	if !reflect.DeepEqual(calls[1], wantMotionCall) {
		t.Fatalf("motion call = %#v, want %#v", calls[1], wantMotionCall)
	}
}

func TestLiveMotionFramerateUsesTimelineDuration(t *testing.T) {
	got := liveMotionFramerate(2400, 2, 8)
	if got != "0.833333" {
		t.Fatalf("liveMotionFramerate() = %q, want %q", got, "0.833333")
	}
}

func TestLiveMotionFrameCountKeepsSingleFrameFallback(t *testing.T) {
	got := liveMotionFrameCount([]string{"frame-0000.png"})
	if got != 1 {
		t.Fatalf("liveMotionFrameCount() = %d, want 1", got)
	}
}

func TestLivePackageBuilderCheckAvailableRequiresFFmpegAndExiftool(t *testing.T) {
	builder := livePackageBuilder{
		LookPath: func(name string) (string, error) {
			if name == "ffmpeg" {
				return "/opt/homebrew/bin/ffmpeg", nil
			}
			return "", errors.New("not found")
		},
	}
	if err := builder.CheckAvailable(); err == nil || !strings.Contains(err.Error(), "exiftool") {
		t.Fatalf("CheckAvailable() error = %v", err)
	}
}

func TestLivePackageBuilderReturnsErrorWhenFFmpegDoesNotProduceOutputs(t *testing.T) {
	root := t.TempDir()
	frame := filepath.Join(root, "frame-0000.png")
	if err := os.WriteFile(frame, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", frame, err)
	}
	runner := &recordingRunner{}
	builder := livePackageBuilder{
		Runner:   runner,
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/tool", nil },
	}
	err := builder.Build(livePackageTask{
		PageName:     "p01-cover",
		OutputDir:    filepath.Join(root, "p01-cover.live"),
		FramePaths:   []string{frame},
		FramePattern: filepath.Join(root, "frame-%04d.png"),
		DurationMS:   2400,
		FPS:          8,
		PhotoFormat:  "jpeg",
		CoverFrame:   "first",
	})
	if err == nil || !strings.Contains(err.Error(), "verify cover image") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestLivePackageBuilderReturnsErrorWhenExiftoolFails(t *testing.T) {
	root := t.TempDir()
	frame := filepath.Join(root, "frame-0000.png")
	if err := os.WriteFile(frame, []byte("png"), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", frame, err)
	}
	runner := &recordingFailRunner{recordingRunner: recordingRunner{outputByName: map[string][]string{
		"ffmpeg": {
			filepath.Join(root, "p01-cover.live", "cover.jpg"),
			filepath.Join(root, "p01-cover.live", "motion.mov"),
		},
	}}, failName: "exiftool", failErr: errors.New("boom")}
	builder := livePackageBuilder{
		Runner:   runner,
		LookPath: func(string) (string, error) { return "/opt/homebrew/bin/tool", nil },
	}
	err := builder.Build(livePackageTask{
		PageName:     "p01-cover",
		OutputDir:    filepath.Join(root, "p01-cover.live"),
		FramePaths:   []string{frame},
		FramePattern: filepath.Join(root, "frame-%04d.png"),
		DurationMS:   2400,
		FPS:          8,
		PhotoFormat:  "jpeg",
		CoverFrame:   "first",
	})
	if err == nil || !strings.Contains(err.Error(), "write live metadata") {
		t.Fatalf("Build() error = %v", err)
	}
}
