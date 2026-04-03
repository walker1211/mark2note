package render

import (
	"html/template"
	"strings"
)

type richText struct {
	Blocks []richBlock
}

type richBlock struct {
	Kind  richBlockKind
	Text  string
	Spans []richSpan
}

type richSpan struct {
	Kind richSpanKind
	Text string
}

type richBlockKind string

type richSpanKind string

const (
	richBlockParagraph richBlockKind = "paragraph"
	richBlockCode      richBlockKind = "code"

	richSpanText       richSpanKind = "text"
	richSpanEmphasis   richSpanKind = "emphasis"
	richSpanInlineCode richSpanKind = "inline-code"
)

type richTextOptions struct {
	AllowCodeBlocks bool
}

func parseRichText(input string, opts richTextOptions) richText {
	if input == "" {
		return richText{Blocks: []richBlock{plainParagraphBlock("")}}
	}
	if containsFencedCodeBlock(input) && !opts.AllowCodeBlocks {
		return plainRichText(input)
	}

	if opts.AllowCodeBlocks {
		blocks, ok := parseBlocksWithCode(input)
		if !ok {
			return plainRichText(input)
		}
		return richText{Blocks: blocks}
	}

	spans, ok := parseInlineSpans(input)
	if !ok {
		return plainRichText(input)
	}
	return richText{Blocks: []richBlock{{Kind: richBlockParagraph, Spans: spans}}}
}

func renderRichTextHTML(rt richText) template.HTML {
	var b strings.Builder
	for _, block := range rt.Blocks {
		switch block.Kind {
		case richBlockCode:
			b.WriteString(`<pre class="code-block">`)
			b.WriteString(template.HTMLEscapeString(block.Text))
			b.WriteString(`</pre>`)
		default:
			b.WriteString(`<span class="rich-text">`)
			for _, span := range block.Spans {
				escaped := strings.ReplaceAll(template.HTMLEscapeString(span.Text), "\n", "<br />")
				switch span.Kind {
				case richSpanEmphasis:
					b.WriteString(`<span class="text-em">`)
					b.WriteString(escaped)
					b.WriteString(`</span>`)
				case richSpanInlineCode:
					b.WriteString(`<span class="inline-code">`)
					b.WriteString(escaped)
					b.WriteString(`</span>`)
				default:
					b.WriteString(escaped)
				}
			}
			b.WriteString(`</span>`)
		}
	}
	return template.HTML(b.String())
}

func parseBlocksWithCode(input string) ([]richBlock, bool) {
	lines := strings.Split(input, "\n")
	var blocks []richBlock
	var paragraph []string

	flushParagraph := func() bool {
		if len(paragraph) == 0 {
			return true
		}
		text := strings.Join(paragraph, "\n")
		paragraph = nil
		spans, ok := parseInlineSpans(text)
		if !ok {
			return false
		}
		blocks = append(blocks, richBlock{Kind: richBlockParagraph, Spans: spans})
		return true
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if !flushParagraph() {
				return nil, false
			}

			j := i + 1
			for ; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "```" {
					break
				}
			}
			if j >= len(lines) {
				return nil, false
			}
			blocks = append(blocks, richBlock{
				Kind: richBlockCode,
				Text: strings.Join(lines[i+1:j], "\n"),
			})
			i = j
			continue
		}
		paragraph = append(paragraph, line)
	}

	if !flushParagraph() {
		return nil, false
	}
	if len(blocks) == 0 {
		return []richBlock{plainParagraphBlock(input)}, true
	}
	return blocks, true
}

func parseInlineSpans(input string) ([]richSpan, bool) {
	var spans []richSpan
	for len(input) > 0 {
		nextEm := strings.Index(input, "**")
		nextCode := strings.Index(input, "`")
		next := nextMarkerIndex(nextEm, nextCode)
		if next < 0 {
			spans = appendTextSpan(spans, input)
			return spans, true
		}
		if next > 0 {
			spans = appendTextSpan(spans, input[:next])
			input = input[next:]
		}

		if strings.HasPrefix(input, "**") {
			end := strings.Index(input[2:], "**")
			if end < 0 {
				return nil, false
			}
			content := input[2 : 2+end]
			if content == "" || strings.Contains(content, "**") || strings.Contains(content, "`") {
				return nil, false
			}
			spans = append(spans, richSpan{Kind: richSpanEmphasis, Text: content})
			input = input[2+end+2:]
			continue
		}

		if strings.HasPrefix(input, "`") {
			end := strings.Index(input[1:], "`")
			if end < 0 {
				return nil, false
			}
			content := input[1 : 1+end]
			if content == "" || strings.Contains(content, "**") || strings.Contains(content, "`") {
				return nil, false
			}
			spans = append(spans, richSpan{Kind: richSpanInlineCode, Text: content})
			input = input[1+end+1:]
			continue
		}
	}
	return spans, true
}

func nextMarkerIndex(indices ...int) int {
	next := -1
	for _, idx := range indices {
		if idx < 0 {
			continue
		}
		if next < 0 || idx < next {
			next = idx
		}
	}
	return next
}

func appendTextSpan(spans []richSpan, text string) []richSpan {
	if len(spans) > 0 && spans[len(spans)-1].Kind == richSpanText {
		spans[len(spans)-1].Text += text
		return spans
	}
	return append(spans, richSpan{Kind: richSpanText, Text: text})
}

func containsFencedCodeBlock(input string) bool {
	lines := strings.Split(input, "\n")
	for i := range lines {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "```" {
				return true
			}
		}
	}
	return false
}

func plainRichText(input string) richText {
	return richText{Blocks: []richBlock{plainParagraphBlock(input)}}
}

func plainParagraphBlock(input string) richBlock {
	return richBlock{
		Kind: richBlockParagraph,
		Spans: []richSpan{{
			Kind: richSpanText,
			Text: input,
		}},
	}
}
