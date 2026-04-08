package render

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

const defaultAppleLiveDirName = "apple-live"

type makeliveAssembler struct {
	Runner   CommandRunner
	Binary   string
	LookPath func(string) (string, error)
}

func (a makeliveAssembler) CheckAvailable() error {
	binary := a.binary()
	lookPath := a.lookPath()
	if _, err := lookPath(binary); err != nil {
		return fmt.Errorf("%s not available: %w", binary, err)
	}
	return nil
}

func (a makeliveAssembler) Assemble(task appleLiveTask) error {
	if stringsTrim(task.PackageDir) == "" {
		return fmt.Errorf("assemble live photo %s: package dir is required", task.PageName)
	}
	if stringsTrim(task.PhotoPath) == "" {
		return fmt.Errorf("assemble live photo %s: photo path is required", task.PageName)
	}
	if stringsTrim(task.VideoPath) == "" {
		return fmt.Errorf("assemble live photo %s: video path is required", task.PageName)
	}
	if err := ensureOutputFile(task.PhotoPath); err != nil {
		return fmt.Errorf("assemble live photo %s: verify photo: %w", task.PageName, err)
	}
	if err := ensureOutputFile(task.VideoPath); err != nil {
		return fmt.Errorf("assemble live photo %s: verify video: %w", task.PageName, err)
	}

	outputDir := resolvedAppleLiveOutputDir(task)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("assemble live photo %s: create output dir: %w", task.PageName, err)
	}

	photoOutput := filepath.Join(outputDir, task.PageName+filepath.Ext(task.PhotoPath))
	videoOutput := filepath.Join(outputDir, task.PageName+filepath.Ext(task.VideoPath))
	if err := copyFile(task.PhotoPath, photoOutput); err != nil {
		return fmt.Errorf("assemble live photo %s: copy photo: %w", task.PageName, err)
	}
	if err := copyFile(task.VideoPath, videoOutput); err != nil {
		return fmt.Errorf("assemble live photo %s: copy video: %w", task.PageName, err)
	}
	if err := a.runner().Run(a.binary(), photoOutput, videoOutput); err != nil {
		return fmt.Errorf("assemble live photo %s: %w", task.PageName, err)
	}
	return nil
}

func resolvedAppleLiveOutputDir(task appleLiveTask) string {
	if stringsTrim(task.OutputDir) != "" {
		return task.OutputDir
	}
	return filepath.Join(filepath.Dir(task.PackageDir), defaultAppleLiveDirName)
}

func copyFile(src string, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

func (a makeliveAssembler) binary() string {
	if a.Binary == "" {
		return "makelive"
	}
	return a.Binary
}

func (a makeliveAssembler) lookPath() func(string) (string, error) {
	if a.LookPath != nil {
		return a.LookPath
	}
	return exec.LookPath
}

func (a makeliveAssembler) runner() CommandRunner {
	if a.Runner != nil {
		return a.Runner
	}
	return execRunner{}
}
