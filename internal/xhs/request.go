package xhs

import (
	"fmt"
	"strings"
	"time"
)

const scheduleTimeLayout = "2006-01-02 15:04:05"

type PublishMode string

const (
	PublishModeImmediate PublishMode = "immediate"
	PublishModeOnlySelf  PublishMode = PublishModeImmediate
	PublishModeSchedule  PublishMode = "schedule"
)

type PublishVisibility string

const (
	PublishVisibilityOnlySelf PublishVisibility = "only-self"
	PublishVisibilityPublic   PublishVisibility = "public"
)

const OriginalDeclarationTypeAIGenerated = "ai_generated"

type MediaKind string

const (
	MediaKindStandard MediaKind = "standard"
	MediaKindLive     MediaKind = "live"
)

type PublishRequest struct {
	Account                 string
	Title                   string
	Content                 string
	Tags                    []string
	Mode                    PublishMode
	Visibility              PublishVisibility
	ScheduleTime            *time.Time
	Collection              string
	MediaKind               MediaKind
	ImagePaths              []string
	Live                    LivePublishSource
	DeclareOriginal         bool
	OriginalDeclarationType string
	AllowContentCopy        bool
	StopBeforeSubmit        bool
}

type LivePublishSource struct {
	ReportPath   string
	RequestedSet []string
	Resolved     *ResolvedLiveSource
}

type PublishResult struct {
	TargetAccount       string
	Mode                PublishMode
	MediaKind           MediaKind
	OnlySelfPublished   bool
	ScheduleTime        *time.Time
	AttachedCount       int
	AttachedItems       []string
	BrowserKept         bool
	StoppedBeforeSubmit bool
	Warnings            []string
}

func ValidateMode(input string) (PublishMode, error) {
	switch PublishMode(strings.TrimSpace(input)) {
	case "", PublishModeImmediate:
		return PublishModeImmediate, nil
	case PublishModeSchedule:
		return PublishModeSchedule, nil
	default:
		return "", fmt.Errorf("mode must be immediate or schedule")
	}
}

func ValidateVisibility(input string) (PublishVisibility, error) {
	switch PublishVisibility(strings.TrimSpace(input)) {
	case "", PublishVisibilityOnlySelf:
		return PublishVisibilityOnlySelf, nil
	case PublishVisibilityPublic:
		return PublishVisibilityPublic, nil
	default:
		return "", fmt.Errorf("visibility must be public or only-self")
	}
}

func ValidateOriginalDeclarationType(input string) (string, error) {
	switch trimmed := strings.TrimSpace(input); trimmed {
	case "", OriginalDeclarationTypeAIGenerated:
		return trimmed, nil
	default:
		return "", fmt.Errorf("original declaration type must be empty or %s", OriginalDeclarationTypeAIGenerated)
	}
}

func (r PublishRequest) Validate(now time.Time) error {
	if strings.TrimSpace(r.Account) == "" {
		return fmt.Errorf("account is required")
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(r.Content) == "" && len(trimmedNonEmpty(r.Tags)) == 0 {
		return fmt.Errorf("content or tags are required")
	}
	if _, err := ValidateVisibility(string(r.Visibility)); err != nil {
		return err
	}
	declarationType, err := ValidateOriginalDeclarationType(r.OriginalDeclarationType)
	if err != nil {
		return err
	}
	if declarationType != "" && !r.DeclareOriginal {
		return fmt.Errorf("original declaration type requires declare_original")
	}
	switch r.Mode {
	case PublishModeImmediate:
		if r.ScheduleTime != nil {
			return fmt.Errorf("immediate mode forbids schedule time")
		}
	case PublishModeSchedule:
		if r.ScheduleTime == nil {
			return fmt.Errorf("schedule mode requires schedule time")
		}
		schedule := r.ScheduleTime.In(shanghaiLocation())
		current := now.In(shanghaiLocation())
		if !schedule.After(current) {
			return fmt.Errorf("schedule time must be in the future")
		}
		if schedule.Sub(current) < 15*time.Minute {
			return fmt.Errorf("schedule lead time must be at least 15 minutes")
		}
	default:
		return fmt.Errorf("mode must be immediate or schedule")
	}

	switch r.MediaKind {
	case MediaKindStandard:
		if len(trimmedNonEmpty(r.ImagePaths)) == 0 {
			return fmt.Errorf("at least one image path is required")
		}
		if strings.TrimSpace(r.Live.ReportPath) != "" || len(r.Live.RequestedSet) > 0 || r.Live.Resolved != nil {
			return fmt.Errorf("standard media request must not include live source")
		}
	case MediaKindLive:
		if strings.TrimSpace(r.Live.ReportPath) == "" {
			return fmt.Errorf("live report path is required")
		}
		if len(trimmedNonEmpty(r.Live.RequestedSet)) != len(r.Live.RequestedSet) {
			return fmt.Errorf("live pages must not contain blank page names")
		}
		if r.Live.Resolved == nil {
			return fmt.Errorf("live source must be resolved before publish")
		}
		if len(r.Live.Resolved.Items) == 0 {
			return fmt.Errorf("resolved live source must contain at least one item")
		}
		if len(trimmedNonEmpty(r.ImagePaths)) > 0 {
			return fmt.Errorf("live media request must not include image paths")
		}
	default:
		return fmt.Errorf("media kind must be standard or live")
	}
	return nil
}

func ParseScheduleTime(input string, now time.Time) (*time.Time, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(scheduleTimeLayout, trimmed, shanghaiLocation())
	if err != nil {
		return nil, fmt.Errorf("schedule time must use format YYYY-MM-DD HH:MM:SS in Asia/Shanghai")
	}
	schedule := parsed.In(shanghaiLocation())
	current := now.In(shanghaiLocation())
	if !schedule.After(current) {
		return nil, fmt.Errorf("schedule time must be in the future")
	}
	if schedule.Sub(current) < 15*time.Minute {
		return nil, fmt.Errorf("schedule lead time must be at least 15 minutes")
	}
	return &parsed, nil
}

func shanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func trimmedNonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
