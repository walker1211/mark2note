#!/usr/bin/env bash
set -euo pipefail

mode=${1:-clean}
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)

run_secret_scan() {
  local current_root=$1

  printf '==> secret scan\n'
  "$repo_root/scripts/secret-scan.sh" --current --current-root "$current_root" --history
}

run_checks() {
  local dir=$1
  local full_tests=${2:-0}

  printf '==> gofmt\n'
  unformatted=$(find "$dir" -name '*.go' -type f -print0 | xargs -0 gofmt -l)
  if [[ -n "$unformatted" ]]; then
    printf '%s\n' "$unformatted"
    exit 1
  fi

  printf '==> go vet\n'
  go -C "$dir" vet ./...

  if [[ "$full_tests" == "1" ]]; then
    printf '==> go test (full)\n'
    TZ=UTC MARK2NOTE_FULL_TESTS=1 go -C "$dir" test ./...
  else
    printf '==> go test\n'
    TZ=UTC go -C "$dir" test ./...
  fi

  printf '==> build\n'
  (cd "$dir" && bash ./build.sh)
}

case "$mode" in
  clean)
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    git -C "$repo_root" ls-files -z | tar -C "$repo_root" --null -T - -cf - | tar -x -C "$tmpdir"
    run_secret_scan "$tmpdir"
    run_checks "$tmpdir"
    ;;
  full)
    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT
    git -C "$repo_root" ls-files -z | tar -C "$repo_root" --null -T - -cf - | tar -x -C "$tmpdir"
    run_secret_scan "$tmpdir"
    run_checks "$tmpdir" 1
    ;;
  worktree)
    run_secret_scan "$repo_root"
    run_checks "$repo_root"
    ;;
  *)
    printf 'usage: %s [clean|full|worktree]\n' "$0" >&2
    exit 2
    ;;
esac
