package xhs

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxMarkdownPublishTopics = 6

var (
	levelOneHeadingPattern   = regexp.MustCompile(`(?m)^\s*#\s+(.+?)\s*$`)
	datePrefixPattern        = regexp.MustCompile(`^(?:\d{2,4}[.\-_]\d{1,2}[.\-_]\d{1,2}|\d{4}年\d{1,2}月\d{1,2}日)[\-_\s]*`)
	markdownLinkPattern      = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
	atxClosingHeadingPattern = regexp.MustCompile(`\s+#+\s*$`)
)

var markdownPublishStopWords = map[string]bool{
	"正文": true, "标题": true, "内容": true, "文章": true, "图片": true,
	"the": true, "and": true, "for": true, "with": true, "this": true,
	"that": true, "from": true, "into": true, "after": true, "before": true,
}

func MarkdownPublishTitle(markdown string, inputPath string) string {
	markdown = stripFencedCodeBlocks(markdown)
	matches := levelOneHeadingPattern.FindAllStringSubmatch(markdown, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		title := NormalizePublishTitle(match[1])
		if title != "" {
			return title
		}
	}
	return NormalizePublishTitle(cleanedFilenameTitle(inputPath))
}

func NormalizePublishTitle(title string) string {
	return normalizeInlineMarkdown(title)
}

func NormalizePublishTopics(values []string) []string {
	collector := newTopicCollector()
	collector.addAll(values)
	return collector.values
}

func MarkdownTopicBody(topics []string) string {
	formatted := NormalizePublishTopics(topics)
	parts := make([]string, 0, len(formatted))
	for _, topic := range formatted {
		parts = append(parts, "#"+topic)
	}
	return strings.Join(parts, " ")
}

type topicCollector struct {
	values []string
	seen   map[string]bool
}

func newTopicCollector() topicCollector {
	return topicCollector{seen: map[string]bool{}}
}

func (c *topicCollector) addAll(values []string) {
	for _, value := range values {
		c.add(value)
	}
}

func (c *topicCollector) add(value string) {
	if len(c.values) >= maxMarkdownPublishTopics {
		return
	}
	normalized := normalizeTopic(value)
	if normalized == "" || c.seen[normalized] {
		return
	}
	c.seen[normalized] = true
	c.values = append(c.values, normalized)
}

func normalizeInlineMarkdown(input string) string {
	value := markdownLinkPattern.ReplaceAllString(input, "$1")
	value = atxClosingHeadingPattern.ReplaceAllString(value, "")
	replacer := strings.NewReplacer("`", "", "**", "", "__", "", "*", "", "_", "", "《", "", "》", "")
	value = replacer.Replace(value)
	return strings.TrimSpace(value)
}

func stripFencedCodeBlocks(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	inFence := false
	var fenceMarker string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence = true
				fenceMarker = marker
				result = append(result, "")
				continue
			}
			if marker == fenceMarker {
				inFence = false
				fenceMarker = ""
				result = append(result, "")
				continue
			}
		}
		if inFence {
			result = append(result, "")
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func normalizeTopic(input string) string {
	value := normalizeInlineMarkdown(input)
	value = strings.TrimSpace(strings.TrimPrefix(value, "#"))
	value = strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\t", "")
	value = strings.TrimFunc(value, func(r rune) bool {
		return unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	if value == "" || !hasTopicText(value) {
		return ""
	}
	lower := strings.ToLower(value)
	if markdownPublishStopWords[lower] || markdownPublishStopWords[value] {
		return ""
	}
	if utf8.RuneCountInString(value) < 2 {
		return ""
	}
	return limitRunes(value, 12)
}

func hasTopicText(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func cleanedFilenameTitle(inputPath string) string {
	base := filepath.Base(strings.TrimSpace(inputPath))
	name := strings.TrimSuffix(base, filepath.Ext(base))
	name = datePrefixPattern.ReplaceAllString(name, "")
	name = strings.TrimSpace(strings.NewReplacer("_", " ", "-", " ").Replace(name))
	if name == "" {
		return "未命名"
	}
	return name
}

func limitRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}
