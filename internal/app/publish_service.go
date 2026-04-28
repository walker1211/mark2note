package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/walker1211/mark2note/internal/xhs"
)

type PublishOptions struct {
	Account          string
	Title            string
	TitleFile        string
	Content          string
	ContentFile      string
	Tags             []string
	Mode             string
	ScheduleAt       string
	ImagePaths       []string
	LiveReportPath   string
	LivePages        []string
	ChromePath       string
	Headless         bool
	ProfileDir       string
	ChromeArgs       []string
	DeclareOriginal  bool
	AllowContentCopy bool
}

type PublishRuntimeOptions struct {
	ChromePath string
	Headless   bool
	ProfileDir string
	ChromeArgs []string
}

type PublishOrchestrator interface {
	Publish(request xhs.PublishRequest, options PublishRuntimeOptions) (xhs.PublishResult, error)
}

type PublishService struct {
	ReadFile        func(string) ([]byte, error)
	Now             func() time.Time
	NewOrchestrator func(PublishRuntimeOptions) PublishOrchestrator
}

type PublishResult struct {
	Request xhs.PublishRequest
	Result  xhs.PublishResult
}

var (
	ErrPublishRequestInvalid = errors.New("publish request invalid")
	ErrPublishReadInput      = errors.New("read publish input failed")
	ErrPublishExecute        = errors.New("publish execute failed")
)

func (s PublishService) Publish(opts PublishOptions) (PublishResult, error) {
	title, err := s.resolveTextInput(opts.Title, opts.TitleFile, "title")
	if err != nil {
		return PublishResult{}, err
	}
	content, err := s.resolveTextInput(opts.Content, opts.ContentFile, "content")
	if err != nil {
		return PublishResult{}, err
	}
	mode, err := xhs.ValidateMode(opts.Mode)
	if err != nil {
		return PublishResult{}, fmt.Errorf("%w: %v", ErrPublishRequestInvalid, err)
	}
	now := s.effectiveNow()()
	scheduleTime, err := xhs.ParseScheduleTime(opts.ScheduleAt, now)
	if err != nil {
		return PublishResult{}, fmt.Errorf("%w: %v", ErrPublishRequestInvalid, err)
	}
	request, err := buildPublishRequest(opts, title, content, mode, scheduleTime)
	if err != nil {
		return PublishResult{}, err
	}
	if err := request.Validate(now); err != nil {
		return PublishResult{}, fmt.Errorf("%w: %v", ErrPublishRequestInvalid, err)
	}
	runtime := PublishRuntimeOptions{ChromePath: strings.TrimSpace(opts.ChromePath), Headless: opts.Headless, ProfileDir: strings.TrimSpace(opts.ProfileDir), ChromeArgs: trimOptionalSlice(opts.ChromeArgs)}
	result, err := s.effectiveNewOrchestrator()(runtime).Publish(request, runtime)
	if err != nil {
		return PublishResult{Request: request, Result: result}, fmt.Errorf("%w: %w", ErrPublishExecute, err)
	}
	return PublishResult{Request: request, Result: result}, nil
}

func buildPublishRequest(opts PublishOptions, title string, content string, mode xhs.PublishMode, scheduleTime *time.Time) (xhs.PublishRequest, error) {
	request := xhs.PublishRequest{
		Account:          strings.TrimSpace(opts.Account),
		Title:            title,
		Content:          content,
		Tags:             trimSlice(opts.Tags),
		Mode:             mode,
		ScheduleTime:     scheduleTime,
		DeclareOriginal:  opts.DeclareOriginal,
		AllowContentCopy: opts.AllowContentCopy,
	}
	imagePaths := trimSlice(opts.ImagePaths)
	livePages := trimSlice(opts.LivePages)
	liveReport := strings.TrimSpace(opts.LiveReportPath)
	hasImages := len(imagePaths) > 0
	hasLive := liveReport != ""
	if hasImages == hasLive {
		return xhs.PublishRequest{}, fmt.Errorf("%w: exactly one media source is required", ErrPublishRequestInvalid)
	}
	if !hasLive && len(livePages) > 0 {
		return xhs.PublishRequest{}, fmt.Errorf("%w: --live-pages requires --live-report", ErrPublishRequestInvalid)
	}
	if hasImages {
		request.MediaKind = xhs.MediaKindStandard
		request.ImagePaths = imagePaths
		return request, nil
	}
	resolved, err := xhs.ResolveLiveSource(liveReport, livePages)
	if err != nil {
		return xhs.PublishRequest{}, fmt.Errorf("%w: %v", ErrPublishRequestInvalid, err)
	}
	request.MediaKind = xhs.MediaKindLive
	request.Live = xhs.LivePublishSource{ReportPath: liveReport, RequestedSet: livePages, Resolved: &resolved}
	return request, nil
}

func (s PublishService) resolveTextInput(inline string, filePath string, fieldName string) (string, error) {
	trimmedInline := strings.TrimSpace(inline)
	trimmedFile := strings.TrimSpace(filePath)
	hasInline := trimmedInline != ""
	hasFile := trimmedFile != ""
	if hasInline == hasFile {
		return "", fmt.Errorf("%w: exactly one of --%s / --%s-file is required", ErrPublishRequestInvalid, fieldName, fieldName)
	}
	if hasInline {
		return trimmedInline, nil
	}
	content, err := s.effectiveReadFile()(trimmedFile)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPublishReadInput, err)
	}
	resolved := strings.TrimSpace(string(content))
	if resolved == "" {
		return "", fmt.Errorf("%w: %s is required", ErrPublishRequestInvalid, fieldName)
	}
	return resolved, nil
}

func (s PublishService) effectiveReadFile() func(string) ([]byte, error) {
	if s.ReadFile != nil {
		return s.ReadFile
	}
	return os.ReadFile
}

func (s PublishService) effectiveNow() func() time.Time {
	if s.Now != nil {
		return s.Now
	}
	return time.Now
}

func (s PublishService) effectiveNewOrchestrator() func(PublishRuntimeOptions) PublishOrchestrator {
	if s.NewOrchestrator != nil {
		return s.NewOrchestrator
	}
	return func(runtime PublishRuntimeOptions) PublishOrchestrator {
		return defaultPublishOrchestrator{runtime: runtime}
	}
}

type defaultPublishOrchestrator struct {
	runtime PublishRuntimeOptions
}

func (o defaultPublishOrchestrator) Publish(request xhs.PublishRequest, _ PublishRuntimeOptions) (xhs.PublishResult, error) {
	session := xhs.NewBrowserSession(xhs.SessionOptions{
		Account:    request.Account,
		ChromePath: o.runtime.ChromePath,
		Headless:   o.runtime.Headless,
		ProfileDir: o.runtime.ProfileDir,
		ChromeArgs: o.runtime.ChromeArgs,
	})
	return xhs.NewOrchestrator(session).Publish(context.Background(), request)
}

func trimSlice(values []string) []string {
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

func trimOptionalSlice(values []string) []string {
	if values == nil {
		return nil
	}
	return trimSlice(values)
}
