# mark2note tech-noir 语义高亮设计

## 背景

当前 `tech-noir` 已经具备基础深色主题 token，但实际输出仍然更像“统一换成深色配色”，还达不到参考图里那种**同一页内只有重点内容变色、不同语义块有不同视觉身份**的效果。

用户已确认以下约束：
- `editorial-mono` 当前可用，本轮只聚焦 `tech-noir`
- 同一份 Markdown 既要用于图片生成，也要复用于公众号等发布链路
- 不能为了图片渲染在原始 Markdown 中引入额外污染性标记
- 不接受额外 sidecar 文件作为日常必需输入
- 希望尽量复用 Markdown 已有语义，而不是让程序自动猜哪些词需要高亮

这意味着本轮问题的核心不是“再调一轮深色 token”，而是为现有渲染链路补一层**语义到视觉的映射能力**。

## 目标

让 `tech-noir` 从“深色主题”升级为“语义化高对比主题”，在不破坏现有 Markdown 复用方式的前提下，为以下内容提供更强视觉表达：

1. 行内代码
2. 代码块
3. 加粗文本
4. 数字 / 序号 / 百分比

设计目标：
- 直接复用 Markdown 现有语义：`**bold**`、`` `inline code` ``、``` fenced code block ```
- 保持原始 Markdown 仍然适合公众号等其他渠道使用
- 不依赖自动关键词猜测
- 不引入完整富文本 / Markdown 渲染器重写
- 让 `tech-noir` 明显受益，其他主题只做最小继承或保持克制

## 非目标

本次明确不做：

1. 自动关键词高亮
2. 代码语法高亮
3. 完整 Markdown 富文本渲染系统
4. 为 `tech-noir` 分叉一套专用模板体系
5. 修改默认主题既有视觉策略
6. 强制所有主题都采用与 `tech-noir` 同等强度的语义着色
7. 进度条能力；本轮不新增 progress 数据结构、模板节点或样式契约，进度条留到后续单独设计

## 仓库现状与约束

### 当前数据链路

现有输入链路是：

1. CLI 读取 Markdown 原文
2. `internal/ai/deck_builder.go` 将原始 Markdown 拼进 `deckPrompt`
3. AI CLI 输出 deck JSON
4. `internal/deck` 负责 JSON 解析与结构校验
5. `internal/render` 根据 `deck.Page` 渲染 HTML / PNG

这里的关键事实是：**仓库当前并没有一个本地 Markdown AST 渲染器**。Markdown 语义是否能进入页面结构，取决于 AI 生成 deck JSON 时是否保留，或者后处理阶段是否显式解析字段文本。

### 当前模板限制

`internal/render/assets/page.tmpl` 现在直接输出纯字符串字段，例如：
- `{{.Page.Content.Title}}`
- `{{.Page.Content.Quote}}`
- `{{.Page.Meta.CTA}}`

因此当前系统只能做到“整段一个样式”，不能做到“同一段里局部强调”、“代码片段胶囊化”或“代码块卡片化”。

### 当前主题变量限制

`internal/render/theme_vars.go` 目前只有基础 token 和少量共享变量，例如：
- `--accent`
- `--accent-soft`
- `--accent-foreground`
- `--watermark-color`
- `--cta-shadow`

还没有为“强调文字 / 数字 / 代码块”这类语义层准备独立变量。

## 设计结论

### 核心方向

采用**Markdown 已有语义 + 少量结构化语义映射**的方案，而不是额外标记或自动猜词。

本轮明确边界：
- 不做正文里“自动识别并高亮任意数字/百分比”的规则型猜测
- 数字 / 序号 / 百分比高亮只来自两类确定来源：
  1. Markdown 原生语义承载的数字（如 `**80%**`、`` `v1.2.3` ``）
  2. 现有页型中本来就结构化存在的位置（如 bullet index、step number、compare 右侧结果）

具体映射如下：

#### 1. 加粗 → 强调文本
- 来源：`**bold**`
- `tech-noir` 表现：金色或强调色 + 更高字重
- 主要用于标题重点词、结论词、CTA 核心动作

#### 2. 行内代码 → 深色胶囊 / 描边块
- 来源：`` `inline code` ``
- 表现：单独背景、边框、内边距、等宽或近等宽感
- 不做语法着色，只做“代码身份”高亮

#### 3. 代码块 → 统一代码卡片
- 来源：``` fenced code block ```
- 表现：统一卡片底 + 统一前景色 + 统一留白
- 不做语法高亮，不区分语言主题

#### 4. 数字 / 序号 / 百分比 → 数值强调
- 来源：
  - 页面结构内现有序号，如 bullet index / step number
  - compare 右侧结论值或结果值
  - 作者已经用 `**...**` 或反引号明确承载的数字 / 百分比
- 表现：单独强调色，优先与正文拉开层次

## 语义来源策略

### 复用 Markdown 自带语义

用户最关心的是“同一份 Markdown 能继续发公众号”。因此优先复用这些原生语义：
- `**bold**`
- `` `inline code` ``
- ``` fenced code block ```

其中 fenced code block 只允许落在能承载长文本的字段中：
- `body`
- `note`
- `tip`

本轮不支持在以下字段中渲染 block-level 代码块：
- `title`
- `subtitle`
- `cta`
- `items`
- `steps`
- `compare.leftLabel`
- `compare.rightLabel`
- `compare.rows[*].left`
- `compare.rows[*].right`

这些字段只允许普通文本、加粗片段和行内代码，不允许块级代码内容进入版式。

它们在公众号链路里仍是自然语义，在 mark2note 链路里则进一步映射成更强的视觉身份。

### 结构化语义继续来自现有页型

以下内容不依赖 Markdown 额外标记，而是由现有页型天然提供：
- bullets 的序号
- gallery-steps 的 step number
- compare 右侧结果区域

也就是说，本轮并不需要把所有高亮都塞进 Markdown 解析器；一部分仍然来自现有页面结构。

## 架构设计

### 1. 内容表示层：在 render 层派生轻量 rich text view model

本轮不修改 `deck.PageContent` 的 JSON schema，也不把现有字符串字段直接改成复杂结构。

实现落点应收敛为：
- `internal/deck` 继续保持当前字符串 schema 与校验方式
- 在 `internal/render` 渲染前，为需要的字段派生一个轻量 rich text view model
- 模板消费这个派生 view model，而不是直接消费原始字符串

建议片段类型只控制在本次需要的最小集合：
- 普通文本
- 强调文本
- 行内代码
- 代码块
- 数字强调（仅来自明确语义或结构化位置）

重要边界：
- 这不是完整富文本引擎
- 不支持任意嵌套样式组合
- 不支持链接、图片、表格等 Markdown 全量语义
- 只支持本轮已确认的几类表达
- `deck` 层保持 schema 稳定，富文本能力先作为 render 阶段的派生表示存在

### 2. 语义提取层：最小解析，不重做整个 Markdown 系统

考虑到当前链路是 AI 先产出 deck JSON，本轮设计建议采用“两段式保留”思路：

#### 路径 A：提示词约束优先
在 `internal/ai/deck_builder.go` 的 prompt 中把以下要求写成强制规则，而不是建议项：
- 必须保留原文里的 `**bold**`
- 必须保留行内代码反引号
- 在允许的字段中，必须保留 fenced code block，而不是改写成普通描述文本
- 当字段不支持 block-level 代码块时，禁止把 fenced code block 塞进该字段

这里的语义保留不是“尽量做到”，而是 v1 的输入契约：如果 AI 输出抹掉这些 Markdown 语义，渲染层就无法恢复强调/代码/代码块身份，因此该行为应被视为不满足本轮设计要求。

#### 路径 B：渲染前字段级轻解析
在 `internal/render` 或 `internal/deck` 附近，对这些字符串字段做字段级轻解析：
- title
- subtitle
- body
- quote
- note
- tip
- cta
- items
- steps
- compare labels / rows

解析目标不是“理解整篇 Markdown”，而只是把已存在于字符串中的 `**`、反引号、代码块拆成片段。

这保证即使 AI 只是把原有 Markdown 语义原样搬进 JSON，也能在后处理阶段恢复成可渲染的语义片段。

### 3. 模板层：从直出字符串改为渲染片段

`internal/render/assets/page.tmpl` 需要从：
- 直接输出 `{{.Page.Content.Title}}`

改成：
- 只在允许富文本的字段上改为消费 rich text view model
- 普通文本输出普通 span
- 强调文本输出强调 span
- 行内代码输出 code pill
- 代码块输出 block 容器

v1 允许富文本渲染的字段：
- `title`
- `subtitle`
- `body`
- `quote`
- `note`
- `tip`
- `cta`
- `items[*]`
- `steps[*]`
- `compare.leftLabel`
- `compare.rightLabel`
- `compare.rows[*].left`
- `compare.rows[*].right`

其中只有 `body` / `note` / `tip` 允许 block-level 代码块；其余字段即使出现 fenced code block，也必须在解析阶段统一降级为普通文本，不进入块级渲染路径。

边界仍然保持：
- 不为 `tech-noir` 单独写一套模板分支
- 模板能力是共享的
- `tech-noir` 只是样式表达最强的消费者

### 4. 样式层：增加语义 token，而不是继续堆基础 token

`internal/render/theme_vars.go` 需要新增一组语义变量，例如：
- `--emphasis-color`
- `--number-color`
- `--inline-code-bg`
- `--inline-code-border`
- `--inline-code-color`
- `--code-block-bg`
- `--code-block-border`
- `--code-block-color`

`internal/render/assets/base.css` 则新增共享语义类：
- `.text-em`
- `.inline-code`
- `.code-block`
- `.metric-value`

### 5. 主题策略：强表达只放在 tech-noir

#### tech-noir
- `**bold**`：金色或高亮强调色
- inline code：更深底 + 金色边 / 浅色前景
- code block：深卡片底 + 清晰描边
- 数字 / 百分比 / 序号：单独强调色

#### 其他主题
- 默认继承共享结构，但保持弱化表达
- 例如：
  - emphasis 只提升字重或轻微变色
  - inline code 只做浅底胶囊
  - code block 只做统一中性卡片
- 避免把当前默认主题系全部带偏成“高对比科技风”

## 页面适配策略

### 立即受益的页型

1. `cover`
- 标题中的 `**重点词**` 可变成金色
- CTA 中的行动词可更突出

2. `quote`
- 观点句中的重点词可被拉出来
- 行内命令/API 名可胶囊化

3. `image-caption`
- body 内的加粗与行内代码能带来层次

4. `bullets`
- bullet index 可独立强调
- item 内部如果包含 `**重点**` 或 `inline code`，也可区分

5. `compare`
- 右侧结果值 / 数字 / 百分比更有表达力

6. `gallery-steps`
- step number 与关键数字可以更强

7. `ending`
- 标题与正文结论词可做更明显收束

### 代码块落位策略

当前页型并没有单独的“代码页”variant，因此 v1 不需要新增页型。

建议优先支持两种情况：
- 代码块出现在 `body` / `note` / `tip` 一类可承载长文案的字段中
- 若某些页型不适合展示多行代码块，则该字段仍允许渲染成统一代码卡片，并交由版式规则决定高度截断或收敛

重要边界：本轮设计不要求所有 variant 都能优雅承载超长代码；目标是支持常见短代码片段与小块命令示例。

## 回归与测试策略

### 单元测试

需要新增字段级语义解析测试，覆盖：
- 普通文本
- `**bold**`
- `` `inline code` ``
- fenced code block
- 非法/不闭合语法的降级行为
- 混合文本中的多个片段
- 结构化数字强调与显式语义数字强调的边界

### 渲染测试

需要新增 HTML 快照或模板输出断言，确认：
- emphasis 输出了正确语义 class
- inline code 输出了独立容器
- code block 输出了统一块级容器
- compare / steps / bullets 的数字强调节点存在

### 视觉回归

至少补这些 fixture 场景：
- cover：标题含 `**重点词**`
- quote：引用含 `**强调**` 与 `` `命令` ``
- image-caption / ending：正文含行内代码
- 至少一个包含 fenced code block 的正文场景
- bullets / gallery-steps：序号与数字强调
- compare：右侧结果值、百分比或关键数字强调

目标是验证 `tech-noir` 不再是“整页一片深色”，而是出现明显但克制的局部语义层次。

## 风险与取舍

### 风险 1：AI 输出时丢失 Markdown 语义

如果 AI 在生成 deck JSON 时把 `**bold**` 直接改写成纯文本，后处理就失去语义来源。

**应对：**
- 在 prompt 中加强“保留 Markdown 语义”的要求
- 为典型输入写回归测试
- 若 AI 仍不稳定，再评估对某些字段做更明确的结构化输出要求

### 风险 2：现有字段全是 string，扩展后波及面较大

如果直接把 `PageContent` 的若干字段从 `string` 改成复杂结构，会牵动 deck 校验、模板、fixture、回归数据。

**应对：**
- v1 优先保留原始 string 字段
- 在渲染前补充派生语义片段，而不是立即重塑整个 deck JSON schema
- 让 JSON 兼容面尽量保持稳定

### 风险 3：代码块可能冲击现有页型版式

某些 variant 对大块多行文本并不友好。

**应对：**
- v1 把代码块能力限定为“小块可读代码卡片”
- 不承诺超长代码展示
- 通过 fixture 控制样例长度

## 最终建议

本轮 `tech-noir` 调整不应再停留在“换更好的黑金 token”，而应该补上一个**共享的最小语义渲染层**：

- 用 Markdown 原生语义承载作者意图
- 用字段级轻解析恢复强调 / 代码 / 代码块语义
- 用共享模板与共享 CSS 输出语义节点
- 用 `tech-noir` 的主题变量把这些语义节点拉开层次

最终实现出的效果应当是：
- 标题里只有重点词被提亮
- 命令/参数/模型名有自己的视觉身份
- 结构化数字、序号、百分比得到单独强调
- 代码卡片形成结构节奏
- 同一份 Markdown 仍然可直接复用于公众号发布链路
