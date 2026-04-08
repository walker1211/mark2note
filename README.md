# mark2note

[中文](#中文) | [English](#english)

## 中文

### 功能说明

`mark2note` 用于把 Markdown 内容转换为演示稿资源，流程为：Markdown -> AI deck JSON -> HTML / PNG。

它会先调用配置文件中指定的 AI CLI，把 Markdown 解析为 deck JSON，再渲染为 HTML，并通过截图生成 PNG。默认主输出仍然是稳定的 HTML + PNG；显式开启 `--animated` 或 `render.animated.enabled` 时，会为每页额外尝试导出 Animated WebP 或 MP4 作为增强产物。对小红书链路，当前更推荐把 MP4 作为中间产物，再继续转 Live Photo；实验性的 `--live` / `render.live.enabled` 会额外尝试生成每页一个 Live package 目录，当前主要面向 macOS/iPhone 导入链路。

另外，`capture-html` 是公开的 CLI 子命令能力，可将已有 HTML 文件或目录中的 HTML 直接转换为同级
PNG。目录模式下，它只扫描当前目录，不递归子目录，只处理小写 `.html` 文件，且 PNG 输出在 HTML 同级目录；该子命令当前不会导出 Animated WebP、MP4 或 Live package。

### 环境要求

- Go 1.25+
- 可通过 `-p <prompt>` 调用的 AI CLI
- Google Chrome，或兼容 `--headless=new` 的浏览器可执行文件
- `img2webp`（仅在启用 Animated WebP 增强导出时需要）
- `ffmpeg`（启用 MP4 或 Live 导出时需要）
- `exiftool`（启用 Live package 导出时需要）
- `makelive`（仅在启用最终 Apple Live Photo 组装时需要）

### 快速开始

1. 准备配置文件。默认配置路径是 `configs/config.yaml`。
2. 复制示例配置：

```bash
cp configs/config.example.yaml configs/config.yaml
```

3. 按需修改 `configs/config.yaml` 中的 AI CLI、输出目录、主题、作者和水印。
4. 先在仓库根目录构建二进制：

```bash
go build -o ./mark2note ./cmd/mark2note
```

5. 先准备一个你自己的 Markdown 文件，例如 `./article.md`，然后运行：

```bash
./mark2note --input ./article.md
```

说明：

- 默认不会读取根目录 `./config.yaml`；默认路径统一为 `configs/config.yaml`。
- 如果你确实需要继续使用根目录旧配置，可显式传入：`--config ./config.yaml`。
- 也可以通过 `--config` 指定任意其他配置文件。

### 配置

默认配置文件：`configs/config.yaml`

关键字段：

- `output.dir`：默认输出根目录
- `ai.command` / `ai.args`：用于生成 deck JSON 的 AI CLI 命令及参数
- `deck.theme`：默认主题，支持 `default`、`warm-paper`、`editorial-cool`、`lifestyle-light`、`tech-noir`、`editorial-mono`
- `deck.author`：封面作者的默认值
- `deck.watermark.enabled`：是否启用页内水印，默认启用
- `deck.watermark.text`：水印文本，默认值为 `walker1211/mark2note`
- `deck.watermark.position`：水印位置，支持 `bottom-right`、`bottom-left`
- `render.viewport.width`：导出宽度，默认 `1242`
- `render.viewport.height`：导出高度，默认 `1656`
- `render.animated.enabled`：是否额外导出每页动图产物，默认关闭
- `render.animated.format`：动图格式，支持 `webp`、`mp4`
- `render.animated.duration_ms`：每页动画时间轴总时长，默认 `2400`；启用 Live 时也会影响 `motion.mov` 的时长
- `render.animated.fps`：动画抓帧密度，默认 `8`；对 Animated WebP/MP4 有用，在 Live 下也会影响帧采样密度
- `render.live.enabled`：是否额外导出每页实验性 Live package，默认关闭
- `render.live.photo_format`：Live 静态封面格式，当前固定写出 `jpeg`
- `render.live.cover_frame`：Live 封面帧选择策略，支持 `first`、`middle`、`last`
- `render.live.assemble`：是否在 `.live/` 中间包生成后继续调用 `makelive` 组装最终 Apple Live Photo，默认关闭
- `render.live.output_dir`：最终 Apple Live Photo 输出目录；留空时默认写到 `<out>/apple-live/`

补充说明：

- `deck.author` 的示例值可以自行修改；留空则不显示作者。
- 示例配置里的作者值只是示例，不代表程序内置默认作者。
- 水印默认启用，渲染在 HTML 内，因此通过 `capture-html` 生成的 PNG 也会包含同一水印。
- `deck.watermark.position` 仅支持 `bottom-right`、`bottom-left`。
- `render.viewport.width` / `render.viewport.height` 会同时作用于 HTML 视口和 PNG / 动图截图尺寸；未配置时回落到默认 `1242 x 1656`。
- 如果你要测试小红书竖屏链路，可先尝试 `720 x 960` 或 `1080 x 1440` 这类 `3:4` 尺寸。
- HTML + PNG 仍然是默认且更稳妥的主输出；Animated WebP、MP4、Live 都属于可选增强输出。
- 启用 Animated WebP 导出时需要系统可用的 `img2webp`；启用 MP4 导出时需要 `ffmpeg`；若缺失，命令会保留 HTML + PNG 成功结果，并输出 warning。
- `render.animated.duration_ms` / `--animated-duration` 不只作用于 `.webp/.mp4`，启用 Live 时也会影响 `motion.mov` 的时间轴总时长。
- `render.animated.fps` / `--animated-fps` 对 Animated WebP/MP4 最直观；在 Live 下它表示抓帧密度，而不是最终 `motion.mov` 的固定播放帧率。
- 启用 Live package 导出时需要 `ffmpeg + exiftool`；当前实现主要面向 macOS/iPhone 导入链路，产物目录内会包含 `cover.jpg`、`motion.mov` 与 `manifest.json`，仍属于实验性能力。
- 若同时启用 `render.live.assemble` 或 `--live-assemble`，还需要系统可用的 `makelive`；缺失时只会输出 warning，不影响 HTML + PNG 和 `.live/` 中间包生成。
- `<page>.live/` 是中间包，不是最终可直接导入的单文件；启用组装后，最终成品会输出到 `<out>/apple-live/` 或 `render.live.output_dir` / `--live-output-dir` 指定目录。
- 仅启用 Live 导出时，程序仍会按动画时间轴捕获帧，但不会额外生成根级 `.webp` / `.mp4` 文件。
- `capture-html` 当前仍然只负责把现有 HTML 转成 PNG，不导出 Animated WebP、MP4 或 Live package；如需复用配置里的导出尺寸，可额外传入 `--config` 读取 `render.viewport.width/height`。
- `--theme` 支持单次覆盖配置中的主题。
- `--author` 支持单次覆盖配置中的作者。
- `--config` 可显式指定其他配置文件。

### AI CLI 示例

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

### 常用命令

```bash
./mark2note --help
./mark2note --input ./article.md
./mark2note --input ./article.md --out ./output/preview
./mark2note --input ./article.md --config ./configs/config.yaml
./mark2note --input ./article.md --config ./config.yaml
./mark2note --input ./article.md --theme warm-paper --author "Your Name"
./mark2note --input ./article.md --theme tech-noir
./mark2note --input ./article.md --animated --animated-format webp --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --animated --animated-format mp4 --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --live --live-cover-frame middle
./mark2note --input ./article.md --live --live-assemble --live-output-dir ./output/apple-live
./mark2note capture-html --input ./output/preview/p02-quote.html
./mark2note capture-html --input ./output/preview
```

说明：`capture-html` 的目录模式只扫描当前目录，不递归子目录，只处理小写 `.html`，PNG 输出在 HTML 同级目录。

### 开发 / 测试

```bash
go test ./...
go build ./cmd/mark2note
```

如果需要查看命令说明：

```bash
./mark2note --help
```

### License

See [LICENSE](./LICENSE).

---

## English

### What it does

`mark2note` converts Markdown into presentation assets through the following flow: Markdown -> AI deck JSON -> HTML /
PNG.

It first calls the AI CLI configured in the config file to turn Markdown into deck JSON, then renders HTML and captures
PNG images from the rendered pages. The default primary output remains stable HTML + PNG; when `--animated` or
`render.animated.enabled` is set, it additionally attempts to export one Animated WebP or MP4 per page as an enhancement
output. For Xiaohongshu workflows, MP4 is currently the more practical intermediate format before converting to Live
Photo; the experimental `--live` / `render.live.enabled` mode additionally tries to build one Live package directory per
page and is mainly intended for macOS/iPhone import workflows.

The `capture-html` command is also a public CLI subcommand that converts existing HTML files, or HTML files inside a
directory, into sibling PNG files. In directory mode, it only scans the current directory, does not recurse into
subdirectories, only processes lowercase `.html` files, and writes PNG output next to the source HTML files. It does not
export Animated WebP, MP4, or Live packages.

### Requirements

- Go 1.25+
- An AI CLI that can be invoked with `-p <prompt>`
- Google Chrome, or a compatible browser binary that supports `--headless=new`
- `img2webp` (required only when Animated WebP enhancement export is enabled)
- `ffmpeg` (required when MP4 or Live export is enabled)
- `exiftool` (required only when experimental Live package export is enabled)

### Quick start

1. Prepare the config file. The default config path is `configs/config.yaml`.
2. Copy the example config:

```bash
cp configs/config.example.yaml configs/config.yaml
```

3. Update `configs/config.yaml` as needed for your AI CLI, output directory, theme, author, and watermark.
4. Build the binary from the repository root first:

```bash
go build -o ./mark2note ./cmd/mark2note
```

5. Prepare your own Markdown file first, for example `./article.md`, then run:

```bash
./mark2note --input ./article.md
```

Notes:

- The repository root `./config.yaml` is not the default path; the default path is always `configs/config.yaml`.
- If you need the legacy root-level config behavior, use it explicitly with `--config ./config.yaml`.
- You can also use `--config` to point to any other config file.

### Configuration

Default config file: `configs/config.yaml`

Key fields:

- `output.dir`: default output root directory
- `ai.command` / `ai.args`: AI CLI command and arguments used to generate deck JSON
- `deck.theme`: default theme, supporting `default`, `warm-paper`, `editorial-cool`, `lifestyle-light`, `tech-noir`, and `editorial-mono`
- `deck.author`: default cover author value
- `deck.watermark.enabled`: enables the page watermark, on by default
- `deck.watermark.text`: watermark text, defaulting to `walker1211/mark2note`
- `deck.watermark.position`: watermark position, supporting `bottom-right` and `bottom-left`
- `render.viewport.width`: export width, default `1242`
- `render.viewport.height`: export height, default `1656`
- `render.animated.enabled`: whether to additionally export one animated asset per page, off by default
- `render.animated.format`: animated format, supporting `webp` and `mp4`
- `render.animated.duration_ms`: total page animation timeline duration, default `2400`; when Live export is enabled it also affects `motion.mov` duration
- `render.animated.fps`: animation capture density, default `8`; it affects Animated WebP/MP4 output and also Live frame sampling density
- `render.live.enabled`: whether to additionally export one experimental Live package per page, off by default
- `render.live.photo_format`: Live still photo format, currently emitted as `jpeg`
- `render.live.cover_frame`: Live cover frame strategy, supporting `first`, `middle`, and `last`

Additional notes:

- The example value for `deck.author` is only an example and can be changed.
- Leave `deck.author` blank if you do not want the author to be shown.
- The example author value is not a built-in program default.
- Watermark is enabled by default and rendered directly in HTML, so PNG files captured from HTML include the same watermark.
- `deck.watermark.position` only supports `bottom-right` and `bottom-left`.
- `render.viewport.width` / `render.viewport.height` affect both the HTML viewport and the PNG / animated capture size; when omitted they fall back to the default `1242 x 1656`.
- If you want to test a Xiaohongshu-friendly portrait pipeline, try `720 x 960` or `1080 x 1440` first.
- HTML + PNG remain the primary stable outputs; Animated WebP, MP4, and Live are optional enhancement outputs.
- Animated WebP export requires `img2webp`; MP4 export requires `ffmpeg`; if either tool is missing, the command keeps successful HTML + PNG output and prints a warning.
- `render.animated.duration_ms` / `--animated-duration` is not only for `.webp/.mp4`; when Live export is enabled it also affects the overall `motion.mov` timeline.
- `render.animated.fps` / `--animated-fps` is most intuitive for Animated WebP/MP4; in Live mode it means capture density rather than a fixed final playback FPS for `motion.mov`.
- Live package export requires `ffmpeg + exiftool`; the current implementation is experimental, mainly intended for macOS/iPhone import workflows, and writes `cover.jpg`, `motion.mov`, and `manifest.json` in each page package directory.
- When only Live export is enabled, the renderer still captures animation frames on the internal timeline, but it does not emit root-level `.webp` or `.mp4` files.
- `capture-html` still only converts existing HTML into PNG files; if you want it to reuse configured export dimensions, pass `--config` so it can read `render.viewport.width/height`.
- `--theme` supports one-off theme overrides.
- `--author` supports one-off author overrides.
- `--config` can explicitly select another config file.

### AI CLI examples

Using `ccs`:

```yaml
ai:
  command: ccs
  args:
    - codex
    - --bare
```

Using `claude`:

```yaml
ai:
  command: claude
  args:
    - -p
```

Note: adjust the arguments to match your local AI CLI setup, as long as `mark2note` can use it to generate deck JSON.

### Common commands

```bash
./mark2note --help
./mark2note --input ./article.md
./mark2note --input ./article.md --out ./output/preview
./mark2note --input ./article.md --config ./configs/config.yaml
./mark2note --input ./article.md --config ./config.yaml
./mark2note --input ./article.md --theme warm-paper --author "Your Name"
./mark2note --input ./article.md --animated --animated-format webp --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --animated --animated-format mp4 --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --live --live-cover-frame middle
./mark2note --input ./article.md --live --live-assemble --live-output-dir ./output/apple-live
./mark2note capture-html --input ./output/preview/p02-quote.html
./mark2note capture-html --input ./output/preview
```

Note: in directory mode, `capture-html` only scans the current directory, does not recurse into subdirectories, only
processes lowercase `.html` files, and writes PNG output next to the source HTML files.

### Development / testing

```bash
go test ./...
go build ./cmd/mark2note
```

To inspect the CLI help:

```bash
./mark2note --help
```

### License

See [LICENSE](./LICENSE).
