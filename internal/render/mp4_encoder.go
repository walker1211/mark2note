package render

import (
	"fmt"
	"os/exec"
)

type MP4Encoder interface {
	CheckAvailable() error
	Encode(outputPath string, sequence animatedSequenceSpec) error
}

type ffmpegEncoder struct {
	Runner   CommandRunner
	Binary   string
	LookPath func(string) (string, error)
}

func (e ffmpegEncoder) CheckAvailable() error {
	binary := e.binary()
	lookPath := e.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath(binary); err != nil {
		return fmt.Errorf("%s not available: %w", binary, err)
	}
	return nil
}

func (e ffmpegEncoder) Encode(outputPath string, sequence animatedSequenceSpec) error {
	args := []string{"-y", "-framerate", fmt.Sprintf("%d", sequence.FPS), "-i", sequence.FramePattern, "-c:v", "libx264", "-pix_fmt", "yuv420p", "-movflags", "+faststart", outputPath}
	if err := e.runner().Run(e.binary(), args...); err != nil {
		return fmt.Errorf("encode mp4 %s: %w", outputPath, err)
	}
	return nil
}

func (e ffmpegEncoder) binary() string {
	if e.Binary == "" {
		return "ffmpeg"
	}
	return e.Binary
}

func (e ffmpegEncoder) runner() CommandRunner {
	if e.Runner != nil {
		return e.Runner
	}
	return execRunner{}
}
