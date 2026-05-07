# mark2note

[中文](./README.zh-CN.md) | [English](./README.en.md)

## What it does

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

## Requirements

- Go 1.25+
- An AI CLI that can be invoked with `-p <prompt>`
- Google Chrome, or a compatible browser binary that supports `--headless=new`
- `img2webp` (required only when Animated WebP enhancement export is enabled)
- `ffmpeg` (required when MP4 or Live export is enabled)
- `exiftool` (required only when experimental Live package export is enabled)

## Install

Prefer downloading the archive for your platform from [GitHub Releases](https://github.com/walker1211/mark2note/releases), for example:

```bash
tar -xzf mark2note_<tag>_<os>_<arch>.tar.gz
./mark2note --help
```

You can also build from source when developing locally:

```bash
bash ./build.sh
./mark2note --help
```

## Quick start

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

## Configuration

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

## AI CLI examples

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

## Common commands

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

## Development / testing

```bash
go test ./...
go build ./cmd/mark2note
```

To inspect the CLI help:

```bash
./mark2note --help
```

## License

See [LICENSE](./LICENSE).
