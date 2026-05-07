package xhs

import (
	"reflect"
	"testing"
)

func TestMarkdownPublishTitleUsesFirstLevelOneHeading(t *testing.T) {
	markdown := "## 不是标题\n\n# 一个AI代理删库之后我开始关心刹车\n\n正文"

	got := MarkdownPublishTitle(markdown, "article.md")

	if got != "一个AI代理删库之后我开始关心刹车" {
		t.Fatalf("MarkdownPublishTitle() = %q", got)
	}
}

func TestMarkdownPublishTitleFallsBackToCleanedFilename(t *testing.T) {
	got := MarkdownPublishTitle("没有一级标题", "/tmp/26.04.26-一个AI代理删库之后我开始关心刹车.md")

	if got != "一个AI代理删库之后我开始关心刹车" {
		t.Fatalf("MarkdownPublishTitle() = %q", got)
	}
}

func TestMarkdownPublishTitleIgnoresFencedCodeBlocks(t *testing.T) {
	markdown := "```markdown\n# 假标题\n```\n\n# 真实标题\n\n正文"

	got := MarkdownPublishTitle(markdown, "article.md")

	if got != "真实标题" {
		t.Fatalf("MarkdownPublishTitle() = %q", got)
	}
}

func TestMarkdownPublishTitleStripsATXClosingMarker(t *testing.T) {
	got := MarkdownPublishTitle("# 真实标题 #\n\n正文", "article.md")

	if got != "真实标题" {
		t.Fatalf("MarkdownPublishTitle() = %q", got)
	}
}

func TestMarkdownPublishTitlePreservesLongHeading(t *testing.T) {
	got := MarkdownPublishTitle("# 一二三四五六七八九十一二三四五六七八九十超长\n\n正文", "article.md")
	want := "一二三四五六七八九十一二三四五六七八九十超长"

	if got != want {
		t.Fatalf("MarkdownPublishTitle() = %q, want %q", got, want)
	}
}

func TestMarkdownPublishTitlePreservesLongFilenameFallback(t *testing.T) {
	got := MarkdownPublishTitle("没有一级标题", "/tmp/26.04.26-一二三四五六七八九十一二三四五六七八九十超长.md")
	want := "一二三四五六七八九十一二三四五六七八九十超长"

	if got != want {
		t.Fatalf("MarkdownPublishTitle() = %q, want %q", got, want)
	}
}

func TestNormalizePublishTitlePreservesShortTitle(t *testing.T) {
	got := NormalizePublishTitle("  **真实标题**  ")

	if got != "真实标题" {
		t.Fatalf("NormalizePublishTitle() = %q", got)
	}
}

func TestNormalizePublishTopicsCleansManualAndAITopics(t *testing.T) {
	got := NormalizePublishTopics([]string{"#AI代理", " 数据安全 ", "AI代理", "1061", "工程反思"})
	want := []string{"AI代理", "数据安全", "工程反思"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizePublishTopics() = %#v, want %#v", got, want)
	}
}

func TestNormalizePublishTopicsLimitsToSix(t *testing.T) {
	got := NormalizePublishTopics([]string{"一号话题", "二号话题", "三号话题", "四号话题", "五号话题", "六号话题", "七号话题"})
	want := []string{"一号话题", "二号话题", "三号话题", "四号话题", "五号话题", "六号话题"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizePublishTopics() = %#v, want %#v", got, want)
	}
}

func TestMarkdownTopicBodyFormatsHashtagsOnly(t *testing.T) {
	got := MarkdownTopicBody([]string{"AI代理", "数据安全", "工程反思"})

	if got != "#AI代理 #数据安全 #工程反思" {
		t.Fatalf("MarkdownTopicBody() = %q", got)
	}
}
