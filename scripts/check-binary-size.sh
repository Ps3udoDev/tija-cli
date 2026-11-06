#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
output_dir="dist/binary-size"
mkdir -p "$output_dir"

# Decimal bytes, authorized for phase 5. Only the maximum blocks the build.
goal=15000000
alert=20000000
maximum=25000000
failed=0
for target in windows/amd64 linux/amd64 darwin/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  binary="$output_dir/tija-$os-$arch"
  if [[ "$os" == windows ]]; then binary+=".exe"; fi
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -ldflags='-s -w' -o "$binary" ./cmd/tija
  bytes="$(wc -c < "$binary" | tr -d '[:space:]')"
  status=GOAL_MET
  if (( bytes > maximum )); then
    status=MAXIMUM_EXCEEDED
    failed=1
  elif (( bytes >= alert )); then
    status=TECHNICAL_ALERT
  elif (( bytes >= goal )); then
    status=ABOVE_GOAL
  fi
  printf '%s %s bytes %s (goal < %s, alert >= %s, maximum %s)\n' "$target" "$bytes" "$status" "$goal" "$alert" "$maximum"
done
exit "$failed"
