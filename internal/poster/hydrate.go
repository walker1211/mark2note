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
	for i, page := range out.Pages {
		selection := selectPagePosters(page, manifest)
		if len(selection) < 2 {
			for _, title := range pageCandidateTitles(page) {
				if _, ok := manifest.Find(title); !ok {
					missing[title] = struct{}{}
				}
			}
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
			report.PagesChanged++
			report.ImagesAdded += len(images)
			out.Pages[i] = updated
			continue
		}
		if page.Variant == "gallery-steps" && len(page.Content.Images) == 0 {
			updated.Content.Images = images
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

func selectPagePosters(page deck.Page, manifest Manifest) []selectedPoster {
	candidates := pageCandidates(page)
	selected := make([]selectedPoster, 0, 2)
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
		selected = append(selected, selectedPoster{Title: cleanTitle(candidate.Title), Text: candidate.Text, Asset: asset})
		if len(selected) == 2 {
			break
		}
	}
	return selected
}

type pageCandidate struct {
	Title string
	Text  string
}

func pageCandidates(page deck.Page) []pageCandidate {
	var candidates []pageCandidate
	content := page.Content
	if content.Compare != nil {
		leftText, rightText := "", ""
		if len(content.Compare.Rows) > 0 {
			leftText = content.Compare.Rows[0].Left
			rightText = content.Compare.Rows[0].Right
		}
		candidates = append(candidates, pageCandidate{Title: content.Compare.LeftLabel, Text: firstNonEmpty(leftText, content.Compare.LeftLabel)})
		candidates = append(candidates, pageCandidate{Title: content.Compare.RightLabel, Text: firstNonEmpty(rightText, content.Compare.RightLabel)})
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
		lead := strings.TrimSpace(text)
		if idx := strings.IndexAny(lead, "：:"); idx > 0 {
			return []string{lead[:idx]}
		}
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
