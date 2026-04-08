package render

import (
	"fmt"
	"os/exec"
)

type WebPEncoder interface {
	CheckAvailable() error
	Encode(outputPath string, frames []frameSpec) error
}

type img2webpEncoder struct {
	Runner   CommandRunner
	Binary   string
	LookPath func(string) (string, error)
}

func (e img2webpEncoder) CheckAvailable() error {
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

func (e img2webpEncoder) Encode(outputPath string, frames []frameSpec) error {
	args := []string{"-loop", "0", "-lossless", "-o", outputPath}
	for _, frame := range frames {
		args = append(args, "-d", fmt.Sprintf("%d", frame.DurationMS), frame.Path)
	}
	if err := e.runner().Run(e.binary(), args...); err != nil {
		return fmt.Errorf("encode webp %s: %w", outputPath, err)
	}
	return nil
}

func (e img2webpEncoder) binary() string {
	if e.Binary == "" {
		return "img2webp"
	}
	return e.Binary
}

func (e img2webpEncoder) runner() CommandRunner {
	if e.Runner != nil {
		return e.Runner
	}
	return execRunner{}
}
