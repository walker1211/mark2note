package ai

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	name   string
	args   []string
	stdout string
	stderr string
	err    error
}

func (r *fakeRunner) Run(name string, args ...string) (string, string, error) {
	r.name = name
	r.args = append([]string(nil), args...)
	return r.stdout, r.stderr, r.err
}

type runnerCall struct {
	stdout string
	stderr string
	err    error
}

type sequenceRunner struct {
	calls []runnerCall
	count int
}

func (r *sequenceRunner) Run(_ string, _ ...string) (string, string, error) {
	if r.count >= len(r.calls) {
		return "", "", errors.New("unexpected runner call")
	}
	call := r.calls[r.count]
	r.count++
	return call.stdout, call.stderr, call.err
}

func TestBuildDeckPromptKeepsLegacyLayoutWhenPromptExtraEmpty(t *testing.T) {
	got := buildDeckPrompt("# 标题", "")
	if !strings.Contains(got, "Markdown 如下：\n# 标题") {
		t.Fatalf("prompt missing markdown footer: %q", got)
	}
	if strings.Contains(got, "以下是本次生成的额外偏好") {
		t.Fatalf("prompt = %q, want no extra-guidance wrapper", got)
	}
}

func TestBuildDeckPromptWrapsPromptExtraBeforeMarkdown(t *testing.T) {
	got := buildDeckPrompt("# 标题", "封面更抓眼，少一点教程感")
	wrapper := promptExtraIntro
	if !strings.Contains(got, wrapper+"\n封面更抓眼，少一点教程感") {
		t.Fatalf("prompt missing wrapped extra guidance: %q", got)
	}
	if strings.Index(got, wrapper) > strings.Index(got, "Markdown 如下：") {
		t.Fatalf("extra guidance should appear before markdown footer: %q", got)
	}
}

func TestBuildDeckPromptTreatsPromptExtraAsHiddenConstraint(t *testing.T) {
	got := buildDeckPrompt("# 标题", "保留封面，但弱化冲击感")
	for _, want := range []string{"额外约束", "不得原文复制", "不得改写", "JSON 的任何可见字段", "title", "body", "cta", "steps", "images.alt"} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing hidden constraint %q: %q", want, got)
		}
	}
}

func TestBuildDeckPromptIgnoresWhitespaceOnlyPromptExtra(t *testing.T) {
	got := buildDeckPrompt("# 标题", "  \n\t  ")
	if strings.Contains(got, "以下是本次生成的额外偏好") {
		t.Fatalf("prompt = %q, want whitespace-only extra guidance ignored", got)
	}
	if strings.Count(got, "Markdown 如下：") != 1 {
		t.Fatalf("prompt markdown footer count = %d, want 1", strings.Count(got, "Markdown 如下："))
	}
}

func TestBuildDeckJSONUsesConfiguredMaxPages(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner, MaxPages: 18}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	prompt := runner.args[len(runner.args)-1]
	if !strings.Contains(prompt, "3-18 页") || !strings.Contains(prompt, "3 到 18 页之间") {
		t.Fatalf("prompt = %q, want configured max page range", prompt)
	}
}

func TestBuildDeckJSONUsesConfiguredCommand(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if runner.name != "ccs" {
		t.Fatalf("command = %q, want %q", runner.name, "ccs")
	}
	if len(runner.args) < 4 {
		t.Fatalf("args = %v, want command args plus --bare and -p prompt", runner.args)
	}
	if runner.args[0] != "codex" {
		t.Fatalf("first arg = %q, want %q", runner.args[0], "codex")
	}
	if runner.args[1] != "--bare" {
		t.Fatalf("args[1] = %q, want %q", runner.args[1], "--bare")
	}
	if runner.args[2] != "-p" {
		t.Fatalf("args[2] = %q, want %q", runner.args[2], "-p")
	}
	prompt := runner.args[3]
	if !strings.Contains(prompt, "# title") {
		t.Fatalf("prompt missing markdown input: %q", prompt)
	}
	if !strings.Contains(prompt, "3-12 页") {
		t.Fatalf("prompt missing dynamic page range: %q", prompt)
	}
	if !strings.Contains(prompt, "第一页必须是 cover") {
		t.Fatalf("prompt missing cover anchor: %q", prompt)
	}
	if !strings.Contains(prompt, "不要强制最后一页使用 ending") {
		t.Fatalf("prompt missing optional ending guidance: %q", prompt)
	}
	if strings.Contains(prompt, "最后一页必须是 ending") {
		t.Fatalf("prompt should not force ending as the last page: %q", prompt)
	}
	if !strings.Contains(prompt, "name、variant、meta、content") {
		t.Fatalf("prompt missing page field schema: %q", prompt)
	}
	if !strings.Contains(prompt, "badge、counter、theme、cta") {
		t.Fatalf("prompt missing required meta fields: %q", prompt)
	}
	if !strings.Contains(prompt, "meta.theme 保留为兼容字段，可使用 default") {
		t.Fatalf("prompt missing legacy meta.theme guidance: %q", prompt)
	}
	if strings.Contains(prompt, "meta.theme 只能使用 orange 或 green") {
		t.Fatalf("prompt still contains old visual meta.theme constraint: %q", prompt)
	}
	if !strings.Contains(prompt, "cover 只能使用 title/subtitle/images") {
		t.Fatalf("prompt missing cover content whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "cover 的 title 必填") {
		t.Fatalf("prompt missing cover required title rule: %q", prompt)
	}
	if !strings.Contains(prompt, "Markdown 开头第一张 alt 包含“封面”或 cover 的图应放入第一页 cover.images") {
		t.Fatalf("prompt missing first Markdown cover image guidance: %q", prompt)
	}
	if !strings.Contains(prompt, "quote 只能使用 title/quote/note/tip") {
		t.Fatalf("prompt missing quote content whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "quote 的 title 和 quote 必填") {
		t.Fatalf("prompt missing quote required fields: %q", prompt)
	}
	if !strings.Contains(prompt, "variant 只能使用：cover、quote、image-caption、text-caption、bullets、compare、gallery-steps、ending") {
		t.Fatalf("prompt missing text-caption in variant whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 只能使用 title/body/images") {
		t.Fatalf("prompt missing image-caption body/images marker: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 的 title 必填") {
		t.Fatalf("prompt missing image-caption required title rule: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 的 images 最多 1 项") {
		t.Fatalf("prompt missing image-caption max image count: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 必须提供 body 或 images") {
		t.Fatalf("prompt missing image-caption body-or-image rule: %q", prompt)
	}
	if strings.Contains(prompt, "image-caption 的 images 必须正好 1 项") {
		t.Fatalf("prompt should allow image-caption without an image when body exists: %q", prompt)
	}
	if !strings.Contains(prompt, "每个 image 都必须包含 src 和 alt") {
		t.Fatalf("prompt missing image src/alt rule: %q", prompt)
	}
	if !strings.Contains(prompt, "text-caption 只能使用 title/body/tip") {
		t.Fatalf("prompt missing text-caption content whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "text-caption 的 title 和 body 必填") {
		t.Fatalf("prompt missing text-caption required fields: %q", prompt)
	}
	if !strings.Contains(prompt, "bullets 只能使用 title/items") {
		t.Fatalf("prompt missing bullets content whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "bullets 的 title 必填、items 至少 1 项") {
		t.Fatalf("prompt missing bullets minimum items rule: %q", prompt)
	}
	if !strings.Contains(prompt, "compare 只能使用 title/compare") {
		t.Fatalf("prompt missing compare schema marker: %q", prompt)
	}
	if !strings.Contains(prompt, "compare 的 title 必填") {
		t.Fatalf("prompt missing compare required title rule: %q", prompt)
	}
	if !strings.Contains(prompt, "compare{leftLabel,rightLabel,rows}") {
		t.Fatalf("prompt missing compare object shape: %q", prompt)
	}
	if !strings.Contains(prompt, "compare.leftLabel/rightLabel 必填") {
		t.Fatalf("prompt missing compare required labels rule: %q", prompt)
	}
	if !strings.Contains(prompt, "rows 至少 1 项") {
		t.Fatalf("prompt missing compare minimum rows rule: %q", prompt)
	}
	if !strings.Contains(prompt, "每个 rows 项都必须包含 left 和 right") {
		t.Fatalf("prompt missing compare row left/right rule: %q", prompt)
	}
	if !strings.Contains(prompt, "gallery-steps 只能使用 title/steps/images") {
		t.Fatalf("prompt missing gallery-steps images marker: %q", prompt)
	}
	if !strings.Contains(prompt, "gallery-steps 的 title 必填") {
		t.Fatalf("prompt missing gallery-steps required title rule: %q", prompt)
	}
	if !strings.Contains(prompt, "steps 至少 2 个") {
		t.Fatalf("prompt missing gallery-steps minimum steps rule: %q", prompt)
	}
	if !strings.Contains(prompt, "每个 image 都必须包含 src 和 alt") {
		t.Fatalf("prompt missing gallery image src/alt rule: %q", prompt)
	}
	if !strings.Contains(prompt, "ending 只能使用 title/body") {
		t.Fatalf("prompt missing ending body marker: %q", prompt)
	}
	if !strings.Contains(prompt, "ending 的 title 和 body 必填") {
		t.Fatalf("prompt missing ending required fields: %q", prompt)
	}
}

func TestBuildDeckJSONPromptLocksMarkdownSemanticPreservation(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	markdown := "**bold** with `inline` code and a fenced block:\n```go\nfmt.Println(\"hello\")\n```"
	_, err := b.BuildDeckJSON(markdown)
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}

	if len(runner.args) < 4 {
		t.Fatalf("args = %v, want command args plus --bare and -p prompt", runner.args)
	}
	prompt := runner.args[3]
	required := []string{
		"必须保留原文里的 **bold**",
		"必须保留行内代码反引号",
		"在允许的字段中，必须保留 fenced code block（例如 body、note、tip），而不是改写成普通描述文本",
		"title/subtitle/cta/items/steps/compare 字段禁止承载 fenced code block；当字段不支持 block-level 代码块时，禁止把 fenced code block 塞进该字段",
		"如果为了适配页型而改写原句，仍然必须把原文中的重点词保留为 **bold**，不能把强调语义改写丢失成普通文本",
		"如果原文某些术语已使用行内代码反引号表示，改写后必须保留该行内代码语义，不得去掉",
	}
	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %q", want, prompt)
		}
	}
}

func TestBuildDeckJSONPromptPreservesFullVisibleCodeBlocks(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildDeckJSON("# 标题")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}

	if len(runner.args) < 4 {
		t.Fatalf("args = %v, want command args plus --bare and -p prompt", runner.args)
	}
	prompt := runner.args[3]
	for _, want := range []string{
		"每页可见内容必须完整放进 1242x1656 竖版卡片",
		"长正文和长代码块必须保留原文完整内容",
		"不要用省略号、省略说明或伪代码替代 fenced code block",
		"需要容纳长内容时优先选择 text-caption、image-caption 或 ending，由渲染层缩小字号和间距",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing full-content fit rule %q: %q", want, prompt)
		}
	}
	for _, forbidden := range []string{
		"fenced code block 超过 8 行时只保留关键命令和省略号",
		"不输出完整长脚本",
		"不超过 220 个中文字符",
		"不超过 180 个中文字符",
	} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt should not ask AI to truncate with %q: %q", forbidden, prompt)
		}
	}
}

func TestBuildDeckJSONPromptSplitsOversizedFencedCodeBlocks(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildDeckJSON("# 标题\n\n```bash\necho one\necho two\n```")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}

	if len(runner.args) < 4 {
		t.Fatalf("args = %v, want command args plus --bare and -p prompt", runner.args)
	}
	prompt := runner.args[3]
	for _, want := range []string{
		"fenced code block 过长时不要强行塞进单页",
		"拆成连续的 text-caption 或 image-caption 页面",
		"每页保留连续、完整、可执行的原始代码片段",
		"不得用省略号替代被拆分的代码",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing long-code split rule %q: %q", want, prompt)
		}
	}
}

func TestBuildPublishTopicsUsesConfiguredCommand(t *testing.T) {
	runner := &fakeRunner{stdout: `{"topics":["AI编程","开源项目","工程实践"]}`}
	b := TopicBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	got, err := b.BuildPublishTopics("# 标题\n\n正文", "标题")
	if err != nil {
		t.Fatalf("BuildPublishTopics() error = %v", err)
	}
	want := []string{"AI编程", "开源项目", "工程实践"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildPublishTopics() = %#v, want %#v", got, want)
	}
	if runner.name != "ccs" {
		t.Fatalf("command = %q, want ccs", runner.name)
	}
	if len(runner.args) < 4 || runner.args[0] != "codex" || runner.args[1] != "--bare" || runner.args[2] != "-p" {
		t.Fatalf("args = %#v, want codex --bare -p prompt", runner.args)
	}
	prompt := runner.args[3]
	for _, want := range []string{"只能输出 JSON", `{"topics":["话题1","话题2"]}`, "优先生成 3 个", "最多 4 个", "更可能已经存在的泛话题", "不要使用纯数字", "标题：标题", "Markdown 如下：\n# 标题"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("topic prompt missing %q: %q", want, prompt)
		}
	}
}

func TestBuildPublishTopicsRejectsInvalidJSON(t *testing.T) {
	runner := &fakeRunner{stdout: `not json`}
	b := TopicBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildPublishTopics("# 标题", "标题")
	if !errors.Is(err, ErrAINoJSONFound) {
		t.Fatalf("BuildPublishTopics() error = %v, want ErrAINoJSONFound", err)
	}
}

func TestBuildPublishTopicsRetriesTransientAILockError(t *testing.T) {
	runner := &sequenceRunner{calls: []runnerCall{
		{stderr: "[X] Lock file is already being held", err: errors.New("exit status 1")},
		{stdout: `{"topics":["AI编程"]}`},
	}}
	b := TopicBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})
	b.SetRetryDelays([]time.Duration{0})

	got, err := b.BuildPublishTopics("# 标题", "标题")
	if err != nil {
		t.Fatalf("BuildPublishTopics() error = %v", err)
	}
	want := []string{"AI编程"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildPublishTopics() = %#v, want %#v", got, want)
	}
	if runner.count != 2 {
		t.Fatalf("runner calls = %d, want 2", runner.count)
	}
}

func TestBuildPublishTitleUsesConfiguredCommand(t *testing.T) {
	runner := &fakeRunner{stdout: `{"title":"把代码合进真实开源"}`}
	b := TitleBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	got, err := b.BuildPublishTitle("# 别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始", "别只用 AI 写 Demo：把代码合进真实开源项目，那会是新的开始", 20)
	if err != nil {
		t.Fatalf("BuildPublishTitle() error = %v", err)
	}
	if got != "把代码合进真实开源" {
		t.Fatalf("BuildPublishTitle() = %q", got)
	}
	if runner.name != "ccs" {
		t.Fatalf("command = %q, want ccs", runner.name)
	}
	if len(runner.args) < 4 || runner.args[0] != "codex" || runner.args[1] != "--bare" || runner.args[2] != "-p" {
		t.Fatalf("args = %#v, want codex --bare -p prompt", runner.args)
	}
	prompt := runner.args[3]
	for _, want := range []string{"只能输出 JSON", `{"title":"改写后的标题"}`, "不超过 20 个字符", "保留原始标题的核心意思", "不要把标题压缩成电报式短语", "保持自然中文语序", "原始标题：别只用 AI 写 Demo", "Markdown 如下：\n# 别只用 AI 写 Demo"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("title prompt missing %q: %q", want, prompt)
		}
	}
}

func TestBuildPublishTitleRejectsInvalidJSON(t *testing.T) {
	runner := &fakeRunner{stdout: `not json`}
	b := TitleBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildPublishTitle("# 标题", "标题", 20)
	if !errors.Is(err, ErrAINoJSONFound) {
		t.Fatalf("BuildPublishTitle() error = %v, want ErrAINoJSONFound", err)
	}
}

func TestBuildPublishTitleRetriesTransientAILockError(t *testing.T) {
	runner := &sequenceRunner{calls: []runnerCall{
		{stderr: "[X] Lock file is already being held", err: errors.New("exit status 1")},
		{stdout: `{"title":"短标题"}`},
	}}
	b := TitleBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})
	b.SetRetryDelays([]time.Duration{0})

	got, err := b.BuildPublishTitle("# 标题", "标题", 20)
	if err != nil {
		t.Fatalf("BuildPublishTitle() error = %v", err)
	}
	if got != "短标题" {
		t.Fatalf("BuildPublishTitle() = %q, want retry title", got)
	}
	if runner.count != 2 {
		t.Fatalf("runner calls = %d, want 2", runner.count)
	}
}

func TestBuildPublishTitleReturnsStderrOnRunnerError(t *testing.T) {
	runner := &fakeRunner{stderr: "boom", err: errors.New("exit status 1")}
	b := TitleBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildPublishTitle("# 标题", "标题", 20)
	if err == nil {
		t.Fatalf("BuildPublishTitle() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrAICommandFailed) {
		t.Fatalf("error = %v, want ErrAICommandFailed", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want stderr included", err)
	}
}

func TestBuildPublishTitleDoesNotDuplicateBareForCCSCodex(t *testing.T) {
	runner := &fakeRunner{stdout: `{"title":"短标题"}`}
	b := TitleBuilder{Runner: runner}
	b.SetCommand("ccs", []string{"codex", "--bare"})

	_, err := b.BuildPublishTitle("# 标题", "标题", 20)
	if err != nil {
		t.Fatalf("BuildPublishTitle() error = %v", err)
	}
	bareCount := 0
	for _, arg := range runner.args {
		if arg == "--bare" {
			bareCount++
		}
	}
	if bareCount != 1 {
		t.Fatalf("--bare count = %d, want 1; args = %v", bareCount, runner.args)
	}
}

func TestBuildDeckJSONReturnsStderrOnRunnerError(t *testing.T) {
	runner := &fakeRunner{stderr: "boom", err: errors.New("exit status 1")}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildDeckJSON("# title")
	if err == nil {
		t.Fatalf("BuildDeckJSON() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrAICommandFailed) {
		t.Fatalf("error = %v, want ErrAICommandFailed", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want stderr included", err)
	}
}

func TestBuildDeckJSONRetriesTransientAILockError(t *testing.T) {
	runner := &sequenceRunner{calls: []runnerCall{
		{stderr: "[X] Lock file is already being held", err: errors.New("exit status 1")},
		{stdout: `{"pages":[]}`},
	}}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})
	b.SetRetryDelays([]time.Duration{0})

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"pages":[]}` {
		t.Fatalf("BuildDeckJSON() = %q, want JSON from retry", got)
	}
	if runner.count != 2 {
		t.Fatalf("runner calls = %d, want 2", runner.count)
	}
}

func TestBuildDeckJSONUsesConfiguredRetryDelays(t *testing.T) {
	runner := &sequenceRunner{calls: []runnerCall{
		{stderr: "timeout", err: errors.New("exit status 1")},
		{stderr: "timeout", err: errors.New("exit status 1")},
		{stdout: `{"pages":[]}`},
	}}
	b := Builder{Runner: runner}
	b.SetCommand("custom-ai", nil)
	b.SetRetryDelays([]time.Duration{0, 0})

	_, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if runner.count != 3 {
		t.Fatalf("runner calls = %d, want len(delays)+1", runner.count)
	}
}

func TestBuildDeckJSONAppendsBareForCCSCodex(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	_, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if len(runner.args) < 4 {
		t.Fatalf("args = %v, want codex --bare -p <prompt>", runner.args)
	}
	if runner.args[1] != "--bare" {
		t.Fatalf("args[1] = %q, want %q", runner.args[1], "--bare")
	}
}

func TestBuildDeckJSONDoesNotDuplicateBareForCCSCodex(t *testing.T) {
	runner := &fakeRunner{stdout: `{"pages":[]}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex", "--bare"})

	_, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}

	bareCount := 0
	for _, arg := range runner.args {
		if arg == "--bare" {
			bareCount++
		}
	}
	if bareCount != 1 {
		t.Fatalf("--bare count = %d, want 1; args = %v", bareCount, runner.args)
	}
}

func TestBuildDeckJSONSanitizesCCSCodexPreamble(t *testing.T) {
	runner := &fakeRunner{stdout: "[i] CLIProxy Plus update: v1 -> v2\n[i] Joined existing CLIProxy on port 8317 (http)\n{\"pages\":[]}"}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"pages":[]}` {
		t.Fatalf("BuildDeckJSON() = %q, want pure json", got)
	}
}

func TestBuildDeckJSONKeepsValidJSONContainingCLIProxy(t *testing.T) {
	runner := &fakeRunner{stdout: `{"title":"CLIProxy"}`}
	b := Builder{Runner: runner}
	b.SetCommand("ccs", []string{"codex"})

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"title":"CLIProxy"}` {
		t.Fatalf("BuildDeckJSON() = %q, want JSON preserved", got)
	}
}

func TestBuildDeckJSONExtractsJSONObjectFromMixedOutput(t *testing.T) {
	runner := &fakeRunner{stdout: "info line\n{\"pages\":[]}\ntrailing note"}
	b := Builder{Runner: runner}
	b.SetCommand("other-cli", nil)

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"pages":[]}` {
		t.Fatalf("BuildDeckJSON() = %q, want pure json", got)
	}
}

func TestBuildDeckJSONSkipsInvalidBraceNoiseBeforeValidJSONObject(t *testing.T) {
	runner := &fakeRunner{stdout: "log {not-json} noise\n{\"pages\":[]}"}
	b := Builder{Runner: runner}
	b.SetCommand("other-cli", nil)

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"pages":[]}` {
		t.Fatalf("BuildDeckJSON() = %q, want pure json", got)
	}
}

func TestBuildDeckJSONExtractsJSONObjectFromFencedBlock(t *testing.T) {
	runner := &fakeRunner{stdout: "```json\n{\"pages\":[]}\n```"}
	b := Builder{Runner: runner}
	b.SetCommand("other-cli", nil)

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"pages":[]}` {
		t.Fatalf("BuildDeckJSON() = %q, want pure json", got)
	}
}

func TestBuildDeckJSONRecoversValidJSONObjectAfterUnbalancedNoise(t *testing.T) {
	runner := &fakeRunner{stdout: "prefix {\nnoise line\n{\"pages\":[]}"}
	b := Builder{Runner: runner}
	b.SetCommand("other-cli", nil)

	got, err := b.BuildDeckJSON("# title")
	if err != nil {
		t.Fatalf("BuildDeckJSON() error = %v", err)
	}
	if got != `{"pages":[]}` {
		t.Fatalf("BuildDeckJSON() = %q, want pure json", got)
	}
}

func TestBuildDeckJSONReturnsNoJSONErrorWhenOutputContainsNoJSONObject(t *testing.T) {
	runner := &fakeRunner{stdout: "model explanation only"}
	b := Builder{Runner: runner}
	b.SetCommand("other-cli", nil)

	_, err := b.BuildDeckJSON("# title")
	if err == nil {
		t.Fatalf("BuildDeckJSON() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrAINoJSONFound) {
		t.Fatalf("error = %v, want ErrAINoJSONFound", err)
	}
}

func TestBuildDeckJSONReturnsInvalidJSONErrorForUnbalancedObject(t *testing.T) {
	runner := &fakeRunner{stdout: "prefix {\"pages\":[}"}
	b := Builder{Runner: runner}
	b.SetCommand("other-cli", nil)

	_, err := b.BuildDeckJSON("# title")
	if err == nil {
		t.Fatalf("BuildDeckJSON() error = nil, want non-nil")
	}
	if !errors.Is(err, ErrAIInvalidJSON) {
		t.Fatalf("error = %v, want ErrAIInvalidJSON", err)
	}
}
