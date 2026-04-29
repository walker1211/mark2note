package render

import (
	"fmt"
	"path/filepath"

	"github.com/walker1211/mark2note/internal/deck"
)

type frameSpec struct {
	Path       string
	DurationMS int
}

type animatedSequenceSpec struct {
	FramePattern string
	FPS          int
}

type animatedCaptureTask struct {
	pageName      string
	frameURIs     []string
	framePaths    []string
	framePattern  string
	outputPath    string
	liveOutputDir string
}

func buildAnimatedCaptureTasks(pages []deck.Page, outDir string, opts normalizedAnimatedOptions) []animatedCaptureTask {
	if !opts.Enabled || len(opts.FrameMS) == 0 {
		return nil
	}

	tasks := make([]animatedCaptureTask, 0, len(pages))
	for _, page := range pages {
		framePaths := make([]string, 0, len(opts.FrameMS))
		frameURIs := make([]string, 0, len(opts.FrameMS))
		framePattern := filepath.Join(outDir, ".animated", page.Name, "frame-%04d.png")
		for index, ms := range opts.FrameMS {
			framePath := filepath.Join(outDir, ".animated", page.Name, fmt.Sprintf("frame-%04d.png", index))
			framePaths = append(framePaths, framePath)
			htmlURI := fileURI(filepath.Join(outDir, page.Name+".html"))
			frameURIs = append(frameURIs, fmt.Sprintf("%s?animated_ms=%d", htmlURI, ms))
		}
		outputExt := ".webp"
		if opts.Format == animatedFormatMP4 {
			outputExt = ".mp4"
		}
		tasks = append(tasks, animatedCaptureTask{
			pageName:      page.Name,
			frameURIs:     frameURIs,
			framePaths:    framePaths,
			framePattern:  framePattern,
			outputPath:    filepath.Join(outDir, page.Name+outputExt),
			liveOutputDir: filepath.Join(outDir, page.Name+".live"),
		})
	}
	return tasks
}

func frameSpecsForTask(task animatedCaptureTask, frameMS []int) []frameSpec {
	frames := make([]frameSpec, 0, len(task.framePaths))
	for i, path := range task.framePaths {
		if i+1 >= len(frameMS) {
			break
		}
		duration := frameMS[i+1] - frameMS[i]
		if duration <= 0 {
			continue
		}
		frames = append(frames, frameSpec{Path: path, DurationMS: duration})
	}
	return frames
}
