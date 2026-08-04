#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
script="$repo_root/scripts/release.sh"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$file"; then
    echo "expected '$expected' in $file" >&2
    cat "$file" >&2
    exit 1
  fi
}

assert_fails_with() {
  local expected="$1"
  shift
  if "$@" >"$tmpdir/out" 2>"$tmpdir/err"; then
    echo "command unexpectedly succeeded: $*" >&2
    exit 1
  fi
  assert_contains "$tmpdir/err" "$expected"
}

"$script" metadata "v1.0.0-rc.1" >"$tmpdir/rc.env"
assert_contains "$tmpdir/rc.env" "PRERELEASE=true"
assert_contains "$tmpdir/rc.env" "LATEST_TAG=false"
assert_contains "$tmpdir/rc.env" "MANIFEST_FILE=resource-exporter-v1.0.0-rc.1.yaml"
"$script" generate-manifest "v1.0.0-rc.1" "$tmpdir"
assert_contains "$tmpdir/resource-exporter-v1.0.0-rc.1.yaml" "image: volcanosh/numatopo:v1.0.0-rc.1"

"$script" metadata "v1.0.0" >"$tmpdir/stable.env"
assert_contains "$tmpdir/stable.env" "PRERELEASE=false"
assert_contains "$tmpdir/stable.env" "LATEST_TAG=true"
"$script" generate-manifest "v1.0.0" "$tmpdir"
assert_contains "$tmpdir/resource-exporter-v1.0.0.yaml" "image: volcanosh/numatopo:v1.0.0"

assert_fails_with "release tag must not be empty" "$script" metadata ""
assert_fails_with "invalid release tag" "$script" metadata "latest"

echo "release helper tests passed"
