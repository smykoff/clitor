#!/usr/bin/env bash
# Update sources.json from the latest GitHub release.
#
# Normally CI does this automatically after every build (see ci.yml).
# This script is for manual/local updates.
set -euo pipefail
cd "$(dirname "$0")/.."

repo="smykoff/clitor"
tag=$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" | jq -r .tag_name)
version=${tag#v}
current=$(jq -r .version sources.json 2>/dev/null || echo none)

if [ "$version" = "$current" ]; then
  echo "up to date: $current"
  exit 0
fi

declare -A files=(
  [x86_64-linux]="clitor-linux-amd64.tar.gz"
  [aarch64-linux]="clitor-linux-arm64.tar.gz"
  [aarch64-darwin]="clitor-darwin-arm64.tar.gz"
)

json=$(jq -n --arg v "$version" '{version: $v}')
for sys in "${!files[@]}"; do
  url="https://github.com/${repo}/releases/download/${tag}/${files[$sys]}"
  hash=$(nix store prefetch-file --json "$url" | jq -r .hash)
  json=$(jq --arg s "$sys" --arg u "$url" --arg h "$hash" \
    '.[$s] = {url: $u, hash: $h}' <<<"$json")
done

jq . <<<"$json" > sources.json
echo "updated: $current -> $version"
echo "version=$version" >> "${GITHUB_OUTPUT:-/dev/null}"
