package poster

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultEnrichConcurrency = 10
	defaultTitleTimeout      = 30 * time.Second
)

type EnrichOptions struct {
	Providers    []Provider
	Concurrency  int
	TitleTimeout time.Duration
}

type EnrichReport struct {
	Titles   []string
	Missing  []string
	Warnings []string
}

func EnrichMarkdown(ctx context.Context, markdown string, opts EnrichOptions) (Manifest, EnrichReport, error) {
	providers := opts.Providers
	if providers == nil {
		providers = DefaultProviders(nil, nil)
	}
	manifest := Manifest{Posters: map[string]PosterAsset{}}
	report := EnrichReport{Titles: ExtractTitlesFromMarkdown(markdown)}
	if len(report.Titles) == 0 {
		return manifest, report, nil
	}
	if len(providers) == 0 {
		return manifest, report, fmt.Errorf("no poster providers enabled")
	}
	results := enrichTitles(ctx, report.Titles, providers, opts)
	for i, title := range report.Titles {
		result := results[i]
		report.Warnings = append(report.Warnings, result.Warnings...)
		if !result.Found {
			report.Missing = append(report.Missing, title)
			continue
		}
		manifest.Posters[title] = result.Asset
	}
	return manifest, report, nil
}

type enrichTitleResult struct {
	Asset    PosterAsset
	Found    bool
	Warnings []string
}

type enrichTitleJob struct {
	Index int
	Title string
}

func enrichTitles(ctx context.Context, titles []string, providers []Provider, opts EnrichOptions) []enrichTitleResult {
	results := make([]enrichTitleResult, len(titles))
	jobs := make(chan enrichTitleJob, len(titles))
	workers := enrichConcurrency(opts.Concurrency, len(titles))
	timeout := enrichTitleTimeout(opts.TitleTimeout)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				results[job.Index] = enrichTitle(ctx, job.Title, providers, timeout)
			}
		}()
	}
	for i, title := range titles {
		jobs <- enrichTitleJob{Index: i, Title: title}
	}
	close(jobs)
	wg.Wait()
	return results
}

func enrichConcurrency(configured int, titleCount int) int {
	if configured <= 0 {
		configured = defaultEnrichConcurrency
	}
	if configured > titleCount {
		return titleCount
	}
	return configured
}

func enrichTitleTimeout(configured time.Duration) time.Duration {
	if configured <= 0 {
		return defaultTitleTimeout
	}
	return configured
}

func enrichTitle(ctx context.Context, title string, providers []Provider, timeout time.Duration) enrichTitleResult {
	titleCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	asset, ok, warnings := firstPosterAsset(titleCtx, title, providers)
	return enrichTitleResult{Asset: asset, Found: ok, Warnings: warnings}
}

func firstPosterAsset(ctx context.Context, title string, providers []Provider) (PosterAsset, bool, []string) {
	var warnings []string
	for _, provider := range providers {
		candidates, err := provider.Search(ctx, title)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %s: %v", title, provider.Name(), err))
			continue
		}
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate.ImageURL) == "" {
				continue
			}
			return posterAssetFromCandidate(candidate), true, warnings
		}
	}
	return PosterAsset{}, false, warnings
}

func posterAssetFromCandidate(candidate Candidate) PosterAsset {
	confidence := strings.TrimSpace(candidate.Confidence)
	return PosterAsset{
		Src:            strings.TrimSpace(candidate.ImageURL),
		Source:         strings.TrimSpace(candidate.Source),
		MatchedTitle:   strings.TrimSpace(candidate.Title),
		Confidence:     confidence,
		Note:           strings.TrimSpace(candidate.Reason),
		ReviewRequired: confidence != "high",
	}
}
