package render

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeAnimatedOptionsAcceptsMP4(t *testing.T) {
	got, warning := normalizeAnimatedOptions(animatedOptions{Enabled: true, Format: "mp4", DurationMS: 2400, FPS: 8})
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if got.Format != "mp4" {
		t.Fatalf("Format = %q, want %q", got.Format, "mp4")
	}
}

func TestNormalizeAnimatedOptionsRejectsUnsupportedFormat(t *testing.T) {
	got, warning := normalizeAnimatedOptions(animatedOptions{Enabled: true, Format: "gif", DurationMS: 2400, FPS: 8})
	if got.Enabled {
		t.Fatalf("Enabled = true, want false")
	}
	if !strings.Contains(warning, "animated export skipped") || !strings.Contains(warning, "format") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestNormalizeAnimatedOptionsRejectsOutOfRangeValues(t *testing.T) {
	got, warning := normalizeAnimatedOptions(animatedOptions{Enabled: true, Format: "webp", DurationMS: 800, FPS: 20})
	if got.Enabled {
		t.Fatalf("Enabled = true, want false")
	}
	if !strings.Contains(warning, "duration_ms") || !strings.Contains(warning, "fps") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestNormalizeAnimatedOptionsNormalizesFormatAndBuildsFrames(t *testing.T) {
	got, warning := normalizeAnimatedOptions(animatedOptions{Enabled: true, Format: " WebP ", DurationMS: 2400, FPS: 8})
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if got.Format != "webp" {
		t.Fatalf("Format = %q, want %q", got.Format, "webp")
	}
	if len(got.FrameMS) == 0 {
		t.Fatalf("FrameMS = %#v, want non-empty", got.FrameMS)
	}
}

func TestNormalizeLiveOptionsFallsBackToJPEGWithWarning(t *testing.T) {
	got, warnings := normalizeLiveOptions(liveOptions{Enabled: true, PhotoFormat: "webp", CoverFrame: "middle"})
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if got.PhotoFormat != "jpeg" {
		t.Fatalf("PhotoFormat = %q, want jpeg", got.PhotoFormat)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "photo_format") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestNormalizeLiveOptionsAcceptsLastCoverFrame(t *testing.T) {
	got, warnings := normalizeLiveOptions(liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "last"})
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if got.CoverFrame != "last" {
		t.Fatalf("CoverFrame = %q, want last", got.CoverFrame)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want empty", warnings)
	}
}

func TestNormalizeLiveOptionsFallsBackToMiddleWithWarning(t *testing.T) {
	got, warnings := normalizeLiveOptions(liveOptions{Enabled: true, PhotoFormat: "jpeg", CoverFrame: "ending"})
	if !got.Enabled {
		t.Fatalf("Enabled = false, want true")
	}
	if got.CoverFrame != "middle" {
		t.Fatalf("CoverFrame = %q, want middle", got.CoverFrame)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "cover_frame") {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestFrameTimesAlwaysIncludeZeroAndDuration(t *testing.T) {
	got := frameTimesMS(2400, 8)
	if got[0] != 0 {
		t.Fatalf("first frame = %d, want 0", got[0])
	}
	if got[len(got)-1] != 2400 {
		t.Fatalf("last frame = %d, want 2400", got[len(got)-1])
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("frame times should be strictly increasing: %#v", got)
		}
	}
}

func TestFrameTimesDistributeWithoutAccumulatedTruncationDrift(t *testing.T) {
	got := frameTimesMS(4000, 12)
	want := []int{0, 333, 666, 1083}
	if !reflect.DeepEqual(got[:4], want) {
		t.Fatalf("leading frames = %#v, want %#v", got[:4], want)
	}
	if got[len(got)-1] != 4000 {
		t.Fatalf("last frame = %d, want 4000", got[len(got)-1])
	}
}

func TestFrameTimesCapsRenderedFramesForPerformance(t *testing.T) {
	got := frameTimesMS(2400, 8)
	want := []int{0, 125, 375, 625, 875, 1125, 1250, 1500, 1750, 2000, 2250, 2400}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frameTimesMS() = %#v, want %#v", got, want)
	}
}

func TestDefaultVariantPresetExistsForEverySupportedVariant(t *testing.T) {
	for _, variant := range []string{"cover", "quote", "image-caption", "bullets", "compare", "gallery-steps", "ending"} {
		if _, ok := defaultAnimatedPreset(variant); !ok {
			t.Fatalf("missing preset for %q", variant)
		}
	}
}

func TestDefaultVariantPresetMatchesKeySpecMappings(t *testing.T) {
	cover, ok := defaultAnimatedPreset("cover")
	if !ok {
		t.Fatalf("missing cover preset")
	}
	if cover.Badge != "fade-in" || cover.Title != "fade-up" || cover.Subtitle != "fade-up" || cover.CTA != "fade-in" || cover.Author != "fade-in" {
		t.Fatalf("cover preset = %#v", cover)
	}

	imageCaption, ok := defaultAnimatedPreset("image-caption")
	if !ok {
		t.Fatalf("missing image-caption preset")
	}
	if imageCaption.Image != "scale-in" || imageCaption.Body != "fade-up" {
		t.Fatalf("image-caption preset = %#v", imageCaption)
	}

	bullets, ok := defaultAnimatedPreset("bullets")
	if !ok {
		t.Fatalf("missing bullets preset")
	}
	if bullets.Items != "reveal" {
		t.Fatalf("bullets preset = %#v", bullets)
	}

	compare, ok := defaultAnimatedPreset("compare")
	if !ok {
		t.Fatalf("missing compare preset")
	}
	if compare.CompareLabels != "fade-in" || compare.CompareRows != "reveal" {
		t.Fatalf("compare preset = %#v", compare)
	}
}
