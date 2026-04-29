package render

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/walker1211/mark2note/internal/deck"
)

func TestAnimatedCaptureTasksUseAnimatedMSQueryAndStableNames(t *testing.T) {
	outDir := t.TempDir()
	page := deck.Page{Name: "p01-cover", Variant: "cover"}
	tasks := buildAnimatedCaptureTasks([]deck.Page{page}, outDir, normalizedAnimatedOptions{
		Enabled:    true,
		Format:     "webp",
		DurationMS: 2400,
		FPS:        8,
		FrameMS:    []int{0, 125, 2400},
	})
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	gotFrames := tasks[0].framePaths
	want := []string{
		filepath.Join(outDir, ".animated", "p01-cover", "frame-0000.png"),
		filepath.Join(outDir, ".animated", "p01-cover", "frame-0001.png"),
		filepath.Join(outDir, ".animated", "p01-cover", "frame-0002.png"),
	}
	if !reflect.DeepEqual(gotFrames, want) {
		t.Fatalf("framePaths = %#v, want %#v", gotFrames, want)
	}
	if tasks[0].framePattern != filepath.Join(outDir, ".animated", "p01-cover", "frame-%04d.png") {
		t.Fatalf("framePattern = %q", tasks[0].framePattern)
	}
	if !strings.Contains(tasks[0].frameURIs[1], "animated_ms=125") {
		t.Fatalf("frame URI = %q", tasks[0].frameURIs[1])
	}
}

func TestAnimatedCaptureTasksUseMP4OutputWhenFormatIsMP4(t *testing.T) {
	outDir := t.TempDir()
	page := deck.Page{Name: "p01-cover", Variant: "cover"}
	tasks := buildAnimatedCaptureTasks([]deck.Page{page}, outDir, normalizedAnimatedOptions{
		Enabled:    true,
		Format:     "mp4",
		DurationMS: 2400,
		FPS:        8,
		FrameMS:    []int{0, 125, 2400},
	})
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].outputPath != filepath.Join(outDir, "p01-cover.mp4") {
		t.Fatalf("outputPath = %q", tasks[0].outputPath)
	}
}

func TestFrameSpecsForTaskKeepsTotalDurationEqualToTimeline(t *testing.T) {
	task := animatedCaptureTask{framePaths: []string{"a.png", "b.png", "c.png"}}
	got := frameSpecsForTask(task, []int{0, 125, 2400})
	want := []frameSpec{{Path: "a.png", DurationMS: 125}, {Path: "b.png", DurationMS: 2275}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frameSpecsForTask() = %#v, want %#v", got, want)
	}
}
