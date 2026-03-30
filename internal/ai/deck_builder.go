package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const deckPrompt = `你是一个严格的 JSON 生成器。
请把输入的 Markdown 内容转换成 3-12 页动态小红书预览 deck JSON。
要求：
1. 只能输出 JSON，不要输出解释、代码块或额外文本
2. pages 数量必须在 3 到 12 页之间（3-12 页）
3. 第一页必须是 cover
4. 最后一页必须是 ending
5. 每页结构必须包含：name、variant、meta、content
6. 每页 name 必须唯一，建议使用顺序编号命名
7. 每页 meta 必须包含：badge、counter、theme、cta
8. meta.theme 只能使用 orange 或 green
9. variant 只能使用：cover、quote、image-caption、bullets、compare、gallery-steps、ending
10. content 字段按 variant 严格约束：
   - cover 只能使用 title/subtitle，且 cover 的 title 必填
   - quote 只能使用 title/quote/note/tip，且 quote 的 title 和 quote 必填
   - image-caption 只能使用 title/body/images，且 image-caption 的 title 必填、image-caption 最多 1 张图；如果提供 images，每个 image 都必须包含 src 和 alt
   - bullets 只能使用 title/items，且 bullets 的 title 必填、items 至少 1 项
   - compare 只能使用 title/compare，且 compare 的 title 必填、compare 必须使用 compare{leftLabel,rightLabel,rows}、compare.leftLabel/rightLabel 必填、rows 至少 1 项，且每个 rows 项都必须包含 left 和 right
   - gallery-steps 只能使用 title/steps/images，且 gallery-steps 的 title 必填、steps 至少 2 个；如果提供 images，每个 image 都必须包含 src 和 alt
   - ending 只能使用 title/body，且 ending 的 title 和 body 必填
11. JSON 结构必须与 Go deck 结构兼容

Markdown 如下：
`

var (
	ErrAICommandFailed = errors.New("ai command failed")
	ErrAINoJSONFound   = errors.New("ai output did not contain a json object")
	ErrAIInvalidJSON   = errors.New("ai output contained invalid json")
)

type CommandRunner interface {
	Run(name string, args ...string) (string, string, error)
}

type Builder struct {
	Command string
	Args    []string
	Runner  CommandRunner
}

type execRunner struct{}

func (execRunner) Run(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), "", nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return "", string(exitErr.Stderr), err
	}
	return "", "", err
}

func (b *Builder) SetCommand(command string, args []string) {
	b.Command = command
	b.Args = append([]string(nil), args...)
}

func (b Builder) effectiveRunner() CommandRunner {
	if b.Runner != nil {
		return b.Runner
	}
	return execRunner{}
}

func (b Builder) BuildDeckJSON(markdown string) (string, error) {
	args := append([]string{}, b.Args...)
	if shouldUseBareOutput(b.Command, b.Args) && !containsArg(args, "--bare") {
		args = append(args, "--bare")
	}
	args = append(args, "-p", deckPrompt+"\n"+markdown)
	stdout, stderr, err := b.effectiveRunner().Run(b.Command, args...)
	if err != nil {
		return "", fmt.Errorf("%w: %v\nstderr: %s", ErrAICommandFailed, err, stderr)
	}

	raw := stdout
	if shouldSanitizeCLIOutput(b.Command, b.Args) {
		raw = sanitizeCLIOutput(raw)
	}

	jsonStr, err := extractFirstJSONObject(trimNoise(raw))
	if err != nil {
		return "", err
	}
	return jsonStr, nil
}

func trimNoise(raw string) string {
	return strings.TrimSpace(raw)
}

func extractFirstJSONObject(raw string) (string, error) {
	sawObjectLike := false

	for start := 0; start < len(raw); start++ {
		if raw[start] != '{' {
			continue
		}
		sawObjectLike = true

		depth := 0
		inString := false
		escaped := false

		for i := start; i < len(raw); i++ {
			ch := raw[i]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}

			if ch == '"' {
				inString = true
				continue
			}
			if ch == '{' {
				depth++
				continue
			}
			if ch == '}' {
				if depth == 0 {
					continue
				}
				depth--
				if depth == 0 {
					candidate := strings.TrimSpace(raw[start : i+1])
					if json.Valid([]byte(candidate)) {
						return candidate, nil
					}
					break
				}
			}
		}
	}

	if !sawObjectLike {
		return "", ErrAINoJSONFound
	}
	return "", ErrAIInvalidJSON
}

func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func shouldUseBareOutput(command string, args []string) bool {
	if command != "ccs" {
		return false
	}
	return containsArg(args, "codex")
}

func shouldSanitizeCLIOutput(command string, args []string) bool {
	return shouldUseBareOutput(command, args)
}

func sanitizeCLIOutput(raw string) string {
	lines := strings.Split(raw, "\n")
	idx := 0
	for idx < len(lines) {
		line := strings.TrimSpace(lines[idx])
		if line == "" {
			idx++
			continue
		}
		if strings.HasPrefix(line, "[i] ") || strings.HasPrefix(line, "[OK] ") || strings.HasPrefix(line, "[warn] ") || strings.Contains(line, "CLIProxy") || strings.HasPrefix(line, `Run "ccs cliproxy stop"`) {
			idx++
			continue
		}
		break
	}
	return strings.TrimSpace(strings.Join(lines[idx:], "\n"))
}
