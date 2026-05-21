#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s [owner/repo]\n' "$0" >&2
}

if [[ $# -gt 1 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 2
fi

if ! command -v gh >/dev/null 2>&1; then
  printf 'missing required command: gh\n' >&2
  exit 127
fi

repo="${1:-}"
if [[ -z "$repo" ]]; then
  repo="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
fi

if [[ "$repo" != */* ]]; then
  printf 'repository must use owner/repo format: %s\n' "$repo" >&2
  exit 2
fi

failed=0

status_line() {
  local label="$1"
  local value="$2"
  printf '%-38s %s\n' "$label:" "$value"
}

require_enabled() {
  local label="$1"
  local value="$2"
  if [[ "$value" == "enabled" || "$value" == "true" ]]; then
    status_line "$label" "$value"
    return
  fi
  status_line "$label" "$value"
  failed=1
}

default_branch="$(gh api "repos/${repo}" --jq '.default_branch')"
secret_scanning="$(gh api "repos/${repo}" --jq '.security_and_analysis.secret_scanning.status // "unavailable"')"
push_protection="$(gh api "repos/${repo}" --jq '.security_and_analysis.secret_scanning_push_protection.status // "unavailable"')"
private_vulnerability_reporting="$(gh api "repos/${repo}/private-vulnerability-reporting" --jq '.enabled')"

if required_checks="$(gh api "repos/${repo}/branches/${default_branch}/protection" --jq '.required_status_checks.contexts // [] | join(", ")' 2>/dev/null)"; then
  branch_protection="enabled"
else
  branch_protection="unavailable or disabled"
  required_checks=""
fi

code_scanning_tools="$(gh api "repos/${repo}/code-scanning/analyses" --jq '.[].tool.name' 2>/dev/null | sort -u || true)"
if [[ -n "$code_scanning_tools" ]]; then
  code_scanning="enabled"
else
  code_scanning="unavailable or no analyses"
  failed=1
fi

status_line "Repository" "$repo"
status_line "Default branch" "$default_branch"
require_enabled "Secret scanning" "$secret_scanning"
require_enabled "Push protection" "$push_protection"
require_enabled "Private vulnerability reporting" "$private_vulnerability_reporting"
require_enabled "Branch protection" "$branch_protection"
if [[ -n "$required_checks" ]]; then
  status_line "Required status checks" "$required_checks"
else
  status_line "Required status checks" "none reported"
  failed=1
fi
status_line "Code scanning" "$code_scanning"
if [[ -n "$code_scanning_tools" ]]; then
  status_line "Code scanning tools" "$code_scanning_tools"
fi

exit "$failed"
