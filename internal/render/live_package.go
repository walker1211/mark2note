package render

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

const (
	liveCoverFilename  = "cover.jpg"
	liveMotionFilename = "motion.mov"
)

type livePackageBuilder struct {
	Runner         CommandRunner
	LookPath       func(string) (string, error)
	FFmpegBinary   string
	ExifToolBinary string
}

type livePackageManifest struct {
	PairingID   string `json:"pairing_id"`
	PhotoFile   string `json:"photo_file"`
	MotionFile  string `json:"motion_file"`
	DurationMS  int    `json:"duration_ms"`
	FPS         int    `json:"fps"`
	CoverFrame  string `json:"cover_frame"`
	CoverSource string `json:"cover_source"`
	PhotoFormat string `json:"photo_format"`
}

func (b livePackageBuilder) CheckAvailable() error {
	lookPath := b.lookPath()
	if _, err := lookPath(b.ffmpegBinary()); err != nil {
		return fmt.Errorf("%s not available: %w", b.ffmpegBinary(), err)
	}
	if _, err := lookPath(b.exiftoolBinary()); err != nil {
		return fmt.Errorf("%s not available: %w", b.exiftoolBinary(), err)
	}
	return nil
}

func (b livePackageBuilder) Build(task livePackageTask) error {
	if stringsTrim(task.OutputDir) == "" {
		return fmt.Errorf("build live package %s: output dir is required", task.PageName)
	}
	if len(task.FramePaths) == 0 {
		return fmt.Errorf("build live package %s: frame paths are required", task.PageName)
	}
	if stringsTrim(task.FramePattern) == "" {
		return fmt.Errorf("build live package %s: frame pattern is required", task.PageName)
	}
	if task.FPS <= 0 {
		return fmt.Errorf("build live package %s: invalid fps %d", task.PageName, task.FPS)
	}
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		return fmt.Errorf("build live package %s: create output dir: %w", task.PageName, err)
	}

	coverIndex := liveCoverFrameIndex(task.FramePaths, task.CoverFrame)
	coverSource := task.FramePaths[coverIndex]
	coverPath := filepath.Join(task.OutputDir, liveCoverFilename)
	motionPath := filepath.Join(task.OutputDir, liveMotionFilename)
	pairingID, err := newLivePairingID()
	if err != nil {
		return fmt.Errorf("build live package %s: generate pairing id: %w", task.PageName, err)
	}

	if err := b.runner().Run(b.ffmpegBinary(), "-y", "-i", coverSource, "-frames:v", "1", coverPath); err != nil {
		return fmt.Errorf("build live package %s: create cover image: %w", task.PageName, err)
	}
	if err := ensureOutputFile(coverPath); err != nil {
		return fmt.Errorf("build live package %s: verify cover image: %w", task.PageName, err)
	}

	motionFrameCount := liveMotionFrameCount(task.FramePaths)
	if err := b.runner().Run(b.ffmpegBinary(), "-y", "-framerate", liveMotionFramerate(task.DurationMS, motionFrameCount, task.FPS), "-i", task.FramePattern, "-frames:v", strconv.Itoa(motionFrameCount), "-c:v", "libx264", "-pix_fmt", "yuv420p", motionPath); err != nil {
		return fmt.Errorf("build live package %s: create motion video: %w", task.PageName, err)
	}
	if err := ensureOutputFile(motionPath); err != nil {
		return fmt.Errorf("build live package %s: verify motion video: %w", task.PageName, err)
	}

	if err := b.runner().Run(b.exiftoolBinary(), "-overwrite_original", "-ContentIdentifier="+pairingID, coverPath); err != nil {
		return fmt.Errorf("build live package %s: write live metadata: %w", task.PageName, err)
	}
	if err := b.runner().Run(b.exiftoolBinary(), "-overwrite_original", "-ContentIdentifier="+pairingID, motionPath); err != nil {
		return fmt.Errorf("build live package %s: write live metadata: %w", task.PageName, err)
	}

	manifest := livePackageManifest{
		PairingID:   pairingID,
		PhotoFile:   liveCoverFilename,
		MotionFile:  liveMotionFilename,
		DurationMS:  task.DurationMS,
		FPS:         task.FPS,
		CoverFrame:  task.CoverFrame,
		CoverSource: filepath.Base(coverSource),
		PhotoFormat: task.PhotoFormat,
	}
	manifestPath := filepath.Join(task.OutputDir, "manifest.json")
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("build live package %s: marshal manifest: %w", task.PageName, err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		return fmt.Errorf("build live package %s: write manifest: %w", task.PageName, err)
	}
	return nil
}

func (b livePackageBuilder) ffmpegBinary() string {
	if b.FFmpegBinary == "" {
		return "ffmpeg"
	}
	return b.FFmpegBinary
}

func (b livePackageBuilder) exiftoolBinary() string {
	if b.ExifToolBinary == "" {
		return "exiftool"
	}
	return b.ExifToolBinary
}

func (b livePackageBuilder) lookPath() func(string) (string, error) {
	if b.LookPath != nil {
		return b.LookPath
	}
	return exec.LookPath
}

func (b livePackageBuilder) runner() CommandRunner {
	if b.Runner != nil {
		return b.Runner
	}
	return execRunner{}
}

func liveMotionFrameCount(framePaths []string) int {
	count := len(framePaths)
	if count <= 1 {
		if count < 0 {
			return 0
		}
		return count
	}
	return count - 1
}

func liveMotionFramerate(durationMS int, frameCount int, fallbackFPS int) string {
	if durationMS <= 0 || frameCount <= 0 {
		return strconv.Itoa(fallbackFPS)
	}
	fps := float64(frameCount) * 1000 / float64(durationMS)
	if fps <= 0 {
		return strconv.Itoa(fallbackFPS)
	}
	return strconv.FormatFloat(fps, 'f', 6, 64)
}

func liveCoverFrameIndex(framePaths []string, strategy string) int {
	if len(framePaths) == 0 {
		return 0
	}
	if strategy == liveCoverFrameFirst {
		return 0
	}
	if strategy == liveCoverFrameLast {
		return len(framePaths) - 1
	}
	return len(framePaths) / 2
}

func newLivePairingID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func ensureOutputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s is empty", path)
	}
	return nil
}

func stringsTrim(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\n' || value[0] == '\t' || value[0] == '\r') {
		value = value[1:]
	}
	for len(value) > 0 {
		last := value[len(value)-1]
		if last != ' ' && last != '\n' && last != '\t' && last != '\r' {
			break
		}
		value = value[:len(value)-1]
	}
	return value
}
