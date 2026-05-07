package poster

import (
	"fmt"
	"strings"

	"github.com/walker1211/mark2note/internal/deck"
)

type HydrateOptions struct {
	BaseDir               string
	DisablePageConversion bool
}

type HydrateReport struct {
	PagesChanged int
	ImagesAdded  int
	Missing      []string
}

func HydrateDeck(d deck.Deck, manifest Manifest, opts HydrateOptions) (deck.Deck, HydrateReport, error) {
	out := d
	out.Pages = append([]deck.Page(nil), d.Pages...)
	report := HydrateReport{}
	missing := map[string]struct{}{}
	used := map[string]int{}
	for i, page := range out.Pages {
		for _, title := range pageCandidateTitles(page) {
			if _, ok := manifest.Find(title); !ok {
				missing[title] = struct{}{}
			}
		}
		selection := selectPagePosters(page, manifest, used)
		if len(selection) < 2 {
			continue
		}
		images := make([]deck.ImageBlock, 0, 2)
		for _, selected := range selection[:2] {
			src, err := MaterializeSrc(selected.Asset.Src, opts.BaseDir)
			if err != nil {
				return deck.Deck{}, HydrateReport{}, fmt.Errorf("materialize poster %q: %w", selected.Title, err)
			}
			images = append(images, deck.ImageBlock{Src: src, Alt: selected.Title + "封面", Caption: "《" + selected.Title + "》"})
		}
		updated := page
		if page.Variant != "gallery-steps" && !opts.DisablePageConversion {
			updated.Variant = "gallery-steps"
			updated.Content = deck.PageContent{Title: page.Content.Title, Steps: preservedSteps(page, selection[:2]), Images: images}
			markSelectedPostersUsed(used, selection[:2])
			report.PagesChanged++
			report.ImagesAdded += len(images)
			out.Pages[i] = updated
			continue
		}
		if page.Variant == "gallery-steps" && len(page.Content.Images) == 0 {
			updated.Content.Images = images
			markSelectedPostersUsed(used, selection[:2])
			report.PagesChanged++
			report.ImagesAdded += len(images)
			out.Pages[i] = updated
		}
	}
	for title := range missing {
		report.Missing = append(report.Missing, title)
	}
	return out, report, nil
}

type selectedPoster struct {
	Title string
	Text  string
	Asset PosterAsset
}

func selectPagePosters(page deck.Page, manifest Manifest, used map[string]int) []selectedPoster {
	candidates := pageCandidates(page)
	available := make([]selectedPoster, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		asset, ok := manifest.Find(candidate.Title)
		if !ok {
			continue
		}
		key := normalizeTitle(candidate.Title)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		available = append(available, selectedPoster{Title: cleanTitle(candidate.Title), Text: candidate.Text, Asset: asset})
	}
	selected := make([]selectedPoster, 0, 2)
	chosen := map[int]struct{}{}
	for len(selected) < 2 && len(chosen) < len(available) {
		best := -1
		bestCount := 0
		for i, candidate := range available {
			if _, ok := chosen[i]; ok {
				continue
			}
			count := used[normalizeTitle(candidate.Title)]
			if best == -1 || count < bestCount {
				best = i
				bestCount = count
			}
		}
		chosen[best] = struct{}{}
		selected = append(selected, available[best])
	}
	return selected
}

func markSelectedPostersUsed(used map[string]int, selection []selectedPoster) {
	for _, selected := range selection {
		key := normalizeTitle(selected.Title)
		if key != "" {
			used[key]++
		}
	}
}

type pageCandidate struct {
	Title string
	Text  string
}

func pageCandidates(page deck.Page) []pageCandidate {
	var candidates []pageCandidate
	content := page.Content
	if content.Compare != nil {
		for _, title := range titlesFromText(content.Compare.LeftLabel) {
			candidates = append(candidates, pageCandidate{Title: title, Text: content.Compare.LeftLabel})
		}
		for _, title := range titlesFromText(content.Compare.RightLabel) {
			candidates = append(candidates, pageCandidate{Title: title, Text: content.Compare.RightLabel})
		}
		for _, row := range content.Compare.Rows {
			for _, title := range titlesFromText(row.Left) {
				candidates = append(candidates, pageCandidate{Title: title, Text: row.Left})
			}
			for _, title := range titlesFromText(row.Right) {
				candidates = append(candidates, pageCandidate{Title: title, Text: row.Right})
			}
		}
	}
	for _, item := range content.Items {
		for _, title := range titlesFromText(item) {
			candidates = append(candidates, pageCandidate{Title: title, Text: item})
		}
	}
	for _, step := range content.Steps {
		for _, title := range titlesFromText(step) {
			candidates = append(candidates, pageCandidate{Title: title, Text: step})
		}
	}
	for _, img := range content.Images {
		if img.Caption != "" {
			candidates = append(candidates, pageCandidate{Title: img.Caption, Text: img.Caption})
		}
	}
	return candidates
}

func pageCandidateTitles(page deck.Page) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, candidate := range pageCandidates(page) {
		addTitle(&out, seen, candidate.Title)
	}
	return out
}

func titlesFromText(text string) []string {
	matches := angleTitleRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		matches = boldTitleRe.FindAllStringSubmatch(text, -1)
	}
	if len(matches) == 0 {
		return nil
	}
	titles := make([]string, 0, len(matches))
	for _, match := range matches {
		titles = append(titles, match[1])
	}
	return titles
}

func preservedSteps(page deck.Page, selection []selectedPoster) []string {
	content := page.Content
	if len(content.Items) > 0 {
		return append([]string(nil), content.Items...)
	}
	if len(content.Steps) > 0 {
		return append([]string(nil), content.Steps...)
	}
	if content.Compare != nil {
		steps := make([]string, 0, len(content.Compare.Rows)*2)
		for _, row := range content.Compare.Rows {
			if strings.TrimSpace(row.Left) != "" {
				steps = append(steps, row.Left)
			}
			if strings.TrimSpace(row.Right) != "" {
				steps = append(steps, row.Right)
			}
		}
		if len(steps) > 0 {
			return steps
		}
	}
	return selectionSteps(selection)
}

func selectionSteps(selection []selectedPoster) []string {
	steps := make([]string, 0, len(selection))
	for _, selected := range selection {
		text := strings.TrimSpace(selected.Text)
		if text == "" {
			text = "《" + selected.Title + "》"
		}
		steps = append(steps, text)
	}
	return steps
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
