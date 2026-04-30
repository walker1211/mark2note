package poster

import (
	"regexp"
	"strings"

	"github.com/walker1211/mark2note/internal/deck"
)

var (
	angleTitleRe = regexp.MustCompile(`《([^》]+)》`)
	boldLeadRe   = regexp.MustCompile(`(?m)^\s*[-*]\s+\*\*([^*\n]+)\*\*\s*[：:]`)
)

func ExtractTitlesFromMarkdown(markdown string) []string {
	var titles []string
	seen := map[string]struct{}{}
	for _, match := range angleTitleRe.FindAllStringSubmatch(markdown, -1) {
		addTitle(&titles, seen, match[1])
	}
	for _, match := range boldLeadRe.FindAllStringSubmatch(markdown, -1) {
		addTitle(&titles, seen, match[1])
	}
	return titles
}

func ExtractTitlesFromDeck(d deck.Deck) []string {
	var titles []string
	seen := map[string]struct{}{}
	for _, page := range d.Pages {
		content := page.Content
		if content.Compare != nil {
			addTitle(&titles, seen, content.Compare.LeftLabel)
			addTitle(&titles, seen, content.Compare.RightLabel)
			for _, row := range content.Compare.Rows {
				addTitlesFromText(&titles, seen, row.Left)
				addTitlesFromText(&titles, seen, row.Right)
			}
		}
		for _, item := range content.Items {
			addTitlesFromText(&titles, seen, item)
		}
		for _, step := range content.Steps {
			addTitlesFromText(&titles, seen, step)
		}
		for _, img := range content.Images {
			addTitle(&titles, seen, img.Caption)
			addTitle(&titles, seen, img.Alt)
		}
	}
	return titles
}

func addTitlesFromText(titles *[]string, seen map[string]struct{}, text string) {
	matches := angleTitleRe.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			addTitle(titles, seen, match[1])
		}
		return
	}
	lead := strings.TrimSpace(text)
	if idx := strings.IndexAny(lead, "：:"); idx > 0 {
		addTitle(titles, seen, lead[:idx])
	}
}

func addTitle(titles *[]string, seen map[string]struct{}, title string) {
	clean := cleanTitle(title)
	key := normalizeTitle(clean)
	if key == "" {
		return
	}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*titles = append(*titles, clean)
}

func cleanTitle(title string) string {
	clean := strings.TrimSpace(title)
	clean = strings.TrimPrefix(clean, "《")
	clean = strings.TrimSuffix(clean, "》")
	clean = strings.Trim(clean, " \t\r\n\"'`*_：:")
	return clean
}
