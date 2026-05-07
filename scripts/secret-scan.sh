#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s [--current] [--history] [--current-root DIR]\n' "$0" >&2
}

scan_current=0
scan_history=0
current_root=

if [[ $# -eq 0 ]]; then
  scan_current=1
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --current)
      scan_current=1
      ;;
    --history)
      scan_history=1
      ;;
    --current-root)
      if [[ $# -lt 2 || -z "$2" ]]; then
        usage
        exit 2
      fi
      current_root=$2
      shift
      ;;
    --help|-h|help)
      usage
      exit 0
      ;;
    *)
      usage
      exit 2
      ;;
  esac
  shift
done

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$script_dir/.." && pwd)
if [[ -z "$current_root" ]]; then
  current_root=$repo_root
fi

python3 - "$repo_root" "$current_root" "$scan_current" "$scan_history" <<'PY'
from __future__ import annotations

import os
import re
import subprocess
import sys
from dataclasses import dataclass
from functools import lru_cache
from typing import Iterable

repo_root = sys.argv[1]
current_root = os.path.abspath(sys.argv[2])
scan_current = sys.argv[3] == "1"
scan_history = sys.argv[4] == "1"

PROTECTED_TRACKED_PATHS = {".env", "configs/config.yaml"}
SKIP_PATHS = {"go.sum"}
MAX_BLOB_BYTES = 5 * 1024 * 1024

SECRET_NAME_RE = re.compile(
    r"(?i)\b[A-Z0-9_]*(?:API_KEY|TOKEN|SECRET|PASSWORD|AUTH_CODE)\b\s*[:=]\s*([\"']?)([^\"'\s,#)}\]]{4,})"
)
TOKEN_PATTERNS = [
    ("openai-token", re.compile(r"\bsk-(?:proj-)?[A-Za-z0-9_-]{20,}\b")),
    ("anthropic-token", re.compile(r"\bsk-ant-[A-Za-z0-9_-]{20,}\b")),
    ("github-token", re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{30,})\b")),
]
PEM_PRIVATE_KEY_RE = re.compile(r"-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----")

PLACEHOLDER_VALUES = {
    "secret",
    "test-secret",
    "your-email-auth-code",
    "your-secret",
    "your-token",
    "your-api-key",
    "example",
    "example-secret",
    "example-token",
    "placeholder",
    "changeme",
    "change-me",
    "dummy",
    "dummy-secret",
    "github.token",
}

@dataclass(frozen=True)
class Finding:
    scope: str
    path: str
    line: int
    reason: str
    commit: str | None = None
    blob: str | None = None


def git(args: list[str], *, input_data: bytes | None = None, check: bool = True) -> subprocess.CompletedProcess[bytes]:
    return subprocess.run(
        ["git", "-C", repo_root, *args],
        input=input_data,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=check,
    )


def git_text(args: list[str], *, check: bool = True) -> str:
    return git(args, check=check).stdout.decode("utf-8", errors="replace")


def iter_tracked_paths() -> list[str]:
    data = git(["ls-files", "-z"]).stdout
    return [p.decode("utf-8", errors="surrogateescape") for p in data.split(b"\0") if p]


def is_text(data: bytes) -> bool:
    if b"\0" in data:
        return False
    return True


def is_skipped_path(path: str) -> bool:
    return path in SKIP_PATHS or path.endswith("/go.sum")


def allowed_secret_value(value: str) -> bool:
    normalized = value.strip().strip("'\"").strip()
    lowered = normalized.lower()
    if not normalized:
        return True
    if lowered in PLACEHOLDER_VALUES:
        return True
    if "${{ github.token }}" in normalized or lowered == "github.token":
        return True
    return False


def line_findings(scope: str, path: str, line_no: int, line: str, *, commit: str | None = None, blob: str | None = None) -> Iterable[Finding]:
    if PEM_PRIVATE_KEY_RE.search(line):
        yield Finding(scope, path, line_no, "pem-private-key", commit=commit, blob=blob)

    for reason, pattern in TOKEN_PATTERNS:
        if pattern.search(line):
            yield Finding(scope, path, line_no, reason, commit=commit, blob=blob)

    for match in SECRET_NAME_RE.finditer(line):
        value = match.group(2)
        if not allowed_secret_value(value):
            yield Finding(scope, path, line_no, "secret-assignment", commit=commit, blob=blob)


def scan_text(scope: str, path: str, data: bytes, *, commit: str | None = None, blob: str | None = None) -> list[Finding]:
    if is_skipped_path(path) or not is_text(data):
        return []
    text = data.decode("utf-8", errors="replace")
    findings: list[Finding] = []
    for index, line in enumerate(text.splitlines(), start=1):
        findings.extend(line_findings(scope, path, index, line, commit=commit, blob=blob))
    return findings


def protected_file_findings() -> list[Finding]:
    tracked = set(iter_tracked_paths())
    return [
        Finding("current", path, 0, "protected-local-secret-file-tracked")
        for path in sorted(PROTECTED_TRACKED_PATHS & tracked)
    ]


def scan_current_tree() -> list[Finding]:
    if not os.path.isdir(current_root):
        print(f"secret-scan: current root does not exist or is not a directory: {current_root}", file=sys.stderr)
        sys.exit(2)

    findings = protected_file_findings()
    for path in iter_tracked_paths():
        if is_skipped_path(path):
            continue
        full_path = os.path.join(current_root, path)
        try:
            with open(full_path, "rb") as f:
                data = f.read(MAX_BLOB_BYTES + 1)
        except FileNotFoundError:
            continue
        if len(data) > MAX_BLOB_BYTES:
            continue
        findings.extend(scan_text("current", path, data))
    return findings


def iter_history_objects() -> Iterable[tuple[str, str]]:
    seen: set[str] = set()
    for raw_line in git_text(["rev-list", "--objects", "--all"]).splitlines():
        if not raw_line:
            continue
        oid, _, path = raw_line.partition(" ")
        if not path or oid in seen:
            continue
        seen.add(oid)
        yield oid, path


@lru_cache(maxsize=4096)
def object_type(oid: str) -> str:
    proc = git(["cat-file", "-t", oid], check=False)
    if proc.returncode != 0:
        return ""
    return proc.stdout.decode("utf-8", errors="replace").strip()


@lru_cache(maxsize=4096)
def object_size(oid: str) -> int:
    proc = git(["cat-file", "-s", oid], check=False)
    if proc.returncode != 0:
        return MAX_BLOB_BYTES + 1
    try:
        return int(proc.stdout.decode("ascii", errors="replace").strip())
    except ValueError:
        return MAX_BLOB_BYTES + 1


@lru_cache(maxsize=4096)
def first_commit_for_blob(oid: str, path: str) -> str | None:
    proc = git(["log", "--all", "--find-object", oid, "--format=%H", "--", path], check=False)
    if proc.returncode != 0:
        return None
    for line in proc.stdout.decode("utf-8", errors="replace").splitlines():
        if line:
            return line[:12]
    return None


def is_shallow_repository() -> bool:
    proc = git(["rev-parse", "--is-shallow-repository"], check=False)
    if proc.returncode == 0:
        return proc.stdout.decode("utf-8", errors="replace").strip().lower() == "true"

    shallow_file = git_text(["rev-parse", "--git-path", "shallow"], check=False).strip()
    return bool(shallow_file and os.path.exists(os.path.join(repo_root, shallow_file) if not os.path.isabs(shallow_file) else shallow_file))


def scan_history_blobs() -> list[Finding]:
    if is_shallow_repository():
        print("secret-scan: history scan requires full history", file=sys.stderr)
        sys.exit(1)

    findings: list[Finding] = []
    for oid, path in iter_history_objects():
        if is_skipped_path(path) or object_type(oid) != "blob" or object_size(oid) > MAX_BLOB_BYTES:
            continue
        data = git(["cat-file", "-p", oid]).stdout
        blob_findings = scan_text("history", path, data, blob=oid[:12])
        if blob_findings:
            commit = first_commit_for_blob(oid, path)
            findings.extend(
                Finding(f.scope, f.path, f.line, f.reason, commit=commit, blob=f.blob)
                for f in blob_findings
            )
    return findings


def print_findings(findings: list[Finding]) -> None:
    for finding in findings:
        location = finding.path
        if finding.line:
            location = f"{location}:{finding.line}"
        details = []
        if finding.commit:
            details.append(f"commit={finding.commit}")
        if finding.blob:
            details.append(f"blob={finding.blob}")
        suffix = f" ({', '.join(details)})" if details else ""
        print(f"secret-scan: {finding.scope} {location}: {finding.reason} [redacted]{suffix}", file=sys.stderr)


all_findings: list[Finding] = []
if scan_current:
    all_findings.extend(scan_current_tree())
if scan_history:
    all_findings.extend(scan_history_blobs())

if all_findings:
    print_findings(all_findings)
    sys.exit(1)

scopes = []
if scan_current:
    scopes.append("current")
if scan_history:
    scopes.append("history")
print(f"secret-scan: ok ({', '.join(scopes)})")
PY
