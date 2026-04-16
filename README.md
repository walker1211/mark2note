# mark2note

A Markdown-to-deck CLI for generating HTML / PNG presentation assets, with optional animated export, Apple Live Photo packaging, and Xiaohongshu publishing helpers.

[中文文档](./README.zh-CN.md) | [English Documentation](./README.en.md)

## Features

- Convert Markdown into AI-generated deck JSON, then render HTML and capture PNG
- Optionally export Animated WebP or MP4 per page
- Optionally build experimental Live package outputs and assemble Apple Live Photos
- Capture existing HTML files into sibling PNG files via `capture-html`
- Publish standard image posts or Live-photo assets to Xiaohongshu via `publish-xhs`

## Output Notes

- HTML + PNG remain the primary stable outputs
- Animated output can be enabled through `render.animated.enabled`; experimental Live output can be enabled through `render.live.enabled`
- Animated WebP export requires `img2webp`; MP4 or Live export requires `ffmpeg`; Live packaging also requires `exiftool`
- `capture-html` does not export Animated WebP, MP4, or Live packages.

## Quick Start

```bash
cp configs/config.example.yaml configs/config.yaml
go build -o ./mark2note ./cmd/mark2note
./mark2note --help
./mark2note --input ./article.md
./mark2note publish-xhs --help
```

## License

See [LICENSE](./LICENSE).
