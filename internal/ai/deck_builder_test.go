package ai

import (
	"errors"
	"strings"
	"testing"
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
	if !strings.Contains(prompt, "最后一页必须是 ending") {
		t.Fatalf("prompt missing ending anchor: %q", prompt)
	}
	if !strings.Contains(prompt, "name、variant、meta、content") {
		t.Fatalf("prompt missing page field schema: %q", prompt)
	}
	if !strings.Contains(prompt, "badge、counter、theme、cta") {
		t.Fatalf("prompt missing required meta fields: %q", prompt)
	}
	if !strings.Contains(prompt, "cover 只能使用 title/subtitle") {
		t.Fatalf("prompt missing cover content whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "cover 的 title 必填") {
		t.Fatalf("prompt missing cover required title rule: %q", prompt)
	}
	if !strings.Contains(prompt, "quote 只能使用 title/quote/note/tip") {
		t.Fatalf("prompt missing quote content whitelist: %q", prompt)
	}
	if !strings.Contains(prompt, "quote 的 title 和 quote 必填") {
		t.Fatalf("prompt missing quote required fields: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 只能使用 title/body/images") {
		t.Fatalf("prompt missing image-caption body/images marker: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 的 title 必填") {
		t.Fatalf("prompt missing image-caption required title rule: %q", prompt)
	}
	if !strings.Contains(prompt, "image-caption 最多 1 张图") {
		t.Fatalf("prompt missing image-caption image limit: %q", prompt)
	}
	if !strings.Contains(prompt, "每个 image 都必须包含 src 和 alt") {
		t.Fatalf("prompt missing image src/alt rule: %q", prompt)
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
