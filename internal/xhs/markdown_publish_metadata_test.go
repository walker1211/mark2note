package xhs

import (
	"reflect"
	"slices"
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

func TestMarkdownPublishTopicsPrefersFrontmatterAndHashtags(t *testing.T) {
	markdown := `---
tags: [AI代理, 数据安全]
---

# 标题

这里提到了 #工程反思，也提到了 #AI代理。
`

	got := MarkdownPublishTopics(markdown, "标题", nil)
	wantPrefix := []string{"AI代理", "数据安全", "工程反思"}

	if len(got) < len(wantPrefix) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want prefix %#v", got, wantPrefix)
	}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want prefix %#v", got, wantPrefix)
	}
}

func TestMarkdownPublishTopicsReadsYAMLListFrontmatterTags(t *testing.T) {
	markdown := `---
tags:
  - AI代理
  - 数据安全
---

# 标题
`

	got := MarkdownPublishTopics(markdown, "标题", nil)
	wantPrefix := []string{"AI代理", "数据安全"}

	if len(got) < len(wantPrefix) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want prefix %#v", got, wantPrefix)
	}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want prefix %#v", got, wantPrefix)
	}
}

func TestMarkdownPublishTopicsReadsQuotedInlineFrontmatterTags(t *testing.T) {
	markdown := `---
tags: ["AI代理", '数据安全']
---

# 标题
`

	got := MarkdownPublishTopics(markdown, "标题", nil)
	wantPrefix := []string{"AI代理", "数据安全"}

	if len(got) < len(wantPrefix) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want prefix %#v", got, wantPrefix)
	}
	if !reflect.DeepEqual(got[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want prefix %#v", got, wantPrefix)
	}
}

func TestMarkdownPublishTopicsIgnoresNonTagFrontmatterValues(t *testing.T) {
	markdown := `---
summary: "#假话题"
category: 假分类
---

# 标题
正文 #真话题
`

	got := MarkdownPublishTopics(markdown, "标题", nil)

	for _, unexpected := range []string{"假话题", "假分类"} {
		if slices.Contains(got, unexpected) {
			t.Fatalf("MarkdownPublishTopics() = %#v, should ignore frontmatter value %q", got, unexpected)
		}
	}
	if !slices.Contains(got, "真话题") {
		t.Fatalf("MarkdownPublishTopics() = %#v, want body hashtag", got)
	}
}

func TestMarkdownPublishTopicsIgnoresHashtagsInFencedCodeBlocks(t *testing.T) {
	markdown := "# 标题\n\n```markdown\n#假话题\n```\n\n正文 #真话题"

	got := MarkdownPublishTopics(markdown, "标题", nil)

	if containsString(got, "假话题") {
		t.Fatalf("MarkdownPublishTopics() = %#v, should ignore fenced hashtag", got)
	}
	if !containsString(got, "真话题") {
		t.Fatalf("MarkdownPublishTopics() = %#v, want real hashtag", got)
	}
}

func TestMarkdownPublishTopicsIgnoresHeadingsInFencedCodeBlocks(t *testing.T) {
	markdown := "# 标题\n\n```markdown\n## 假话题\n```\n\n## 真话题"

	got := MarkdownPublishTopics(markdown, "标题", nil)

	if containsString(got, "假话题") {
		t.Fatalf("MarkdownPublishTopics() = %#v, should ignore fenced heading", got)
	}
	if !containsString(got, "真话题") {
		t.Fatalf("MarkdownPublishTopics() = %#v, want real heading", got)
	}
}

func TestMarkdownPublishTopicsStripsATXClosingHeadingMarker(t *testing.T) {
	got := MarkdownPublishTopics("# 标题\n\n## 真实话题 ##", "标题", nil)

	if !slices.Contains(got, "真实话题") {
		t.Fatalf("MarkdownPublishTopics() = %#v, want heading topic", got)
	}
	if slices.Contains(got, "真实话题##") {
		t.Fatalf("MarkdownPublishTopics() = %#v, should strip closing marker", got)
	}
}

func TestMarkdownPublishTopicsUsesManualOverride(t *testing.T) {
	markdown := "# 原文标题\n\n#旧话题"

	got := MarkdownPublishTopics(markdown, "原文标题", []string{"#AI代理", " 数据安全 ", "AI代理", "工程反思"})
	want := []string{"AI代理", "数据安全", "工程反思"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want %#v", got, want)
	}
}

func TestMarkdownPublishTopicsInfersFromTitleAndBody(t *testing.T) {
	markdown := `# 一个AI代理删库之后我开始关心刹车

## 数据安全复盘

AI代理需要权限边界。数据安全需要工程刹车。`

	got := MarkdownPublishTopics(markdown, "一个AI代理删库之后我开始关心刹车", nil)

	for _, want := range []string{"AI代理", "数据安全"} {
		if !slices.Contains(got, want) {
			t.Fatalf("MarkdownPublishTopics() = %#v, want %q", got, want)
		}
	}
	if len(got) == 0 || len(got) > 6 {
		t.Fatalf("MarkdownPublishTopics() returned %d topics: %#v", len(got), got)
	}
}

func TestMarkdownPublishTopicsInferredBodyOrderingIsDeterministic(t *testing.T) {
	markdown := "# 标题\n\n苹果香蕉。香蕉苹果。猫狗同行。同行猫狗。"

	got := MarkdownPublishTopics(markdown, "标题", nil)
	want := []string{"同行猫狗", "猫狗同行", "苹果香蕉", "香蕉苹果"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want %#v", got, want)
	}
}

func TestMarkdownPublishTopicsPadsAutomaticTopicsToThree(t *testing.T) {
	got := MarkdownPublishTopics("# 短标题", "短标题", nil)
	want := []string{"短标题", "小红书", "图文笔记"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want %#v", got, want)
	}
}

func TestMarkdownPublishTopicsAllowsManualOverrideBelowThree(t *testing.T) {
	got := MarkdownPublishTopics("# 短标题", "短标题", []string{"AI代理"})
	want := []string{"AI代理"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want %#v", got, want)
	}
}

func TestMarkdownPublishTopicsLimitsToSix(t *testing.T) {
	got := MarkdownPublishTopics("", "标题", []string{"一号话题", "二号话题", "三号话题", "四号话题", "五号话题", "六号话题", "七号话题"})

	want := []string{"一号话题", "二号话题", "三号话题", "四号话题", "五号话题", "六号话题"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MarkdownPublishTopics() = %#v, want %#v", got, want)
	}
}

func TestMarkdownTopicBodyFormatsHashtagsOnly(t *testing.T) {
	got := MarkdownTopicBody([]string{"AI代理", "数据安全", "工程反思"})

	if got != "#AI代理 #数据安全 #工程反思" {
		t.Fatalf("MarkdownTopicBody() = %q", got)
	}
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}
