# Contributing

Thanks for helping improve `mark2note`.

## Development environment

Install Go using the version declared in `go.mod` and make sure the repository builds locally before opening a pull request.

## Local configuration

Use committed templates for non-sensitive examples. Keep real local configuration, secrets, generated notes, logs, and private assets out of git.

## Build and run

```bash
bash ./build.sh
./mark2note --help
```

## Tests and local CI

Run the full local check before submitting a pull request:

```bash
bash ./scripts/ci-local.sh
```

The local CI covers secret scanning, formatting, vetting, tests, and build checks.

## Secret scanning

Run the scanner directly when changing configuration, examples, workflows, or release packaging:

```bash
bash ./scripts/secret-scan.sh --current --history
```

Do not commit `.env`, local config files, API keys, tokens, passwords, private documents, generated private notes, logs, or local artifacts.

## Pull requests

Keep pull requests focused. Include what changed, why it changed, and the verification commands you ran.

## Commit messages

Use Conventional Commits, for example `fix: 修复解析错误` or `docs: 更新安装说明`.

## Releases

Maintainers publish releases by creating version tags with `scripts/tag-release.sh`. Do not publish release tags from pull request branches.
