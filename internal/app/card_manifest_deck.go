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
}

type cardManifestItem struct {
	ID          string            `json:"id"`
	Category    string            `json:"category"`
	Title       string            `json:"title"`
	Summary     string            `json:"summary"`
	Impact      string            `json:"impact"`
	Source      string            `json:"source"`
	PublishedAt string            `json:"published_at"`
	URL         string            `json:"url"`
	Image       cardManifestImage `json:"image"`
}

type cardManifestImage struct {
	Src string `json:"src"`
	Alt string `json:"alt"`
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
			CTA:     cardManifestCoverCTA(manifest.SourceApp),
		},
		Content: deck.PageContent{
			Title:    strings.TrimSpace(manifest.Document.Title),
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

func cardManifestCoverSubtitle(document cardManifestDocument) string {
	bullets := make([]string, 0, minInt(len(document.Summary), cardManifestCoverSummaryMaxItems))
	for _, item := range document.Summary {
		item = cardManifestFitText(item, cardManifestCoverSummaryItemMaxRunes)
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

func cardManifestBodyMaxRunes(variant string) int {
	if variant == "image-caption" {
		return cardManifestImageCaptionBodyMaxRunes
	}
	return cardManifestTextCaptionBodyMaxRunes
}

func cardManifestItemBody(item cardManifestItem, maxRunes int) string {
	summary, impact := cardManifestFitSummaryImpact(item.Summary, item.Impact, maxRunes)
	parts := make([]string, 0, 2)
	if summary != "" {
		parts = append(parts, "摘要："+summary)
	}
	if impact != "" {
		parts = append(parts, "影响："+impact)
	}
	return strings.Join(parts, "\n\n")
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
		impactBudget := maxRunes - runeCount("影响：")
		return "", cardManifestFitText(impact, impactBudget)
	}
	if impact == "" {
		summaryBudget := maxRunes - runeCount("摘要：")
		return cardManifestFitText(summary, summaryBudget), ""
	}

	contentBudget := maxRunes - runeCount("摘要：\n\n影响：")
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
		parts = append(parts, "摘要："+summary)
	}
	if impact != "" {
		parts = append(parts, "影响："+impact)
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
	parts := make([]string, 0, 2)
	if source := strings.TrimSpace(item.Source); source != "" {
		parts = append(parts, source)
	}
	if published := formatCardManifestPublishedAt(item.PublishedAt); published != "" {
		parts = append(parts, published)
	}
	if len(parts) == 0 {
		return "来源：未标注"
	}
	return "来源：" + strings.Join(parts, " / ")
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

func cardManifestCoverCTA(sourceApp string) string {
	if sourceApp = strings.TrimSpace(sourceApp); sourceApp != "" {
		return "来源：" + sourceApp
	}
	return "来源：manifest"
}
