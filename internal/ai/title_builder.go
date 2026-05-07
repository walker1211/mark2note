package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

const publishTitlePromptTemplate = `你是一个小红书发布标题改写器。
请把原始标题改写成适合小红书发布的中文标题。
要求：
1. 只能输出 JSON，不要输出解释、代码块或额外文本
2. JSON 结构必须是 {"title":"改写后的标题"}
3. title 必须不超过 %d 个字符；中文、英文、数字、空格和标点都按 1 个字符计算
4. 保留原始标题的核心意思，不要编造原文没有的信息
5. 标题要自然、有信息量，不要像关键词堆砌
6. 不要把标题压缩成电报式短语或生硬的名词堆叠
7. 保持自然中文语序，读起来像人写的标题
8. 可以删减修饰语，但不能改丢原题的主谓宾、转折或因果关系

原始标题：%s

Markdown 如下：
%s`

type TitleBuilder struct {
	Command string
	Args    []string
	Runner  CommandRunner
}

type titleResponse struct {
	Title string `json:"title"`
}

func buildPublishTitlePrompt(markdown, title string, maxRunes int) string {
	return fmt.Sprintf(publishTitlePromptTemplate, maxRunes, strings.TrimSpace(title), markdown)
}

func (b *TitleBuilder) SetCommand(command string, args []string) {
	b.Command = command
	b.Args = append([]string(nil), args...)
}

func (b TitleBuilder) effectiveRunner() CommandRunner {
	if b.Runner != nil {
		return b.Runner
	}
	return execRunner{}
}

func (b TitleBuilder) BuildPublishTitle(markdown, title string, maxRunes int) (string, error) {
	args := append([]string{}, b.Args...)
	if shouldUseBareOutput(b.Command, b.Args) && !containsArg(args, "--bare") {
		args = append(args, "--bare")
	}
	args = append(args, "-p", buildPublishTitlePrompt(markdown, title, maxRunes))
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
	var response titleResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return "", fmt.Errorf("%w: %v", ErrAIInvalidJSON, err)
	}
	return strings.TrimSpace(response.Title), nil
}
