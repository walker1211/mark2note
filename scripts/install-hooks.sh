#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)
hook_path="$repo_root/.git/hooks/pre-push"

if [[ -e "$hook_path" ]] && ! grep -q 'scripts/ci-local.sh' "$hook_path"; then
  printf 'pre-push hook already exists and was not created by this project: %s\n' "$hook_path" >&2
  exit 1
fi

cat > "$hook_path" <<'HOOK'
#!/usr/bin/env bash
set -euo pipefail

if [[ "${SKIP_CI_LOCAL_ON_PRE_PUSH:-}" == "1" ]]; then
  exit 0
fi

repo_root=$(git rev-parse --show-toplevel)
"$repo_root/scripts/ci-local.sh" clean
HOOK

chmod +x "$hook_path"
printf 'installed pre-push hook: %s\n' "$hook_path"
