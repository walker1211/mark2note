# mark2note

[中文](#中文) | [English](#english)

## 中文

### 功能说明

`mark2note` 用于把 Markdown 内容转换为演示稿资源，流程为：Markdown -> AI deck JSON -> HTML / PNG。

它会先调用配置文件中指定的 AI CLI，把 Markdown 解析为 deck JSON，再渲染为 HTML，并通过截图生成 PNG。

另外，`capture-html` 是公开的 CLI 子命令能力，可将已有 HTML 文件或目录中的 HTML 直接转换为同级
PNG。目录模式下，它只扫描当前目录，不递归子目录，只处理小写 `.html` 文件，且 PNG 输出在 HTML 同级目录。

### 环境要求

- Go 1.25+
- 可通过 `-p <prompt>` 调用的 AI CLI
- Google Chrome，或兼容 `--headless=new` 的浏览器可执行文件

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
- `deck.theme`：默认主题，支持 `default`、`warm-paper`、`editorial-cool`、`lifestyle-light`
- `deck.author`：封面作者的默认值
- `deck.watermark.enabled`：是否启用页内水印，默认启用
- `deck.watermark.text`：水印文本，默认值为 `walker1211/mark2note`
- `deck.watermark.position`：水印位置，支持 `bottom-right`、`bottom-left`

补充说明：

- `deck.author` 的示例值可以自行修改；留空则不显示作者。
- 示例配置里的作者值只是示例，不代表程序内置默认作者。
- 水印默认启用，渲染在 HTML 内，因此通过 `capture-html` 生成的 PNG 也会包含同一水印。
- `deck.watermark.position` 仅支持 `bottom-right`、`bottom-left`。
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
PNG images from the rendered pages.

The `capture-html` command is also a public CLI subcommand that converts existing HTML files, or HTML files inside a
directory, into sibling PNG files. In directory mode, it only scans the current directory, does not recurse into
subdirectories, only processes lowercase `.html` files, and writes PNG output next to the source HTML files.

### Requirements

- Go 1.25+
- An AI CLI that can be invoked with `-p <prompt>`
- Google Chrome, or a compatible browser binary that supports `--headless=new`

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
- `deck.theme`: default theme, supporting `default`, `warm-paper`, `editorial-cool`, and `lifestyle-light`
- `deck.author`: default cover author value
- `deck.watermark.enabled`: enables the page watermark, on by default
- `deck.watermark.text`: watermark text, defaulting to `walker1211/mark2note`
- `deck.watermark.position`: watermark position, supporting `bottom-right` and `bottom-left`

Additional notes:

- The example value for `deck.author` is only an example and can be changed.
- Leave `deck.author` blank if you do not want the author to be shown.
- The example author value is not a built-in program default.
- Watermark is enabled by default and rendered directly in HTML, so PNG files captured from HTML include the same watermark.
- `deck.watermark.position` only supports `bottom-right` and `bottom-left`.
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
