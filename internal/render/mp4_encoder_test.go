package render

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestMP4EncoderCheckAvailableUsesFFmpegBinary(t *testing.T) {
	lookedUp := ""
	encoder := ffmpegEncoder{
		LookPath: func(name string) (string, error) {
			lookedUp = name
			return "/opt/homebrew/bin/ffmpeg", nil
		},
	}

	if err := encoder.CheckAvailable(); err != nil {
		t.Fatalf("CheckAvailable() error = %v", err)
	}
	if lookedUp != "ffmpeg" {
		t.Fatalf("lookedUp = %q, want %q", lookedUp, "ffmpeg")
	}
}

func TestMP4EncoderCheckAvailableWrapsLookupError(t *testing.T) {
	encoder := ffmpegEncoder{
		LookPath: func(string) (string, error) {
			return "", errors.New("not found")
		},
	}

	err := encoder.CheckAvailable()
	if err == nil {
		t.Fatalf("CheckAvailable() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "ffmpeg not available") {
		t.Fatalf("CheckAvailable() error = %q", err.Error())
	}
}

func TestMP4EncoderBuildsExpectedCommand(t *testing.T) {
	runner := &fakeRunner{}
	encoder := ffmpegEncoder{Runner: runner, Binary: "ffmpeg"}

	err := encoder.Encode("out.mp4", animatedSequenceSpec{FramePattern: "frames/frame-%04d.png", FPS: 8})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	call := runner.snapshotCalls()[0]
	want := []string{"ffmpeg", "-y", "-framerate", "8", "-i", "frames/frame-%04d.png", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "out.mp4"}
	if !reflect.DeepEqual(call, want) {
		t.Fatalf("call = %#v, want %#v", call, want)
	}
}

func TestMP4EncoderWrapsRunnerError(t *testing.T) {
	runner := &failRunner{failName: "ffmpeg", failErr: errors.New("boom")}
	encoder := ffmpegEncoder{Runner: runner, Binary: "ffmpeg"}

	err := encoder.Encode("out.mp4", animatedSequenceSpec{FramePattern: "frames/frame-%04d.png", FPS: 8})
	if err == nil {
		t.Fatalf("Encode() error = nil, want non-nil")
	}
	if got := err.Error(); got != "encode mp4 out.mp4: boom" {
		t.Fatalf("Encode() error = %q", got)
	}
}
