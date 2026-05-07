package ai

import (
	"encoding/json"
	"fmt"
	"strings"
)

const topicPromptTemplate = `你是一个小红书发布话题生成器。
请根据标题和 Markdown 原文生成中文小红书话题。
要求：
1. 只能输出 JSON，不要输出解释、代码块或额外文本
2. JSON 结构必须是 {"topics":["话题1","话题2"]}
3. topics 优先生成 3 个字符串，最多 4 个字符串
4. 每个话题不要包含 #
5. 优先选择小红书上更可能已经存在的泛话题，避免过细、过新或自造的组合词
6. 不要使用纯数字、日期、GitHub PR/issue 编号、链接、文件名或路径作为话题
7. 优先选择能概括文章主题、创作场景、技术领域或经验复盘的短话题
8. 每个话题建议 2-10 个中文字符或常见中英混合短语

标题：%s

Markdown 如下：
%s`

type TopicBuilder struct {
	Command string
	Args    []string
	Runner  CommandRunner
}

type topicResponse struct {
	Topics []string `json:"topics"`
}

func buildTopicPrompt(markdown, title string) string {
	return fmt.Sprintf(topicPromptTemplate, strings.TrimSpace(title), markdown)
}

func (b *TopicBuilder) SetCommand(command string, args []string) {
	b.Command = command
	b.Args = append([]string(nil), args...)
}

func (b TopicBuilder) effectiveRunner() CommandRunner {
	if b.Runner != nil {
		return b.Runner
	}
	return execRunner{}
}

func (b TopicBuilder) BuildPublishTopics(markdown, title string) ([]string, error) {
	args := append([]string{}, b.Args...)
	if shouldUseBareOutput(b.Command, b.Args) && !containsArg(args, "--bare") {
		args = append(args, "--bare")
	}
	args = append(args, "-p", buildTopicPrompt(markdown, title))
	stdout, stderr, err := b.effectiveRunner().Run(b.Command, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v\nstderr: %s", ErrAICommandFailed, err, stderr)
	}

	raw := stdout
	if shouldSanitizeCLIOutput(b.Command, b.Args) {
		raw = sanitizeCLIOutput(raw)
	}

	jsonStr, err := extractFirstJSONObject(trimNoise(raw))
	if err != nil {
		return nil, err
	}
	var response topicResponse
	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIInvalidJSON, err)
	}
	return append([]string(nil), response.Topics...), nil
}
