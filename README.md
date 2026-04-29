# mark2note

A Markdown-to-deck CLI for generating HTML / PNG presentation assets, with optional animated export, Apple Live Photo packaging, and Xiaohongshu publishing helpers.

[中文文档](./README.zh-CN.md) | [English Documentation](./README.en.md)

## Features

- Convert Markdown into AI-generated deck JSON, then render HTML and capture PNG
- Import generated PNG files into Apple Photos with `--import-photos`
- Support deck themes: `default`, `warm-paper`, `editorial-cool`, `tech-noir`, `plum-ink`, `sage-mist`, and `fresh-green`
- Optionally export Animated WebP or MP4 per page
- Optionally build experimental Live package outputs and assemble Apple Live Photos
- Capture existing HTML files into sibling PNG files via `capture-html`
- Publish standard image posts or Live-photo assets to Xiaohongshu via `publish-xhs`

## Theme Notes

- Supported stable themes: `default`, `warm-paper`, `editorial-cool`, `tech-noir`, `plum-ink`, `sage-mist`, `fresh-green`
- `deck.theme_mode: weekly` chooses a fixed theme from `deck.weekly_themes` by local weekday; `deck.theme` remains the fallback
- `--theme` overrides fixed or weekly selection for one run
- Unknown theme names fall back to `default`

## Output Notes

- HTML + PNG remain the primary stable outputs
- Use `--import-photos` to import the generated top-level PNG files into Apple Photos; add `--import-album <name>` to choose the album. The same defaults can be set with `render.import_photos`, `render.import_album`, and `render.import_timeout` in config.
- Animated output can be enabled through `render.animated.enabled`; experimental Live output can be enabled through `render.live.enabled`
- Animated WebP export requires `img2webp`; MP4 or Live export requires `ffmpeg`; Live packaging also requires `exiftool`
- `capture-html` does not export Animated WebP, MP4, or Live packages.

## Quick Start

```bash
cp configs/config.example.yaml configs/config.yaml
go build -o ./mark2note ./cmd/mark2note
./mark2note --help
./mark2note --input ./article.md
./mark2note --input ./article.md --import-photos --import-album "mark2note"
./mark2note publish-xhs --help
```

### Regenerate from saved layout

Every successful render writes `deck.json` and `render-meta.json` into the output directory. To regenerate HTML/PNG from the saved layout without rereading Markdown or rerunning AI layout:

```bash
./mark2note --from-deck ./output/preview/deck.json
```

The rerender path still supports post-render flows:

```bash
./mark2note --from-deck ./output/preview/deck.json --import-photos --import-album "mark2note"
./mark2note --from-deck ./output/preview/deck.json --live --live-assemble --live-import-photos
```

When `--out` is omitted, the new output directory is based on the old deck directory name plus a timestamp. If a sibling `render-meta.json` exists, it restores theme, viewport, author, and watermark. `--prompt-extra` is only valid with `--input`, not `--from-deck`.

## License

See [LICENSE](./LICENSE).
