# mark2note

[Landing Page](./README.md) | [中文文档](./README.zh-CN.md)

`mark2note` converts Markdown into presentation assets through the flow: Markdown -> AI deck JSON -> HTML / PNG.

It calls the AI CLI configured in your config file to generate deck JSON, renders HTML, and captures PNG images from the rendered pages. The default primary output remains stable HTML + PNG. When `--animated` or `render.animated.enabled` is enabled, it additionally tries to export one Animated WebP or MP4 per page as an enhancement output. For Xiaohongshu workflows, MP4 is currently the more practical intermediate format before converting to Live Photo. The experimental `--live` / `render.live.enabled` mode additionally tries to build one Live package directory per page and is mainly intended for macOS / iPhone import workflows.

The `capture-html` command is also a public CLI subcommand that converts existing HTML files, or HTML files inside a directory, into sibling PNG files. In directory mode, it only scans the current directory, does not recurse into subdirectories, only processes lowercase `.html` files, and writes PNG output next to the source HTML files. It does not export Animated WebP, MP4, or Live packages.

## Features

- Markdown -> AI deck JSON -> HTML / PNG
- Optional Animated WebP / MP4 export
- Optional experimental Live package export and Apple Live Photo assembly
- Convert existing HTML files into sibling PNG files
- Publish standard image posts or Live-photo assets to Xiaohongshu through `publish-xhs`
- Publish to Xiaohongshu automatically after the main PNG render with `--publish-xhs`

## Requirements

- Go 1.25+
- An AI CLI that can be invoked with `-p <prompt>`
- Google Chrome, or a compatible browser binary that supports `--headless=new`
- `img2webp` (required only when Animated WebP export is enabled)
- `ffmpeg` (required when MP4 or Live export is enabled)
- `exiftool` (required when Live package export is enabled)
- `makelive` (required only when final Apple Live Photo assembly is enabled)

## Quick Start

### 1. Initialize the config

```bash
cp configs/config.example.yaml configs/config.yaml
```

### 2. Configure the AI CLI and defaults

The default config path is `configs/config.yaml`. Update it as needed for your AI CLI, output directory, theme, author, and watermark.

### 3. Build the binary

```bash
go build -o ./mark2note ./cmd/mark2note
```

### 4. Generate presentation assets

Prepare a Markdown file first, for example `./article.md`, then run:

```bash
./mark2note --input ./article.md
```

Notes:

- The repository root `./config.yaml` is not the default path; the default path is always `configs/config.yaml`
- If you still need the legacy root-level config behavior, pass it explicitly with `--config ./config.yaml`
- You can also use `--config` to point to any other config file

## Regenerate from a saved layout

Successful renders write `deck.json` and `render-meta.json` to the output directory. Use `--from-deck` to regenerate HTML/PNG from that saved layout without rereading Markdown or rerunning AI layout:

```bash
./mark2note --from-deck ./output/preview/deck.json
```

The rerender path reuses the same post-render flows:

```bash
./mark2note --from-deck ./output/preview/deck.json --import-photos --import-album "mark2note"
./mark2note --from-deck ./output/preview/deck.json --live --live-assemble --live-import-photos
```

Without `--out`, mark2note creates a new timestamped directory from the original deck directory name. If a sibling `render-meta.json` exists, it restores theme, viewport, author, watermark, and saved `shuffle-light` page colors. `--prompt-extra` is only valid with `--input`, not `--from-deck`.

## Theme Notes

- `deck.theme` and `--theme` now support `shuffle-light`
- `shuffle-light` rerandomizes page palette assignment on each run
- It only reuses these six existing non-`tech-noir` palettes: `default-orange`, `default-green`, `warm-paper`, `editorial-cool`, `lifestyle-light`, `editorial-mono`
- Adjacent pages never repeat the same palette, and `tech-noir` is excluded from the pool

## Configuration

Default config file: `configs/config.yaml`

Key fields:

- `output.dir`: default output root directory
- `ai.command` / `ai.args`: AI CLI command and arguments used to generate deck JSON
- `deck.theme`: default theme, supporting `default`, `shuffle-light`, `warm-paper`, `editorial-cool`, `lifestyle-light`, `tech-noir`, and `editorial-mono`
- `deck.author`: default cover author value
- `deck.watermark.enabled`: enables the page watermark, on by default
- `deck.watermark.text`: watermark text, defaulting to `walker1211/mark2note`
- `deck.watermark.position`: watermark position, supporting `bottom-right` and `bottom-left`
- `render.viewport.width`: export width, default `1242`
- `render.viewport.height`: export height, default `1656`
- `render.animated.enabled`: whether to additionally export one animated asset per page, off by default
- `render.animated.format`: animated format, supporting `webp` and `mp4`
- `render.animated.duration_ms`: total page animation timeline duration, default `2400`; when Live export is enabled it also affects `motion.mov`
- `render.animated.fps`: animation capture density, default `8`; it affects Animated WebP / MP4 output and also Live frame sampling density
- `render.import_photos`: whether to import generated PNG files into Apple Photos, off by default
- `render.import_album`: Apple Photos album name for PNG import; when empty, `mark2note-photos-<timestamp>` is generated
- `render.import_timeout`: PNG import timeout, default `2m`; supports Go duration strings such as `45s` and `2m`
- `render.live.enabled`: whether to additionally export one experimental Live package per page, off by default
- `render.live.photo_format`: Live still photo format, currently emitted as `jpeg`
- `render.live.cover_frame`: Live cover frame strategy, supporting `first`, `middle`, and `last`
- `render.live.assemble`: whether to call `makelive` after `.live/` packages are generated, off by default
- `render.live.output_dir`: final Apple Live Photo output directory; when empty it defaults to `<out>/apple-live/`
- `render.live.import_photos`: whether to import assembled Live Photos into Apple Photos, off by default; requires `render.live.assemble`
- `render.live.import_album`: Apple Photos album name for Live import; when empty, `mark2note-live-<timestamp>` is generated
- `render.live.import_timeout`: Live import timeout, default `2m`; supports Go duration strings such as `45s` and `2m`
- `xhs.publish.account`: default account for `publish-xhs` and `--publish-xhs`
- `xhs.publish.headless`: default browser headless mode for `publish-xhs` and `--publish-xhs`
- `xhs.publish.browser_path`: default browser executable path for `publish-xhs` and `--publish-xhs`; CLI `--chrome` can override it for one run
- `xhs.publish.profile_dir`: default browser profile directory for `publish-xhs` and `--publish-xhs`
- `xhs.publish.mode`: default publish mode for `publish-xhs` and `--publish-xhs`, supporting `only-self` and `schedule`
- `xhs.publish.chrome_args`: extra Chrome launch arguments used only by Xiaohongshu publishing

Additional notes:

- The example value for `deck.author` can be changed; leave it blank if you do not want the author to be shown
- The example author value is not a built-in program default
- Watermark is enabled by default and rendered directly in HTML, so PNG files captured from HTML include the same watermark
- `deck.watermark.position` only supports `bottom-right` and `bottom-left`
- `render.viewport.width` / `render.viewport.height` affect both the HTML viewport and the PNG / animated capture size; when omitted they fall back to `1242 x 1656`
- If you want to test a Xiaohongshu-friendly portrait pipeline, try `720 x 960` or `1080 x 1440` first
- HTML + PNG remain the primary stable outputs; Animated WebP, MP4, and Live are optional enhancement outputs
- CLI import flags override config defaults when explicitly passed, including `--import-photos=false` and `--live-import-photos=false`
- Animated WebP export requires `img2webp`; MP4 export requires `ffmpeg`; if either tool is missing, the command keeps successful HTML + PNG output and prints a warning
- `render.animated.duration_ms` / `--animated-duration` is not only for `.webp/.mp4`; when Live export is enabled it also affects the overall `motion.mov` timeline
- `render.animated.fps` / `--animated-fps` is most intuitive for Animated WebP / MP4; in Live mode it means capture density rather than a fixed final playback FPS for `motion.mov`
- Live package export requires `ffmpeg + exiftool`; the current implementation is mainly intended for macOS / iPhone import workflows, and writes `cover.jpg`, `motion.mov`, and `manifest.json` in each page package directory
- When `render.live.assemble` or `--live-assemble` is also enabled, `makelive` must be available; if it is missing, the command only prints a warning and still keeps HTML + PNG and `.live/` package output
- `<page>.live/` is an intermediate package, not the final directly importable asset; after assembly, final outputs are written to `<out>/apple-live/` or the directory specified by `render.live.output_dir` / `--live-output-dir`
- When only Live export is enabled, the renderer still captures frames on the animation timeline, but it does not emit root-level `.webp` or `.mp4` files
- `capture-html` still only converts existing HTML into PNG files; if you want it to reuse configured export dimensions, pass `--config` so it can read `render.viewport.width/height`
- `--theme` supports one-off theme overrides
- `--author` supports one-off author overrides
- `--config` can explicitly select another config file
- `--prompt-extra` appends one-off natural-language guidance for the Markdown -> deck JSON stage, such as pacing, tone, or structure
- `--prompt-extra` only affects deck generation; it does not directly change HTML rendering, PNG capture, Animated / Live export, or `publish-xhs` behavior
- `--publish-xhs` publishes to Xiaohongshu after the main render flow successfully generates standard PNG files; the title comes from the Markdown H1 and the body contains only 3-6 parsed topic hashtags
- `--xhs-tags` manually overrides parsed topics, for example `--xhs-tags "AI agent,data safety,engineering reflection"`; it is valid only with `--publish-xhs`
- When `xhs.publish.chrome_args` is omitted, Xiaohongshu publishing uses `disable-background-networking`, `disable-component-update`, `no-first-run`, and `no-default-browser-check`; set `chrome_args: []` to launch without extra args for debugging
- `xhs.publish.chrome_args` entries may include or omit the leading `--`, and `name=value` arguments are supported

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
./mark2note --input ./article.md --theme shuffle-light
./mark2note --input ./article.md --theme tech-noir
./mark2note --input ./article.md --prompt-extra "make the cover more attention-grabbing and frame it like an experience recap"
./mark2note --input ./article.md --theme shuffle-light --prompt-extra "make the output concise but keep image pages" --live=false --publish-xhs
./mark2note --input ./article.md --theme shuffle-light --publish-xhs --xhs-tags "AI agent,data safety,engineering reflection"
./mark2note --input ./article.md --animated --animated-format webp --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --animated --animated-format mp4 --animated-duration 2400 --animated-fps 8
./mark2note --input ./article.md --import-photos --import-album "mark2note"
./mark2note --input ./article.md --live --live-cover-frame middle
./mark2note --input ./article.md --live --live-assemble --live-import-photos --live-import-album "mark2note-live"
./mark2note --input ./article.md --live --live-assemble --live-output-dir ./output/apple-live
./mark2note capture-html --input ./output/preview/p02-quote.html
./mark2note capture-html --input ./output/preview
./mark2note publish-xhs --account main --title "Title" --content "Body" --images ./cover.jpg
./mark2note publish-xhs --account main --mode schedule --schedule-at "2026-04-18 20:30:00" --title-file ./title.txt --content-file ./body.md --images ./cover.jpg,./detail.jpg
./mark2note publish-xhs --account main --title-file ./title.txt --content-file ./body.md --live-report ./output/report.json --live-pages p01-cover,p02-bullets
```

Note: in directory mode, `capture-html` only scans the current directory, does not recurse into subdirectories, only processes lowercase `.html`, and writes PNG output next to the source HTML files.

## Xiaohongshu publishing

### Auto-publish after render with `--publish-xhs`

The main render command can automatically invoke the Xiaohongshu publish flow after standard PNG files are generated successfully. This flow reuses `xhs.publish` defaults for account, browser path, browser profile, headless mode, publish mode, originality declaration, and content-copy preference. It publishes the standard PNG pages from the current render, not Live Photo artifacts.

```bash
./mark2note \
  --input ./article.md \
  --theme shuffle-light \
  --prompt-extra "make the output concise but keep image pages" \
  --live=false \
  --publish-xhs
```

Auto-publish behavior:

- the title comes from the first Markdown H1; when absent, it falls back to the cleaned input filename
- the body contains only topic hashtags, for example `#AI代理 #数据安全 #工程反思`
- topics are parsed locally from frontmatter `tags`, Markdown hashtags, title terms, level-two/three headings, and frequent body terms; automatic results target 3-6 topics
- use `--xhs-tags` to manually override parsed topics; manual values skip automatic extraction and are still rendered as body hashtags:

```bash
./mark2note \
  --input ./article.md \
  --theme shuffle-light \
  --publish-xhs \
  --xhs-tags "AI agent,data safety,engineering reflection"
```

Rules:

- `--publish-xhs` is supported only with `--input`, not `--from-deck`
- `--xhs-tags` is accepted only with `--publish-xhs`
- render failures skip publishing
- if no generated standard PNG is found, or a listed PNG path is missing, the command prints the render summary first and then returns an error

### Standalone publish subcommand `publish-xhs`

`publish-xhs` publishes generated image assets or Live Photo outputs to Xiaohongshu Creator Center.

Current behavior:

- standard publish mode currently submits as only-self-visible
- schedule mode still submits as scheduled publish
- media source is exclusive: use `--images` for standard image posts or `--live-report` for the Live pipeline

### Usage

```bash
mark2note publish-xhs --account <name> [flags]
mark2note publish-xhs --help
```

### Flags

- `--config <file>`: config file path, default `configs/config.yaml`
- `--account <name>`: publish account / profile target; when omitted it falls back to `xhs.publish.account`
- `--title <text>`: inline title text; exactly one of `--title` / `--title-file` is required
- `--title-file <file>`: title file path
- `--content <text>`: inline content text; exactly one of `--content` / `--content-file` is required
- `--content-file <file>`: content file path
- `--tags <csv>`: comma-separated tags
- `--mode <name>`: publish mode, supporting `only-self` and `schedule`; when omitted it falls back to `xhs.publish.mode`, otherwise defaults to `only-self`
- `--schedule-at <time>`: scheduled publish time in `YYYY-MM-DD HH:MM:SS`, parsed in Asia/Shanghai; required when `--mode schedule`
- `--images <csv>`: comma-separated image paths for standard image posts
- `--live-report <file>`: Live delivery report path used to resolve publishable Live assets
- `--live-pages <csv>`: ordered Live page subset; valid only with `--live-report`
- `--chrome <path>`: Chrome binary path; when omitted it first falls back to `xhs.publish.browser_path`
- `--headless`: run browser automation headless; the built-in default is `true`, but `xhs.publish.headless` can override it
- `--profile-dir <dir>`: browser profile directory; when omitted it first falls back to `xhs.publish.profile_dir`

### What `--profile-dir` does

`publish-xhs` uses a dedicated browser user-data directory to keep the Xiaohongshu Creator Center login state, cookies, and session data. `--profile-dir` points to that browser profile directory.

Its precedence is:

1. CLI `--profile-dir`
2. config `xhs.publish.profile_dir`
3. if both are absent, after the final `account` is resolved it automatically falls back to `os.UserConfigDir()/mark2note/xhs/profiles/<account>`

Why it matters:

- after you log in once by QR scan, reusing the same profile usually avoids repeated login
- different accounts can use different persistent browser profiles
- account-specific session problems are easier to isolate and debug

Notes:

- `--profile-dir` and `xhs.publish.profile_dir` now support `~` expansion, so `~/.config/...` works directly
- the automatic fallback path still depends on the OS; only explicit `~/.config/...` configuration forces that directory style
- if you explicitly point multiple accounts to the same `profile_dir`, they will share the same browser session directory, which is bad for isolation and debugging

Example config:

```yaml
xhs:
  publish:
    account: walker
    headless: false
    browser_path: /Applications/Google Chrome.app/Contents/MacOS/Google Chrome
    profile_dir: ~/.config/mark2note/xhs/profiles/walker
    mode: only-self
```

Example command:

```bash
./mark2note publish-xhs \
  --account walker \
  --profile-dir ~/.config/mark2note/xhs/profiles/walker \
  --title "Published this card set today" \
  --content "Post body" \
  --images ./output/cover.jpg,./output/detail.jpg
```

### Validation rules

- exactly one of `--title` / `--title-file` is required
- exactly one of `--content` / `--content-file` is required
- exactly one media source is required: `--images` or `--live-report`
- `--schedule-at` is required when `--mode schedule`
- `--live-pages` is accepted only with `--live-report`

### Standard image publish

```bash
./mark2note publish-xhs \
  --account main \
  --title "Title" \
  --content "Body" \
  --tags "efficiency,AI" \
  --images ./cover.jpg,./detail.jpg
```

### Scheduled publish

```bash
./mark2note publish-xhs \
  --account main \
  --mode schedule \
  --schedule-at "2026-04-18 20:30:00" \
  --title-file ./title.txt \
  --content-file ./body.md \
  --images ./cover.jpg
```

### Live Photo publish

```bash
./mark2note publish-xhs \
  --account main \
  --title-file ./title.txt \
  --content-file ./body.md \
  --live-report ./output/report.json \
  --live-pages p01-cover,p02-bullets
```

### Login and session notes

If the command prints a message similar to “not logged in to Xiaohongshu creator center”, the current browser profile does not have a valid login session yet. Open the browser with the same profile directory, complete the QR login to Xiaohongshu Creator Center, then retry the publish command.

For first-time login or expired sessions, it is best to disable `--headless`, or set `xhs.publish.headless: false` in config, complete the QR login once, and then reuse the same profile directory.

## Development / testing

```bash
go test ./...
go build -o ./mark2note ./cmd/mark2note
```

To inspect CLI help:

```bash
./mark2note --help
./mark2note publish-xhs --help
```

## License

See [LICENSE](./LICENSE).
