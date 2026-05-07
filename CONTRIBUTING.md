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

Run the fast local check before pushing or creating a tag:

```bash
bash ./scripts/ci-local.sh clean
```

This is the same default path installed by `scripts/install-hooks.sh` for pre-push checks. It covers secret scanning, formatting, vetting, fast tests, and build checks.

Run the full local check before requesting review when touching browser automation or Xiaohongshu publishing behavior:

```bash
bash ./scripts/ci-local.sh full
```

Full mode sets `MARK2NOTE_FULL_TESTS=1` and includes slow Rod/Chrome browser tests. GitHub CI also runs the full test mode.

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
