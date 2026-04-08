package render

import "strings"

const (
	animatedFormatWebP   = "webp"
	animatedFormatMP4    = "mp4"
	livePhotoFormatJPEG  = "jpeg"
	liveCoverFrameFirst  = "first"
	liveCoverFrameMiddle = "middle"
	liveCoverFrameLast   = "last"
	minAnimatedDuration  = 1200
	maxAnimatedDuration  = 4000
	minAnimatedFPS       = 4
	maxAnimatedFPS       = 12
	maxAnimatedFrameTime = 12
)

type animatedOptions struct {
	Enabled    bool
	Format     string
	DurationMS int
	FPS        int
}

type AnimatedOptions = animatedOptions

type liveOptions struct {
	Enabled     bool
	PhotoFormat string
	CoverFrame  string
	Assemble    bool
	OutputDir   string
}

type LiveOptions = liveOptions

type normalizedAnimatedOptions struct {
	Enabled    bool
	Format     string
	DurationMS int
	FPS        int
	FrameMS    []int
}

type normalizedLiveOptions struct {
	Enabled     bool
	PhotoFormat string
	CoverFrame  string
	Assemble    bool
	OutputDir   string
}

type animatedPreset struct {
	Badge         string
	Title         string
	Subtitle      string
	Body          string
	Quote         string
	Note          string
	Tip           string
	CTA           string
	Author        string
	Image         string
	Items         string
	CompareLabels string
	CompareRows   string
	Steps         string
	Images        string
}

func normalizeAnimatedOptions(in animatedOptions) (normalizedAnimatedOptions, string) {
	if !in.Enabled {
		return normalizedAnimatedOptions{}, ""
	}

	format := strings.ToLower(strings.TrimSpace(in.Format))
	problems := make([]string, 0, 3)
	if format != animatedFormatWebP && format != animatedFormatMP4 {
		problems = append(problems, "format")
	}
	if in.DurationMS < minAnimatedDuration || in.DurationMS > maxAnimatedDuration {
		problems = append(problems, "duration_ms")
	}
	if in.FPS < minAnimatedFPS || in.FPS > maxAnimatedFPS {
		problems = append(problems, "fps")
	}
	if len(problems) > 0 {
		return normalizedAnimatedOptions{}, "animated export skipped: invalid " + strings.Join(problems, ", ")
	}

	return normalizedAnimatedOptions{
		Enabled:    true,
		Format:     format,
		DurationMS: in.DurationMS,
		FPS:        in.FPS,
		FrameMS:    frameTimesMS(in.DurationMS, in.FPS),
	}, ""
}

func normalizeLiveOptions(in liveOptions) (normalizedLiveOptions, []string) {
	warnings := make([]string, 0, 3)
	if !in.Enabled {
		if in.Assemble {
			warnings = append(warnings, "live assemble ignored: live export is disabled")
		}
		return normalizedLiveOptions{}, warnings
	}

	photoFormat := strings.ToLower(strings.TrimSpace(in.PhotoFormat))
	if photoFormat == "" {
		photoFormat = livePhotoFormatJPEG
	}
	if photoFormat != livePhotoFormatJPEG {
		warnings = append(warnings, "live export: unsupported photo_format, fallback to jpeg")
		photoFormat = livePhotoFormatJPEG
	}

	coverFrame := strings.ToLower(strings.TrimSpace(in.CoverFrame))
	if coverFrame == "" {
		coverFrame = liveCoverFrameMiddle
	}
	if coverFrame != liveCoverFrameFirst && coverFrame != liveCoverFrameMiddle && coverFrame != liveCoverFrameLast {
		warnings = append(warnings, "live export: unsupported cover_frame, fallback to middle")
		coverFrame = liveCoverFrameMiddle
	}

	return normalizedLiveOptions{
		Enabled:     true,
		PhotoFormat: photoFormat,
		CoverFrame:  coverFrame,
		Assemble:    in.Assemble,
		OutputDir:   strings.TrimSpace(in.OutputDir),
	}, warnings
}

func frameTimesMS(durationMS int, fps int) []int {
	if durationMS <= 0 || fps <= 0 {
		return nil
	}

	frames := make([]int, 0, durationMS*fps/1000+2)
	for index := 0; ; index++ {
		ms := index * 1000 / fps
		if ms >= durationMS {
			break
		}
		frames = append(frames, ms)
	}
	if len(frames) == 0 || frames[len(frames)-1] != durationMS {
		frames = append(frames, durationMS)
	}
	return capFrameTimes(frames, maxAnimatedFrameTime)
}

func capFrameTimes(frameMS []int, limit int) []int {
	if len(frameMS) <= limit || limit < 2 {
		return frameMS
	}

	capped := make([]int, 0, limit)
	lastIndex := len(frameMS) - 1
	for index := 0; index < limit; index++ {
		position := index * lastIndex / (limit - 1)
		if len(capped) == 0 || capped[len(capped)-1] != frameMS[position] {
			capped = append(capped, frameMS[position])
		}
	}
	if capped[len(capped)-1] != frameMS[lastIndex] {
		capped = append(capped, frameMS[lastIndex])
	}
	return capped
}

func defaultAnimatedPreset(variant string) (animatedPreset, bool) {
	switch strings.TrimSpace(variant) {
	case "cover":
		return animatedPreset{
			Badge:    "fade-in",
			Title:    "fade-up",
			Subtitle: "fade-up",
			CTA:      "fade-in",
			Author:   "fade-in",
		}, true
	case "quote":
		return animatedPreset{
			Title: "fade-in",
			Quote: "fade-up",
			Note:  "fade-in",
			Tip:   "fade-in",
			CTA:   "fade-in",
		}, true
	case "image-caption":
		return animatedPreset{
			Title: "fade-in",
			Body:  "fade-up",
			Image: "scale-in",
			CTA:   "fade-in",
		}, true
	case "bullets":
		return animatedPreset{
			Title: "fade-in",
			Items: "reveal",
			CTA:   "fade-in",
		}, true
	case "compare":
		return animatedPreset{
			Title:         "fade-in",
			CompareLabels: "fade-in",
			CompareRows:   "reveal",
			CTA:           "fade-in",
		}, true
	case "gallery-steps":
		return animatedPreset{
			Title:  "fade-in",
			Steps:  "reveal",
			Images: "fade-in",
			CTA:    "fade-in",
		}, true
	case "ending":
		return animatedPreset{
			Title: "fade-in",
			Body:  "fade-up",
			CTA:   "fade-in",
		}, true
	default:
		return animatedPreset{}, false
	}
}
