package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const topicPromptTemplate = `你是一个小红书发布话题生成器。
请根据标题和 Markdown 原文生成中文小红书话题。
要求：
1. 只能输出 JSON，不要输出解释、代码块或额外文本
2. JSON 结构必须是 {"topics":["话题1","话题2"]}
3. topics 优先生成 3 个字符串，最多 4 个字符串
4. 每个话题不要包含 #
5. 每个话题必须短，建议 2-10 个中文字符或常见中英混合短语
6. 每个话题不能包含空格、制表符或换行
7. 每个话题不能包含特殊符号或标点，例如：｜、【】、（）、[]、!、?、/、\\、:、：、#、@
8. 不要直接复制很长的视频标题、文章标题或章节标题
9. 不要包含点赞、收藏、投币、播放量、日期、序号、纯数字、链接、文件名或路径
10. 优先选择小红书上更可能已经存在的泛话题，避免过细、过新或自造的组合词
11. 优先选择能概括文章主题、创作场景、技术领域、娱乐类别或经验复盘的短话题
12. 如果 Markdown 是“电子榨菜”内容，请根据每个视频标题归纳短话题，例如影视、综艺、游戏、动画、足球、理财、生活记录、AI工具等；不要输出原始长视频标题

标题：%s

Markdown 如下：
%s`

type TopicBuilder struct {
	Command     string
	Args        []string
	RetryDelays []time.Duration
	Runner      CommandRunner
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

func (b *TopicBuilder) SetRetryDelays(delays []time.Duration) {
	b.RetryDelays = cloneDurations(delays)
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
	stdout, stderr, err := runAICommand(b.effectiveRunner(), b.Command, b.RetryDelays, args...)
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
