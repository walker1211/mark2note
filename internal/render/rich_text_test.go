package render

import (
	"strings"
	"testing"
)

func TestParseRichTextParsesBoldAndInlineCode(t *testing.T) {
	got := parseRichText("先看 **重点** 再跑 `mark2note`", richTextOptions{AllowCodeBlocks: false})
	if len(got.Blocks) != 1 {
		t.Fatalf("len(Blocks) = %d, want 1", len(got.Blocks))
	}
	spans := got.Blocks[0].Spans
	if len(spans) != 4 {
		t.Fatalf("len(Spans) = %d, want 4", len(spans))
	}
	if spans[1].Kind != richSpanEmphasis || spans[1].Text != "重点" {
		t.Fatalf("spans[1] = %#v", spans[1])
	}
	if spans[3].Kind != richSpanInlineCode || spans[3].Text != "mark2note" {
		t.Fatalf("spans[3] = %#v", spans[3])
	}
}

func TestParseRichTextKeepsFencedCodeBlocksWhenAllowed(t *testing.T) {
	got := parseRichText("命令如下：\n```bash\nmark2note --input article.md\n```", richTextOptions{AllowCodeBlocks: true})
	if len(got.Blocks) != 2 {
		t.Fatalf("len(Blocks) = %d, want 2", len(got.Blocks))
	}
	if got.Blocks[1].Kind != richBlockCode {
		t.Fatalf("Blocks[1].Kind = %q, want %q", got.Blocks[1].Kind, richBlockCode)
	}
	if got.Blocks[1].Text != "mark2note --input article.md" {
		t.Fatalf("Blocks[1].Text = %q", got.Blocks[1].Text)
	}
}

func TestParseRichTextPreservesTextBeforeAndAfterCodeBlock(t *testing.T) {
	got := parseRichText("前置说明\n```bash\nmark2note --input article.md\n```\n结尾总结", richTextOptions{AllowCodeBlocks: true})
	if len(got.Blocks) != 3 {
		t.Fatalf("len(Blocks) = %d, want 3", len(got.Blocks))
	}
	if got.Blocks[0].Kind != richBlockParagraph || got.Blocks[0].Spans[0].Text != "前置说明" {
		t.Fatalf("Blocks[0] = %#v", got.Blocks[0])
	}
	if got.Blocks[1].Kind != richBlockCode || got.Blocks[1].Text != "mark2note --input article.md" {
		t.Fatalf("Blocks[1] = %#v", got.Blocks[1])
	}
	if got.Blocks[2].Kind != richBlockParagraph || got.Blocks[2].Spans[0].Text != "结尾总结" {
		t.Fatalf("Blocks[2] = %#v", got.Blocks[2])
	}
}

func TestParseRichTextDowngradesFencedCodeBlocksWhenDisallowed(t *testing.T) {
	raw := "```bash\nmark2note --input article.md\n```"
	got := parseRichText(raw, richTextOptions{AllowCodeBlocks: false})
	if len(got.Blocks) != 1 {
		t.Fatalf("len(Blocks) = %d, want 1", len(got.Blocks))
	}
	if got.Blocks[0].Kind != richBlockParagraph {
		t.Fatalf("Blocks[0].Kind = %q, want %q", got.Blocks[0].Kind, richBlockParagraph)
	}
	if len(got.Blocks[0].Spans) != 1 || got.Blocks[0].Spans[0].Text != raw {
		t.Fatalf("Blocks[0].Spans = %#v", got.Blocks[0].Spans)
	}
}

func TestParseRichTextTreatsMalformedSyntaxAsPlainText(t *testing.T) {
	got := parseRichText("未闭合 **重点 和 `code", richTextOptions{AllowCodeBlocks: false})
	if len(got.Blocks) != 1 || len(got.Blocks[0].Spans) != 1 {
		t.Fatalf("got = %#v", got)
	}
	if got.Blocks[0].Spans[0].Kind != richSpanText {
		t.Fatalf("Blocks[0].Spans[0].Kind = %q, want %q", got.Blocks[0].Spans[0].Kind, richSpanText)
	}
}

func TestRenderRichTextHTMLPreservesParagraphNewlinesAsBreaks(t *testing.T) {
	rt := parseRichText("第一行\n第二行", richTextOptions{AllowCodeBlocks: false})
	got := string(renderRichTextHTML(rt))
	if !strings.Contains(got, "第一行<br />第二行") {
		t.Fatalf("renderRichTextHTML() = %q, want preserved line break", got)
	}
}
