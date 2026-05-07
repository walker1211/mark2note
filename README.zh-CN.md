# mark2note

[入口页](./README.md) | [English](./README.en.md)

`mark2note` 用于把 Markdown 内容转换为演示稿资源，主流程为：Markdown -> AI deck JSON -> HTML / PNG。

它会先调用配置文件中指定的 AI CLI，把 Markdown 解析为 deck JSON，再渲染为 HTML，并通过截图生成 PNG。默认主输出仍然是稳定的 HTML + PNG；显式开启 `--animated` 或 `render.animated.enabled` 时，会为每页额外尝试导出 Animated WebP 或 MP4 作为增强产物。对小红书链路，当前更推荐把 MP4 作为中间产物，再继续转 Live Photo；实验性的 `--live` / `render.live.enabled` 会额外尝试生成每页一个 Live package 目录，主要面向 macOS / iPhone 导入链路。

另外，`capture-html` 是公开的 CLI 子命令能力，可将已有 HTML 文件或目录中的 HTML 直接转换为同级 PNG。目录模式下，它只扫描当前目录，不递归子目录，只处理小写 `.html` 文件，且 PNG 输出在 HTML 同级目录；该子命令当前不会导出 Animated WebP、MP4 或 Live package。

## 功能概览

- Markdown -> AI deck JSON -> HTML / PNG
- 可选导出 Animated WebP / MP4
- 可选导出实验性的 Live package，并进一步组装 Apple Live Photo
- 支持把已有 HTML 直接截图为 PNG
- 支持 `publish-xhs` 发布普通图片内容或 Live Photo 产物到小红书
- 支持主渲染命令 `--publish-xhs` 在生成 PNG 后自动发布到小红书

## 环境要求

- Go 1.25+
- 可通过 `-p <prompt>` 调用的 AI CLI
- Google Chrome，或兼容 `--headless=new` 的浏览器可执行文件
- `img2webp`（仅在启用 Animated WebP 增强导出时需要）
- `ffmpeg`（启用 MP4 或 Live 导出时需要）
- `exiftool`（启用 Live package 导出时需要）
- `makelive`（仅在启用最终 Apple Live Photo 组装时需要）

## 快速开始

### 1. 初始化配置

```bash
cp configs/config.example.yaml configs/config.yaml
```

### 2. 配置 AI CLI 与默认输出

默认配置路径是 `configs/config.yaml`。按需修改其中的 AI CLI、输出目录、主题、作者和水印。

### 3. 构建二进制

```bash
go build -o ./mark2note ./cmd/mark2note
```

### 4. 生成演示稿资源

先准备一个 Markdown 文件，例如 `./article.md`，然后运行：

```bash
./mark2note --input ./article.md
```

说明：

- 默认不会读取根目录 `./config.yaml`；默认路径统一为 `configs/config.yaml`
- 如果你确实需要继续使用根目录旧配置，可显式传入：`--config ./config.yaml`
- 也可以通过 `--config` 指定任意其他配置文件

## 从已保存布局重新生成

每次成功渲染都会在输出目录写入 `deck.json` 和 `render-meta.json`。如果只想从已保存布局重新生成 HTML/PNG，不重新读取 Markdown，也不重新调用 AI 布局：

```bash
./mark2note --from-deck ./output/preview/deck.json
```

这个流程仍然支持后续导入相册和 Live 输出：

```bash
./mark2note --from-deck ./output/preview/deck.json --import-photos --import-album "mark2note"
./mark2note --from-deck ./output/preview/deck.json --live --live-assemble --live-import-photos
```

不传 `--out` 时，会按原 deck 所在目录名加时间戳生成新的输出目录。如果同目录存在 `render-meta.json`，会恢复旧运行的主题、视口、作者、水印和 `shuffle-light` 页面配色。`--prompt-extra` 只适用于 `--input`，不能和 `--from-deck` 一起使用。

## 主题说明

- `deck.theme` 和 `--theme` 现在都支持 `shuffle-light`
- `shuffle-light` 每次生成都会重新随机分配页面配色
- 它只会复用这 6 套现有非 `tech-noir` 配色：`default-orange`、`default-green`、`warm-paper`、`editorial-cool`、`lifestyle-light`、`editorial-mono`
- 相邻页面不会重复同一套配色，且不会使用 `tech-noir`

## 配置

默认配置文件：`configs/config.yaml`

关键字段：

- `output.dir`：默认输出根目录
- `ai.command` / `ai.args`：用于生成 deck JSON 的 AI CLI 命令及参数
- `deck.theme`：默认主题，支持 `default`、`shuffle-light`、`warm-paper`、`editorial-cool`、`lifestyle-light`、`tech-noir`、`editorial-mono`
- `deck.author`：封面作者默认值
- `deck.watermark.enabled`：是否启用页内水印，默认启用
- `deck.watermark.text`：水印文本，默认值为 `walker1211/mark2note`
- `deck.watermark.position`：水印位置，支持 `bottom-right`、`bottom-left`
- `render.viewport.width`：导出宽度，默认 `1242`
- `render.viewport.height`：导出高度，默认 `1656`
- `render.animated.enabled`：是否额外导出每页动图产物，默认关闭
- `render.animated.format`：动图格式，支持 `webp`、`mp4`
- `render.animated.duration_ms`：每页动画时间轴总时长，默认 `2400`；启用 Live 时也会影响 `motion.mov` 的时长
- `render.animated.fps`：动画抓帧密度，默认 `8`；对 Animated WebP / MP4 有用，在 Live 下也会影响帧采样密度
- `render.import_photos`：是否把生成的普通 PNG 导入 Apple Photos，默认关闭
- `render.import_album`：普通 PNG 导入相册名；留空时自动生成 `mark2note-photos-<timestamp>`
- `render.import_timeout`：普通 PNG 导入超时，默认 `2m`；支持 `45s`、`2m` 这类 Go duration 字符串
- `render.live.enabled`：是否额外导出每页实验性 Live package，默认关闭
- `render.live.photo_format`：Live 静态封面格式，当前固定写出 `jpeg`
- `render.live.cover_frame`：Live 封面帧选择策略，支持 `first`、`middle`、`last`
- `render.live.assemble`：是否在 `.live/` 中间包生成后继续调用 `makelive` 组装最终 Apple Live Photo，默认关闭
- `render.live.output_dir`：最终 Apple Live Photo 输出目录；留空时默认写到 `<out>/apple-live/`
- `render.live.import_photos`：是否把组装后的 Live Photos 导入 Apple Photos，默认关闭；需要同时启用 `render.live.assemble`
- `render.live.import_album`：Live 导入相册名；留空时自动生成 `mark2note-live-<timestamp>`
- `render.live.import_timeout`：Live 导入超时，默认 `2m`；支持 `45s`、`2m` 这类 Go duration 字符串
- `xhs.publish.account`：`publish-xhs` 和 `--publish-xhs` 默认发布账号
- `xhs.publish.headless`：`publish-xhs` 和 `--publish-xhs` 默认是否无头运行浏览器
- `xhs.publish.browser_path`：`publish-xhs` 和 `--publish-xhs` 默认浏览器可执行文件路径；命令行 `--chrome` 可单次覆盖
- `xhs.publish.profile_dir`：`publish-xhs` 和 `--publish-xhs` 默认浏览器 profile 目录
- `xhs.publish.mode`：`publish-xhs` 和 `--publish-xhs` 默认发布模式，支持 `only-self`、`schedule`
- `xhs.publish.topic_generation.enabled`：`--publish-xhs` 未传 `--xhs-tags` 时是否调用 AI 生成 3-6 个小红书话题，默认开启
- `xhs.publish.title_generation.enabled`：`--publish-xhs` 标题超过 `max_runes` 时是否调用 AI 改写标题，默认开启
- `xhs.publish.title_generation.max_runes`：自动发布标题长度上限，默认 `20`；按 Unicode 字符计数，中文、英文、数字、空格和标点通常都算 1 个字符
- `xhs.publish.chrome_args`：小红书发布浏览器使用的额外 Chrome 启动参数

补充说明：

- `deck.author` 的示例值可以自行修改；留空则不显示作者
- 示例配置里的作者值只是示例，不代表程序内置默认作者
- 水印默认启用，渲染在 HTML 内，因此通过 `capture-html` 生成的 PNG 也会包含同一水印
- `deck.watermark.position` 仅支持 `bottom-right`、`bottom-left`
- `render.viewport.width` / `render.viewport.height` 会同时作用于 HTML 视口和 PNG / 动图截图尺寸；未配置时回落到默认 `1242 x 1656`
- 如果你要测试小红书竖屏链路，可先尝试 `720 x 960` 或 `1080 x 1440` 这类 `3:4` 尺寸
- HTML + PNG 仍然是默认且更稳妥的主输出；Animated WebP、MP4、Live 都属于可选增强输出
- 显式传入命令行导入参数时会覆盖配置默认值，包括 `--import-photos=false` 和 `--live-import-photos=false`
- 启用 Animated WebP 导出时需要系统可用的 `img2webp`；启用 MP4 导出时需要 `ffmpeg`；若缺失，命令会保留 HTML + PNG 成功结果，并输出 warning
- `render.animated.duration_ms` / `--animated-duration` 不只作用于 `.webp/.mp4`，启用 Live 时也会影响 `motion.mov` 的时间轴总时长
- `render.animated.fps` / `--animated-fps` 对 Animated WebP / MP4 最直观；在 Live 下它表示抓帧密度，而不是最终 `motion.mov` 的固定播放帧率
- 启用 Live package 导出时需要 `ffmpeg + exiftool`；当前实现主要面向 macOS / iPhone 导入链路，产物目录内会包含 `cover.jpg`、`motion.mov` 与 `manifest.json`
- 若同时启用 `render.live.assemble` 或 `--live-assemble`，还需要系统可用的 `makelive`；缺失时只会输出 warning，不影响 HTML + PNG 和 `.live/` 中间包生成
- `<page>.live/` 是中间包，不是最终可直接导入的单文件；启用组装后，最终成品会输出到 `<out>/apple-live/` 或 `render.live.output_dir` / `--live-output-dir` 指定目录
- 仅启用 Live 导出时，程序仍会按动画时间轴捕获帧，但不会额外生成根级 `.webp` / `.mp4` 文件
- `capture-html` 当前仍然只负责把现有 HTML 转成 PNG，不导出 Animated WebP、MP4 或 Live package；如需复用配置里的导出尺寸，可额外传入 `--config` 读取 `render.viewport.width/height`
- `--theme` 支持单次覆盖配置中的主题
- `--author` 支持单次覆盖配置中的作者
- `--config` 可显式指定其他配置文件
- `--prompt-extra` 支持单次追加自然语言引导，用来控制 Markdown -> deck JSON 阶段的分页、标题语气和内容组织方向
- `--prompt-extra` 只影响 deck 生成，不直接改变 HTML 渲染、PNG 截图、Animated / Live 导出或 `publish-xhs` 发布逻辑
- `--publish-xhs` 会在主渲染流程成功生成普通 PNG 后发布到小红书；标题来自 Markdown 一级标题，小红书正文只包含 3-6 个话题
- 自动发布标题超过 `xhs.publish.title_generation.max_runes` 时，会按 `xhs.publish.title_generation.enabled` 调用同一套 `ai.command` / `ai.args` 改写标题；代码只校验长度，不再本地截断
- 未传 `--xhs-tags` 时，`--publish-xhs` 会按 `xhs.publish.topic_generation.enabled` 调用同一套 `ai.command` / `ai.args` 生成话题；AI 调用失败、JSON 不合法或没有有效话题时会跳过发布并报错，不再回退到本地规则推理
- `--xhs-tags` 可手动覆盖 AI 话题，例如 `--xhs-tags "AI代理,数据安全,工程反思"`；它只能和 `--publish-xhs` 一起使用，且传入后不会调用 AI 生成话题
- `xhs.publish.chrome_args` 不配置时，小红书发布默认使用 `disable-background-networking`、`disable-component-update`、`no-first-run`、`no-default-browser-check`；调试时可写 `chrome_args: []` 表示不加额外参数
- `xhs.publish.chrome_args` 每项可以带或不带开头的 `--`，也支持 `name=value` 形式

## AI CLI 示例

使用 `ccs`：

```yaml
ai:
  command: ccs
  args:
    - codex
    - --bare
```

使用 `claude`：

```yaml
ai:
  command: claude
  args:
    - -p
```

说明：请根据你本地 AI CLI 的实际调用方式调整参数，只要能被 `mark2note` 用来生成 deck JSON 即可。

## 常用命令

```bash
./mark2note --help
./mark2note --input ./article.md
./mark2note --input ./article.md --out ./output/preview
./mark2note --input ./article.md --config ./configs/config.yaml
./mark2note --input ./article.md --config ./config.yaml
./mark2note --input ./article.md --theme warm-paper --author "Your Name"
./mark2note --input ./article.md --theme shuffle-light
./mark2note --input ./article.md --theme tech-noir
./mark2note --input ./article.md --prompt-extra "封面更抓眼，整体更像经验复盘"
./mark2note --input ./article.md --theme shuffle-light --prompt-extra "精简输出，但不要精简掉图片" --live=false --publish-xhs
./mark2note --input ./article.md --theme shuffle-light --publish-xhs --xhs-tags "AI代理,数据安全,工程反思"
./mark2note --input ./article.md --animated --animated-format webp --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --animated --animated-format mp4 --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --import-photos --import-album "mark2note"
./mark2note --input ./article.md --live --live-cover-frame middle
./mark2note --input ./article.md --live --live-assemble --live-import-photos --live-import-album "mark2note-live"
./mark2note --input ./article.md --live --live-assemble --live-output-dir ./output/apple-live
./mark2note capture-html --input ./output/preview/p02-quote.html
./mark2note capture-html --input ./output/preview
./mark2note publish-xhs --account main --title "标题" --content "正文" --images ./cover.jpg
./mark2note publish-xhs --account main --mode schedule --schedule-at "2026-04-18 20:30:00" --title-file ./title.txt --content-file ./body.md --images ./cover.jpg,./detail.jpg
./mark2note publish-xhs --account main --title-file ./title.txt --content-file ./body.md --live-report ./output/report.json --live-pages p01-cover,p02-bullets
```

说明：`capture-html` 的目录模式只扫描当前目录，不递归子目录，只处理小写 `.html`，PNG 输出在 HTML 同级目录。

## 小红书发布

### 渲染后自动发布 `--publish-xhs`

主渲染命令可以在成功生成普通 PNG 后自动调用小红书发布流程。这个流程复用 `xhs.publish` 的账号、浏览器路径、浏览器 profile、无头模式、发布模式、原创声明和正文复制配置；发布素材使用本次渲染出的普通 PNG，不使用 Live Photo 产物。

```bash
./mark2note \
  --input ~/mark/2026/26.04.26-一个AI代理删库之后我开始关心刹车.md \
  --theme shuffle-light \
  --prompt-extra "精简输出，但不要精简掉图片" \
  --live=false \
  --publish-xhs
```

自动发布时：

- 标题来自 Markdown 第一个一级标题；没有一级标题时回退到清理后的文件名
- 标题不超过 `xhs.publish.title_generation.max_runes` 时直接使用；超过时才调用 AI 改写，返回标题仍超长或为空会报错停止发布
- 正文只包含话题，例如 `#AI代理 #数据安全 #工程反思`
- 话题由 AI 生成，自动结果会尽量保持 3-6 个
- 如果要手动指定话题，可以加 `--xhs-tags`，手动值会跳过 AI 话题生成并仍用于正文 hashtag：

```bash
./mark2note \
  --input ./article.md \
  --theme shuffle-light \
  --publish-xhs \
  --xhs-tags "AI代理,数据安全,工程反思"
```

约束：

- `--publish-xhs` 只支持 `--input` 流程，不能和 `--from-deck` 一起使用
- `--xhs-tags` 只能和 `--publish-xhs` 一起使用
- 如果渲染失败，不会尝试发布
- 如果没有找到本次生成的普通 PNG，或 PNG 路径不存在，会在打印渲染摘要后返回错误

### 独立发布子命令 `publish-xhs`

`publish-xhs` 用于把已生成好的图片资源或 Live Photo 产物发布到小红书创作中心。

当前规则：

- 普通发布模式当前统一走「仅自己可见」发布
- 定时模式仍走定时发布
- 媒体来源二选一：普通图片用 `--images`，Live 链路用 `--live-report`

### 用法

```bash
mark2note publish-xhs --account <name> [flags]
mark2note publish-xhs --help
```

### 参数说明

- `--config <file>`：配置文件路径，默认 `configs/config.yaml`
- `--account <name>`：发布账号 / profile 名称；未显式传入时，回退到 `xhs.publish.account`
- `--title <text>`：直接传标题文本；与 `--title-file` 二选一且必填
- `--title-file <file>`：从文件读取标题
- `--content <text>`：直接传正文；与 `--content-file` 二选一且必填
- `--content-file <file>`：从文件读取正文
- `--tags <csv>`：逗号分隔标签列表
- `--mode <name>`：发布模式，支持 `only-self`、`schedule`；未显式传入时，回退到 `xhs.publish.mode`，否则默认 `only-self`
- `--schedule-at <time>`：定时发布时间，格式 `YYYY-MM-DD HH:MM:SS`，按 Asia/Shanghai 解析；仅 `--mode schedule` 时必填
- `--images <csv>`：逗号分隔图片路径列表，用于普通图文发布
- `--live-report <file>`：Live 交付报告路径，用于从 Live 导出结果里提取可发布素材
- `--live-pages <csv>`：只发布指定顺序的 Live 页面子集；必须和 `--live-report` 一起使用
- `--chrome <path>`：Chrome 可执行文件路径；未显式传入时先回退到 `xhs.publish.browser_path`
- `--headless`：是否无头运行浏览器；默认值是 `true`，但可被 `xhs.publish.headless` 覆盖
- `--profile-dir <dir>`：浏览器 profile 目录；未显式传入时先回退到 `xhs.publish.profile_dir`

### `--profile-dir` 是做什么的

`publish-xhs` 会使用独立的浏览器用户目录来保存小红书创作中心登录态、Cookie 和会话数据。`--profile-dir` 就是这个浏览器 profile 的目录位置。

它的优先级是：

1. 命令行 `--profile-dir`
2. 配置文件 `xhs.publish.profile_dir`
3. 若两者都没设置，则在最终解析出 `account` 后，自动回退到 `os.UserConfigDir()/mark2note/xhs/profiles/<account>`

为什么建议配它：

- 首次扫码登录后，后续复用同一个 profile 通常不需要反复登录
- 多账号时可以给不同 `account` 使用不同 profile 目录
- 出问题时也更容易单独排查某个账号的浏览器会话

注意：

- `--profile-dir` 和 `xhs.publish.profile_dir` 现在支持 `~` 自动展开，可直接写 `~/.config/...`
- 自动回退路径仍取决于操作系统；只有在你显式配置 `~/.config/...` 时，才会固定落到这套目录风格
- 如果你显式给多个账号配置了同一个 `profile_dir`，这些账号就会共用同一个浏览器会话目录，不利于隔离与排障

示例配置：

```yaml
xhs:
  publish:
    account: walker
    headless: false
    browser_path: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
    profile_dir: ~/.config/mark2note/xhs/profiles/walker
    mode: only-self
```

示例命令：

```bash
./mark2note publish-xhs \
  --account walker \
  --profile-dir ~/.config/mark2note/xhs/profiles/walker \
  --title "今天这套卡片发了" \
  --content "正文内容" \
  --images ./output/cover.jpg,./output/detail.jpg
```

### 约束规则

- 必须且只能提供其一：`--title` / `--title-file`
- 必须且只能提供其一：`--content` / `--content-file`
- 媒体来源必须二选一：`--images` 或 `--live-report`
- `--mode schedule` 时必须提供 `--schedule-at`
- `--live-pages` 只能与 `--live-report` 一起使用

### 普通图片发布

```bash
./mark2note publish-xhs \
  --account main \
  --title "标题" \
  --content "正文" \
  --tags "效率,AI" \
  --images ./cover.jpg,./detail.jpg
```

### 定时发布

```bash
./mark2note publish-xhs \
  --account main \
  --mode schedule \
  --schedule-at "2026-04-18 20:30:00" \
  --title-file ./title.txt \
  --content-file ./body.md \
  --images ./cover.jpg
```

### Live Photo 发布

```bash
./mark2note publish-xhs \
  --account main \
  --title-file ./title.txt \
  --content-file ./body.md \
  --live-report ./output/report.json \
  --live-pages p01-cover,p02-bullets
```

### 登录与会话说明

如果命令输出类似“not logged in to Xiaohongshu creator center”，表示当前 profile 还没有可用登录态。此时应使用同一个 profile 目录打开浏览器并完成小红书创作中心扫码登录，然后再重试发布。

首次登录或登录态失效时，建议先关闭 `--headless`，或在配置里把 `xhs.publish.headless` 设为 `false`，完成扫码后再恢复无头运行。

## 开发 / 测试

```bash
go test ./...
go build -o ./mark2note ./cmd/mark2note
```

如果需要查看命令说明：

```bash
./mark2note --help
./mark2note publish-xhs --help
```

## License

See [LICENSE](./LICENSE).
