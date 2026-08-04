#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  release.sh metadata <git-tag>
  release.sh generate-manifest <git-tag> <output-dir>
EOF
}

is_semver_tag() {
  local tag="${1:-}"
  [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]
}

require_tag() {
  local tag="${1:-}"
  if [[ -z "$tag" ]]; then
    echo "release tag must not be empty" >&2
    exit 1
  fi
  if ! is_semver_tag "$tag"; then
    echo "invalid release tag: $tag" >&2
    exit 1
  fi
}

command="${1:-}"

case "$command" in
  metadata)
    tag="${2:-}"
    require_tag "$tag"

    prerelease=false
    latest_tag=false
    if [[ "$tag" == *-* ]]; then
      prerelease=true
    else
      latest_tag=true
    fi

    cat <<EOF
TAG=$tag
VERSION=$tag
PRERELEASE=$prerelease
LATEST_TAG=$latest_tag
MANIFEST_FILE=resource-exporter-$tag.yaml
EOF
    ;;
  generate-manifest)
    tag="${2:-}"
    output_dir="${3:-}"
    require_tag "$tag"
    if [[ -z "$output_dir" ]]; then
      echo "output directory must not be empty" >&2
      exit 1
    fi

    repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
    template="$repo_root/installer/numa-topo.yaml"
    output_file="$output_dir/resource-exporter-$tag.yaml"

    mkdir -p "$output_dir"
    sed "s|volcanosh/numatopo:latest|volcanosh/numatopo:$tag|g" "$template" > "$output_file"
    echo "$output_file"
    ;;
  *)
    usage >&2
    exit 1
    ;;
esac
