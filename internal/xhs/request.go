package xhs

import (
	"fmt"
	"strings"
	"time"
)

const scheduleTimeLayout = "2006-01-02 15:04:05"

type PublishMode string

const (
	PublishModeOnlySelf PublishMode = "only-self"
	PublishModeSchedule PublishMode = "schedule"
)

type MediaKind string

const (
	MediaKindStandard MediaKind = "standard"
	MediaKindLive     MediaKind = "live"
)

type PublishRequest struct {
	Account          string
	Title            string
	Content          string
	Tags             []string
	Mode             PublishMode
	ScheduleTime     *time.Time
	MediaKind        MediaKind
	ImagePaths       []string
	Live             LivePublishSource
	DeclareOriginal  bool
	AllowContentCopy bool
}

type LivePublishSource struct {
	ReportPath   string
	RequestedSet []string
	Resolved     *ResolvedLiveSource
}

type PublishResult struct {
	TargetAccount     string
	Mode              PublishMode
	MediaKind         MediaKind
	OnlySelfPublished bool
	ScheduleTime      *time.Time
	AttachedCount     int
	AttachedItems     []string
	BrowserKept       bool
	Warnings          []string
}

func ValidateMode(input string) (PublishMode, error) {
	switch PublishMode(strings.TrimSpace(input)) {
	case "", PublishModeOnlySelf:
		return PublishModeOnlySelf, nil
	case PublishModeSchedule:
		return PublishModeSchedule, nil
	default:
		return "", fmt.Errorf("mode must be only-self or schedule")
	}
}

func (r PublishRequest) Validate(now time.Time) error {
	if strings.TrimSpace(r.Account) == "" {
		return fmt.Errorf("account is required")
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(r.Content) == "" {
		return fmt.Errorf("content is required")
	}
	switch r.Mode {
	case PublishModeOnlySelf:
		if r.ScheduleTime != nil {
			return fmt.Errorf("only-self mode forbids schedule time")
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
		return fmt.Errorf("mode must be only-self or schedule")
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
