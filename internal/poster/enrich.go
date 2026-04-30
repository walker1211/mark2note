package poster

import (
	"context"
	"fmt"
	"strings"
)

type EnrichOptions struct {
	Providers []Provider
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
	for _, title := range report.Titles {
		asset, ok := firstPosterAsset(ctx, title, providers, &report)
		if !ok {
			report.Missing = append(report.Missing, title)
			continue
		}
		manifest.Posters[title] = asset
	}
	return manifest, report, nil
}

func firstPosterAsset(ctx context.Context, title string, providers []Provider, report *EnrichReport) (PosterAsset, bool) {
	for _, provider := range providers {
		candidates, err := provider.Search(ctx, title)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %s: %v", title, provider.Name(), err))
			continue
		}
		for _, candidate := range candidates {
			if strings.TrimSpace(candidate.ImageURL) == "" {
				continue
			}
			return posterAssetFromCandidate(candidate), true
		}
	}
	return PosterAsset{}, false
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
