package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/walker1211/mark2note/internal/deck"
)

var (
	electronicPicklesH1Pattern    = regexp.MustCompile(`(?m)^#\s+(.+电子榨菜[^\n]*)\s*$`)
	electronicPicklesPagePattern  = regexp.MustCompile(`^###\s+第\s*(\d+)\s*页｜(.+)$`)
	electronicPicklesImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
)

type electronicPicklesItem struct {
	Index     int
	Title     string
	FullTitle string
	Stats     string
	Image     deck.ImageBlock
	Overview  string
}

func buildElectronicPicklesDeckJSON(markdown string) (string, bool, error) {
	title := electronicPicklesTitle(markdown)
	if title == "" || !strings.Contains(markdown, "## 小红书卡片") {
		return "", false, nil
	}
	items := parseElectronicPicklesItems(markdown)
	if len(items) == 0 {
		return "", false, nil
	}
	pageCount := len(items) + 1
	pages := make([]deck.Page, 0, pageCount)
	pages = append(pages, deck.Page{
		Name:    "p01-cover",
		Variant: "cover",
		Meta: deck.PageMeta{
			Badge:   electronicPicklesColumnName(title),
			Counter: fmt.Sprintf("1/%d", pageCount),
			Theme:   "default",
			CTA:     "B站热门",
		},
		Content: deck.PageContent{
			Title:    title,
			Subtitle: electronicPicklesCoverSubtitle(items),
			Images:   electronicPicklesCoverImages(items),
		},
	})
	for i, item := range items {
		pages = append(pages, deck.Page{
			Name:    fmt.Sprintf("p%02d-video", i+2),
			Variant: "image-caption",
			Meta: deck.PageMeta{
				Badge:   electronicPicklesBadge(item),
				Counter: fmt.Sprintf("%d/%d", i+2, pageCount),
				Theme:   "default",
				CTA:     "B站热门",
			},
			Content: deck.PageContent{
				Title:  item.Title,
				Body:   electronicPicklesBody(item.Overview),
				Images: electronicPicklesItemImages(item),
			},
		})
	}
	data, err := json.Marshal(deck.Deck{Pages: pages})
	if err != nil {
		return "", true, err
	}
	return string(data), true, nil
}

func electronicPicklesTitle(markdown string) string {
	match := electronicPicklesH1Pattern.FindStringSubmatch(markdown)
	if len(match) < 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func parseElectronicPicklesItems(markdown string) []electronicPicklesItem {
	sectionStart := strings.Index(markdown, "## 小红书卡片")
	if sectionStart < 0 {
		return nil
	}
	lines := strings.Split(markdown[sectionStart:], "\n")
	items := make([]electronicPicklesItem, 0)
	var current *electronicPicklesItem
	var overview strings.Builder
	flush := func() {
		if current == nil {
			return
		}
		current.Overview = strings.TrimSpace(overview.String())
		items = append(items, *current)
		overview.Reset()
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if match := electronicPicklesPagePattern.FindStringSubmatch(trimmed); len(match) == 3 {
			flush()
			index := len(items) + 1
			if parsed := strings.TrimSpace(match[1]); parsed != "" {
				fmt.Sscanf(parsed, "%d", &index)
			}
			fullTitle := strings.TrimSpace(match[2])
			title, stats := splitElectronicPicklesTitleStats(fullTitle)
			current = &electronicPicklesItem{Index: index, Title: title, FullTitle: fullTitle, Stats: stats}
			continue
		}
		if current == nil {
			continue
		}
		if imageMatch := electronicPicklesImagePattern.FindStringSubmatch(trimmed); len(imageMatch) == 3 && strings.TrimSpace(current.Image.Src) == "" {
			current.Image = deck.ImageBlock{Alt: strings.TrimSpace(imageMatch[1]), Src: strings.TrimSpace(imageMatch[2])}
			continue
		}
		if strings.HasPrefix(trimmed, "* **内容概览**：") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "* **内容概览**："))
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "## ") {
			if overview.Len() > 0 {
				overview.WriteByte('\n')
			}
			overview.WriteString(trimmed)
		}
	}
	flush()
	return items
}

func splitElectronicPicklesTitleStats(fullTitle string) (string, string) {
	fullTitle = strings.TrimSpace(fullTitle)
	start := strings.LastIndex(fullTitle, "（点赞 ")
	if start < 0 || !strings.HasSuffix(fullTitle, "）") {
		return fullTitle, ""
	}
	return strings.TrimSpace(fullTitle[:start]), strings.TrimSuffix(strings.TrimPrefix(fullTitle[start:], "（"), "）")
}

func electronicPicklesColumnName(title string) string {
	if before, _, ok := strings.Cut(title, "｜"); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(title)
}

func electronicPicklesCoverSubtitle(items []electronicPicklesItem) string {
	lines := []string{"今日彩蛋"}
	limit := 6
	if len(items) < limit {
		limit = len(items)
	}
	for i := 0; i < limit; i++ {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, items[i].Title))
	}
	return strings.Join(lines, "\n")
}

func electronicPicklesCoverImages(items []electronicPicklesItem) []deck.ImageBlock {
	for _, item := range items {
		if strings.TrimSpace(item.Image.Src) != "" {
			return []deck.ImageBlock{item.Image}
		}
	}
	return nil
}

func electronicPicklesItemImages(item electronicPicklesItem) []deck.ImageBlock {
	if strings.TrimSpace(item.Image.Src) == "" {
		return nil
	}
	return []deck.ImageBlock{item.Image}
}

func electronicPicklesBadge(item electronicPicklesItem) string {
	if stats := strings.TrimSpace(item.Stats); stats != "" {
		return fmt.Sprintf("%02d｜%s", item.Index, stats)
	}
	return fmt.Sprintf("%02d｜视频话题", item.Index)
}

func electronicPicklesBody(overview string) string {
	return strings.TrimSpace(overview)
}
