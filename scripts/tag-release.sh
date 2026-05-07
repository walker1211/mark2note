#!/usr/bin/env bash
set -euo pipefail

version=${1:-}
if [[ -z "$version" ]]; then
  printf 'usage: %s vX.Y.Z\n' "$0" >&2
  exit 2
fi

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]]; then
  printf 'invalid version tag: %s\n' "$version" >&2
  exit 2
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)
cd "$repo_root"

if [[ -n "$(git status --porcelain)" ]]; then
  printf 'working tree has uncommitted changes; commit or stash them before tagging\n' >&2
  exit 1
fi

"$repo_root/scripts/ci-local.sh" clean

git tag "$version"
SKIP_CI_LOCAL_ON_PRE_PUSH=1 git push origin "$version"
