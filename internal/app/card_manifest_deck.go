package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/walker1211/mark2note/internal/deck"
)

const cardArticleManifestSchemaV1 = "card-article-manifest/v1"

const (
	cardManifestCoverSummaryMaxItems     = 6
	cardManifestCoverSummaryItemMaxRunes = 46
	cardManifestImageCaptionBodyMaxRunes = 360
	cardManifestTextCaptionBodyMaxRunes  = 460
)

type cardArticleManifest struct {
	SchemaVersion string               `json:"schema_version"`
	SourceApp     string               `json:"source_app"`
	Document      cardManifestDocument `json:"document"`
	Items         []cardManifestItem   `json:"items"`
}

type cardManifestDocument struct {
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	Period   string   `json:"period"`
	Summary  []string `json:"summary"`
	Subtitle string   `json:"subtitle"`
	Source   string   `json:"source"`
	Badge    string   `json:"badge"`
}

type cardManifestItem struct {
	ID          string                `json:"id"`
	Category    string                `json:"category"`
	Title       string                `json:"title"`
	Summary     string                `json:"summary"`
	Impact      string                `json:"impact"`
	Sections    []cardManifestSection `json:"sections"`
	Source      string                `json:"source"`
	PublishedAt string                `json:"published_at"`
	URL         string                `json:"url"`
	Image       cardManifestImage     `json:"image"`
}

type cardManifestSection struct {
	Label string `json:"label"`
	Body  string `json:"body"`
}

type cardManifestImage struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
}

type cardManifestBodySection struct {
	Label string
	Body  string
}

func (s Service) buildCardManifestDeckJSON(path string) (string, error) {
	data, err := s.effectiveReadFile()(path)
	if err != nil {
		return "", fmt.Errorf("read card manifest: %w", err)
	}
	return buildCardManifestDeckJSON(data)
}

func buildCardManifestDeckJSON(data []byte) (string, error) {
	var manifest cardArticleManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return "", fmt.Errorf("parse card manifest json: %w", err)
	}
	if strings.TrimSpace(manifest.SchemaVersion) != cardArticleManifestSchemaV1 {
		return "", fmt.Errorf("unsupported card manifest schema_version %q", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.Document.Title) == "" {
		return "", fmt.Errorf("card manifest document.title is required")
	}
	if len(manifest.Items) == 0 {
		return "", fmt.Errorf("card manifest items is required")
	}

	pageCount := len(manifest.Items) + 1
	pages := make([]deck.Page, 0, pageCount)
	pages = append(pages, deck.Page{
		Name:    "p01-cover",
		Variant: "cover",
		Meta: deck.PageMeta{
			Badge:   cardManifestCoverBadge(manifest.Document),
			Counter: fmt.Sprintf("1/%d", pageCount),
			Theme:   deck.ThemeDefault,
			CTA:     cardManifestCoverCTA(manifest),
		},
		Content: deck.PageContent{
			Title:    cardManifestCoverTitle(manifest.Document),
			Subtitle: cardManifestCoverSubtitle(manifest.Document),
		},
	})

	for i, item := range manifest.Items {
		page, err := cardManifestItemPage(item, i+2, pageCount)
		if err != nil {
			return "", err
		}
		pages = append(pages, page)
	}

	rawDeck, err := json.Marshal(deck.Deck{Pages: pages})
	if err != nil {
		return "", err
	}
	return string(rawDeck), nil
}

func cardManifestItemPage(item cardManifestItem, pageNumber int, pageCount int) (deck.Page, error) {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		return deck.Page{}, fmt.Errorf("card manifest items[%d].title is required", pageNumber-2)
	}
	variant := "text-caption"
	imageSrc := strings.TrimSpace(item.Image.Src)
	if imageSrc != "" {
		variant = "image-caption"
	}
	body := cardManifestItemBody(item, cardManifestBodyMaxRunes(variant))
	if body == "" {
		body = title
	}
	content := deck.PageContent{Title: title, Body: body}
	if imageSrc != "" {
		imageAlt := strings.TrimSpace(item.Image.Alt)
		if imageAlt == "" {
			imageAlt = title
		}
		content.Images = []deck.ImageBlock{{Src: imageSrc, Alt: imageAlt}}
	}
	return deck.Page{
		Name:    fmt.Sprintf("p%02d-%s", pageNumber, variant),
		Variant: variant,
		Meta: deck.PageMeta{
			Badge:   cardManifestItemBadge(item, pageNumber),
			Counter: fmt.Sprintf("%d/%d", pageNumber, pageCount),
			Theme:   deck.ThemeDefault,
			CTA:     cardManifestSourceMarker(item),
		},
		Content: content,
	}, nil
}

func cardManifestCoverTitle(document cardManifestDocument) string {
	title := strings.TrimSpace(document.Title)
	before, after, ok := strings.Cut(title, "｜")
	if ok && cardManifestShouldBreakCoverTitle(document, before) && strings.TrimSpace(after) != "" {
		return strings.TrimSpace(before) + "\n" + strings.TrimSpace(after)
	}
	return title
}

func cardManifestShouldBreakCoverTitle(document cardManifestDocument, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if strings.Contains(prefix, "电子榨菜") || prefix == "每日热门" || prefix == "每周必看" {
		return true
	}
	return cardManifestIsElectronicPicklesDocument(document)
}

func cardManifestCoverSubtitle(document cardManifestDocument) string {
	bullets := make([]string, 0, minInt(len(document.Summary), cardManifestCoverSummaryMaxItems))
	for _, item := range document.Summary {
		item = cardManifestCoverSummaryItem(document, item)
		if item == "" {
			continue
		}
		bullets = append(bullets, "• "+item)
		if len(bullets) == cardManifestCoverSummaryMaxItems {
			break
		}
	}
	if len(bullets) > 0 {
		return strings.Join(bullets, "\n")
	}
	return cardManifestFitText(document.Subtitle, cardManifestCoverSummaryItemMaxRunes*cardManifestCoverSummaryMaxItems)
}

func cardManifestCoverSummaryItem(document cardManifestDocument, item string) string {
	if cardManifestIsElectronicPicklesDocument(document) {
		return cardManifestNormalizeText(item)
	}
	return cardManifestFitText(item, cardManifestCoverSummaryItemMaxRunes)
}

func cardManifestIsElectronicPicklesDocument(document cardManifestDocument) bool {
	return strings.Contains(strings.TrimSpace(document.Title), "电子榨菜") || strings.Contains(strings.TrimSpace(document.Badge), "电子榨菜")
}

func cardManifestBodyMaxRunes(variant string) int {
	if variant == "image-caption" {
		return cardManifestImageCaptionBodyMaxRunes
	}
	return cardManifestTextCaptionBodyMaxRunes
}

func cardManifestItemBody(item cardManifestItem, maxRunes int) string {
	if body := cardManifestSectionsBody(item.Sections, maxRunes); body != "" {
		return body
	}
	if source := cardManifestItemSourceText(item); source != "" {
		return cardManifestFitSections(cardManifestNewsBodySections(item, source), maxRunes)
	}
	summary, impact := cardManifestFitSummaryImpact(item.Summary, item.Impact, maxRunes)
	parts := make([]string, 0, 2)
	if summary != "" {
		parts = append(parts, cardManifestLabelPrefix("摘要")+summary)
	}
	if impact != "" {
		parts = append(parts, cardManifestLabelPrefix("影响")+impact)
	}
	return strings.Join(parts, "\n\n")
}

func cardManifestNewsBodySections(item cardManifestItem, source string) []cardManifestBodySection {
	sections := make([]cardManifestBodySection, 0, 3)
	sections = appendCardManifestBodySection(sections, "摘要", item.Summary)
	sections = appendCardManifestBodySection(sections, "影响", item.Impact)
	sections = appendCardManifestBodySection(sections, "来源", source)
	return sections
}

func appendCardManifestBodySection(sections []cardManifestBodySection, label string, body string) []cardManifestBodySection {
	body = cardManifestNormalizeText(body)
	if body == "" {
		return sections
	}
	return append(sections, cardManifestBodySection{Label: label, Body: body})
}

func cardManifestSectionsBody(sections []cardManifestSection, maxRunes int) string {
	normalized := cardManifestNormalizeSections(sections)
	if len(normalized) == 0 {
		return ""
	}
	return cardManifestFitSections(normalized, maxRunes)
}

func cardManifestNormalizeSections(sections []cardManifestSection) []cardManifestBodySection {
	normalized := make([]cardManifestBodySection, 0, len(sections))
	for _, section := range sections {
		body := cardManifestNormalizeText(section.Body)
		if body == "" {
			continue
		}
		normalized = append(normalized, cardManifestBodySection{
			Label: cardManifestNormalizeText(section.Label),
			Body:  body,
		})
	}
	return normalized
}

func cardManifestFitSections(sections []cardManifestBodySection, maxRunes int) string {
	if len(sections) == 0 || maxRunes <= 0 {
		return ""
	}
	text := cardManifestSectionsText(sections)
	if runeCount(text) <= maxRunes {
		return text
	}
	fixedRunes := cardManifestSectionsFixedRuneCount(sections)
	contentBudget := maxRunes - fixedRunes
	if contentBudget < len(sections) {
		return cardManifestFitText(text, maxRunes)
	}
	budgets := cardManifestSectionBudgets(sections, contentBudget)
	fitted := make([]cardManifestBodySection, 0, len(sections))
	for i, section := range sections {
		body := cardManifestFitText(section.Body, budgets[i])
		if body == "" {
			continue
		}
		fitted = append(fitted, cardManifestBodySection{Label: section.Label, Body: body})
	}
	text = cardManifestSectionsText(fitted)
	for runeCount(text) > maxRunes {
		index := cardManifestLongestSectionBodyIndex(fitted)
		if index < 0 {
			return cardManifestFitText(text, maxRunes)
		}
		nextBudget := runeCount(fitted[index].Body) - 1
		if nextBudget <= 0 {
			fitted = append(fitted[:index], fitted[index+1:]...)
			if len(fitted) == 0 {
				return ""
			}
		} else {
			fitted[index].Body = cardManifestFitText(fitted[index].Body, nextBudget)
		}
		text = cardManifestSectionsText(fitted)
	}
	return text
}

func cardManifestSectionsText(sections []cardManifestBodySection) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		parts = append(parts, cardManifestSectionText(section))
	}
	return strings.Join(parts, "\n\n")
}

func cardManifestSectionText(section cardManifestBodySection) string {
	if section.Label == "" {
		return section.Body
	}
	return cardManifestLabelPrefix(section.Label) + section.Body
}

func cardManifestLabelPrefix(label string) string {
	return "**" + label + "：** "
}

func cardManifestSectionsFixedRuneCount(sections []cardManifestBodySection) int {
	count := 0
	for i, section := range sections {
		if i > 0 {
			count += runeCount("\n\n")
		}
		if section.Label != "" {
			count += runeCount(cardManifestLabelPrefix(section.Label))
		}
	}
	return count
}

func cardManifestSectionBudgets(sections []cardManifestBodySection, contentBudget int) []int {
	budgets := make([]int, len(sections))
	remaining := contentBudget
	for remaining > 0 {
		active := make([]int, 0, len(sections))
		for i, section := range sections {
			if budgets[i] < runeCount(section.Body) {
				active = append(active, i)
			}
		}
		if len(active) == 0 {
			break
		}
		share := remaining / len(active)
		if share < 1 {
			share = 1
		}
		for _, index := range active {
			need := runeCount(sections[index].Body) - budgets[index]
			add := minInt(share, need)
			add = minInt(add, remaining)
			budgets[index] += add
			remaining -= add
			if remaining == 0 {
				break
			}
		}
	}
	return budgets
}

func cardManifestLongestSectionBodyIndex(sections []cardManifestBodySection) int {
	index := -1
	maxRunes := 0
	for i, section := range sections {
		bodyRunes := runeCount(section.Body)
		if bodyRunes > maxRunes {
			index = i
			maxRunes = bodyRunes
		}
	}
	return index
}

func cardManifestFitSummaryImpact(summary string, impact string, maxRunes int) (string, string) {
	summary = cardManifestNormalizeText(summary)
	impact = cardManifestNormalizeText(impact)
	if cardManifestBodyRuneCount(summary, impact) <= maxRunes {
		return summary, impact
	}
	if maxRunes <= 0 {
		return "", ""
	}
	if summary == "" {
		impactBudget := maxRunes - runeCount(cardManifestLabelPrefix("影响"))
		return "", cardManifestFitText(impact, impactBudget)
	}
	if impact == "" {
		summaryBudget := maxRunes - runeCount(cardManifestLabelPrefix("摘要"))
		return cardManifestFitText(summary, summaryBudget), ""
	}

	contentBudget := maxRunes - runeCount(cardManifestLabelPrefix("摘要")+"\n\n"+cardManifestLabelPrefix("影响"))
	if contentBudget < 2 {
		return "", ""
	}
	summaryBudget := contentBudget * 2 / 3
	impactBudget := contentBudget - summaryBudget
	summaryRunes := runeCount(summary)
	impactRunes := runeCount(impact)
	if summaryRunes < summaryBudget {
		impactBudget += summaryBudget - summaryRunes
		summaryBudget = summaryRunes
	}
	if impactRunes < impactBudget {
		summaryBudget += impactBudget - impactRunes
		impactBudget = impactRunes
	}
	summary = cardManifestFitText(summary, summaryBudget)
	impact = cardManifestFitText(impact, impactBudget)
	for cardManifestBodyRuneCount(summary, impact) > maxRunes {
		if runeCount(impact) >= runeCount(summary) {
			impact = cardManifestFitText(impact, runeCount(impact)-1)
		} else {
			summary = cardManifestFitText(summary, runeCount(summary)-1)
		}
	}
	return summary, impact
}

func cardManifestBodyRuneCount(summary string, impact string) int {
	return runeCount(cardManifestBodyText(summary, impact))
}

func cardManifestBodyText(summary string, impact string) string {
	parts := make([]string, 0, 2)
	if summary != "" {
		parts = append(parts, cardManifestLabelPrefix("摘要")+summary)
	}
	if impact != "" {
		parts = append(parts, cardManifestLabelPrefix("影响")+impact)
	}
	return strings.Join(parts, "\n\n")
}

func cardManifestFitText(value string, maxRunes int) string {
	value = cardManifestNormalizeText(value)
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 0 {
		return ""
	}
	if maxRunes == 1 {
		return "…"
	}
	limit := maxRunes - 1
	boundary := cardManifestSentenceBoundary(runes, limit)
	if boundary == 0 {
		boundary = limit
	}
	return strings.TrimSpace(string(runes[:boundary])) + "…"
}

func cardManifestSentenceBoundary(runes []rune, limit int) int {
	minBoundary := limit * 2 / 3
	if minBoundary < 12 {
		minBoundary = minInt(limit, 12)
	}
	boundary := 0
	for i, r := range runes {
		position := i + 1
		if position > limit {
			break
		}
		if position >= minBoundary && isCardManifestSentenceBoundary(r) {
			boundary = position
		}
	}
	return boundary
}

func isCardManifestSentenceBoundary(r rune) bool {
	switch r {
	case '。', '！', '？', '；', '.', '!', '?', ';':
		return true
	default:
		return false
	}
}

func cardManifestNormalizeText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func runeCount(value string) int {
	return len([]rune(value))
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func cardManifestSourceMarker(item cardManifestItem) string {
	if source := cardManifestItemSourceText(item); source != "" {
		return "来源：" + source
	}
	return "来源：未标注"
}

func cardManifestItemSourceText(item cardManifestItem) string {
	parts := make([]string, 0, 2)
	if source := strings.TrimSpace(item.Source); source != "" {
		parts = append(parts, source)
	}
	if published := formatCardManifestPublishedAt(item.PublishedAt); published != "" {
		parts = append(parts, published)
	}
	return strings.Join(parts, " / ")
}

func formatCardManifestPublishedAt(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return value
}

func cardManifestItemBadge(item cardManifestItem, pageNumber int) string {
	if category := strings.TrimSpace(item.Category); category != "" {
		return fmt.Sprintf("%02d｜%s", pageNumber-1, category)
	}
	return fmt.Sprintf("第 %d 页", pageNumber)
}

func cardManifestCoverBadge(document cardManifestDocument) string {
	if badge := strings.TrimSpace(document.Badge); badge != "" {
		return badge
	}
	date := strings.TrimSpace(document.Date)
	period := strings.TrimSpace(document.Period)
	switch {
	case date != "" && period != "":
		return date + " " + period
	case date != "":
		return date
	case period != "":
		return period
	default:
		return "今日速览"
	}
}

func cardManifestCoverCTA(manifest cardArticleManifest) string {
	if source := strings.TrimSpace(manifest.Document.Source); source != "" {
		return "来源：" + source
	}
	if sourceApp := strings.TrimSpace(manifest.SourceApp); sourceApp != "" {
		return "来源：" + sourceApp
	}
	return "来源：manifest"
}
