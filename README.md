# mark2note

[中文](./README.zh-CN.md) | [English](./README.en.md)

`mark2note` converts Markdown into presentation assets through the flow: Markdown -> AI deck JSON -> HTML / PNG, with optional animated, Live Photo, Apple Photos, and Xiaohongshu publishing workflows.

## Install

Download the archive for your platform from [GitHub Releases](https://github.com/walker1211/mark2note/releases), for example:

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

```bash
cp configs/config.example.yaml configs/config.yaml
./mark2note --input ./article.md
```

Update `configs/config.yaml` for your AI CLI, output directory, theme, author, and watermark before running real workloads.

## Output notes

HTML + PNG remain the primary stable outputs. Optional animation features can export Animated WebP or MP4, or experimental Live packages when `render.live.enabled` is enabled. Animated WebP export needs `img2webp`; MP4 and Live export need `ffmpeg`; Live package export also needs `exiftool`. The `capture-html` command does not export Animated WebP, MP4, or Live packages.

## Documentation

See [中文](./README.zh-CN.md) or [English](./README.en.md) for detailed usage and configuration.

## License

See [LICENSE](./LICENSE).
