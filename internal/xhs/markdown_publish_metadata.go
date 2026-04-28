package xhs

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minMarkdownPublishTopics = 3
	maxMarkdownPublishTopics = 6
)

var markdownPublishFallbackTopics = []string{"小红书", "图文笔记", "内容创作"}

var (
	levelOneHeadingPattern      = regexp.MustCompile(`(?m)^\s*#\s+(.+?)\s*$`)
	levelTwoThreeHeadingPattern = regexp.MustCompile(`(?m)^\s*#{2,3}\s+(.+?)\s*$`)
	hashtagPattern              = regexp.MustCompile(`(?:^|[\s，。；;：:、])#([\p{Han}A-Za-z0-9_][\p{Han}A-Za-z0-9_-]{0,31})`)
	mixedTermPattern            = regexp.MustCompile(`[A-Za-z0-9]{2,}[\p{Han}]{1,8}`)
	englishTermPattern          = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]{1,}`)
	chinesePhrasePattern        = regexp.MustCompile(`[\p{Han}]{2,8}`)
	datePrefixPattern           = regexp.MustCompile(`^(?:\d{2,4}[.\-_]\d{1,2}[.\-_]\d{1,2}|\d{4}年\d{1,2}月\d{1,2}日)[\-_\s]*`)
	markdownLinkPattern         = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
	atxClosingHeadingPattern    = regexp.MustCompile(`\s+#+\s*$`)
)

var markdownPublishChineseStopWords = []string{
	"为什么", "复盘", "需要", "权限", "边界", "工程", "一个", "这个", "那个", "这些", "那些",
	"之后", "之前", "开始", "关心", "自己", "我们", "他们", "你们", "它们", "不是",
	"没有", "如果", "因为", "所以", "但是", "然后", "以及", "关于", "如何", "什么",
	"怎么", "今天", "目前", "平时", "正文", "标题", "内容", "文章", "图片",
}

var markdownPublishStopWords = map[string]bool{
	"复盘": true, "需要": true, "权限": true, "边界": true, "工程": true,
	"一个": true, "这个": true, "那个": true, "这些": true, "那些": true,
	"之后": true, "之前": true, "开始": true, "关心": true, "自己": true,
	"我们": true, "他们": true, "你们": true, "它们": true, "不是": true,
	"没有": true, "如果": true, "因为": true, "所以": true, "但是": true,
	"然后": true, "以及": true, "关于": true, "如何": true, "什么": true,
	"为什么": true, "怎么": true, "今天": true, "目前": true, "平时": true,
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
		title := normalizeInlineMarkdown(match[1])
		if title != "" {
			return title
		}
	}
	return cleanedFilenameTitle(inputPath)
}

func MarkdownPublishTopics(markdown string, title string, overrideTags []string) []string {
	collector := newTopicCollector()
	if len(overrideTags) > 0 {
		collector.addAll(overrideTags)
		return collector.withFallback(title)
	}

	markdown = stripFencedCodeBlocks(markdown)
	frontmatter := parseFrontmatter(markdown)
	markdown = frontmatter.body
	collector.addAll(frontmatter.tags)
	collector.addAll(markdownHashtags(markdown))
	collector.addTerms(title)
	collector.addAll(headingTerms(markdown))
	collector.addAll(frequentBodyTerms(markdown))
	collector.addUntilMin([]string{title}, minMarkdownPublishTopics)
	collector.addUntilMin(markdownPublishFallbackTopics, minMarkdownPublishTopics)
	return collector.withFallback(title)
}

func MarkdownTopicBody(topics []string) string {
	collector := newTopicCollector()
	collector.addAll(topics)
	formatted := collector.withFallback("")
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

func (c *topicCollector) addTerms(text string) {
	for _, value := range termCandidates(text) {
		c.add(value)
	}
}

func (c *topicCollector) addUntilMin(values []string, min int) {
	for _, value := range values {
		if len(c.values) >= min {
			return
		}
		c.add(value)
	}
}

func (c *topicCollector) add(value string) {
	value = strings.TrimSpace(value)
	if isNoisyMixedTerm(value) {
		return
	}
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

func (c *topicCollector) withFallback(title string) []string {
	if len(c.values) == 0 {
		c.add(title)
	}
	return append([]string(nil), c.values...)
}

type parsedFrontmatter struct {
	tags []string
	body string
}

func parseFrontmatter(markdown string) parsedFrontmatter {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	prefixLen := len(normalized) - len(strings.TrimLeftFunc(normalized, unicode.IsSpace))
	trimmed := normalized[prefixLen:]
	if !strings.HasPrefix(trimmed, "---\n") {
		return parsedFrontmatter{body: markdown}
	}
	lines := strings.Split(trimmed, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return parsedFrontmatter{body: markdown}
	}

	var tags []string
	collectingList := false
	for _, line := range lines[1:end] {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "tags:") {
			collectingList = true
			value := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "tags:"))
			if value != "" {
				tags = append(tags, splitTagValues(value)...)
			}
			continue
		}
		if collectingList && strings.HasPrefix(trimmedLine, "-") {
			tags = append(tags, strings.TrimSpace(strings.TrimPrefix(trimmedLine, "-")))
			continue
		}
		collectingList = false
	}
	body := strings.Join(lines[end+1:], "\n")
	if prefixLen > 0 {
		body = normalized[:prefixLen] + body
	}
	return parsedFrontmatter{tags: tags, body: body}
}

func isNoisyMixedTerm(value string) bool {
	if !mixedTermPattern.MatchString(value) {
		return false
	}
	for i, r := range value {
		if unicode.Is(unicode.Han, r) {
			return utf8.RuneCountInString(value[i:]) > 3
		}
	}
	return false
}

func splitTagValues(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(strings.Trim(field, `"'`))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func markdownHashtags(markdown string) []string {
	matches := hashtagPattern.FindAllStringSubmatch(markdown, -1)
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 2 {
			result = append(result, match[1])
		}
	}
	return result
}

func headingTerms(markdown string) []string {
	matches := levelTwoThreeHeadingPattern.FindAllStringSubmatch(markdown, -1)
	result := []string{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		heading := normalizeInlineMarkdown(match[1])
		if heading != "" {
			result = append(result, heading)
			for _, phrase := range chinesePhrasePattern.FindAllString(stopWordsAsSeparators(heading), -1) {
				result = append(result, phrase)
			}
		}
		result = append(result, termCandidates(heading)...)
	}
	return result
}

func frequentBodyTerms(markdown string) []string {
	counts := map[string]int{}
	for _, candidate := range termCandidates(markdown) {
		normalized := normalizeTopic(candidate)
		if normalized != "" {
			counts[normalized]++
		}
	}
	type counted struct {
		value string
		count int
	}
	items := make([]counted, 0, len(counts))
	for value, count := range counts {
		items = append(items, counted{value: value, count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count != items[j].count {
			return items[i].count > items[j].count
		}
		leftLen := utf8.RuneCountInString(items[i].value)
		rightLen := utf8.RuneCountInString(items[j].value)
		if leftLen != rightLen {
			return leftLen > rightLen
		}
		return items[i].value < items[j].value
	})
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.value)
	}
	return result
}

func termCandidates(text string) []string {
	cleaned := normalizeInlineMarkdown(text)
	result := []string{}
	result = append(result, mixedTermPattern.FindAllString(cleaned, -1)...)
	result = append(result, mixedTermSubphrases(cleaned)...)
	result = append(result, englishTermPattern.FindAllString(cleaned, -1)...)
	for _, phrase := range chinesePhrasePattern.FindAllString(stopWordsAsSeparators(cleaned), -1) {
		result = append(result, phrase)
	}
	return result
}

func mixedTermSubphrases(text string) []string {
	matches := mixedTermPattern.FindAllString(text, -1)
	result := []string{}
	for _, match := range matches {
		asciiEnd := 0
		for i, r := range match {
			if unicode.Is(unicode.Han, r) {
				asciiEnd = i
				break
			}
		}
		if asciiEnd == 0 {
			continue
		}
		prefix := match[:asciiEnd]
		suffix := []rune(match[asciiEnd:])
		for size := 2; size <= 2 && size <= len(suffix); size++ {
			result = append(result, prefix+string(suffix[:size]))
		}
	}
	return result
}

func normalizeInlineMarkdown(input string) string {
	value := markdownLinkPattern.ReplaceAllString(input, "$1")
	value = atxClosingHeadingPattern.ReplaceAllString(value, "")
	replacer := strings.NewReplacer("`", "", "**", "", "__", "", "*", "", "_", "", "《", "", "》", "")
	value = replacer.Replace(value)
	return strings.TrimSpace(value)
}

func stopWordsAsSeparators(input string) string {
	value := input
	for _, word := range markdownPublishChineseStopWords {
		value = strings.ReplaceAll(value, word, " ")
	}
	return value
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
	if value == "" {
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
