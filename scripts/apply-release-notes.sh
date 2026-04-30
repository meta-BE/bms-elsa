#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [ ! -d docs/release-notes ]; then
  echo "ERROR: docs/release-notes/ ディレクトリが存在しません" >&2
  exit 1
fi

shopt -s nullglob
files=(docs/release-notes/v*.md)
if [ ${#files[@]} -eq 0 ]; then
  echo "ERROR: docs/release-notes/v*.md にファイルがありません" >&2
  exit 1
fi

echo "適用対象: ${#files[@]} 件"
for f in "${files[@]}"; do
  tag="$(basename "$f" .md)"
  echo "  - $tag"
done

read -r -p "実行しますか？ [y/N] " confirm
if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
  echo "キャンセルしました。"
  exit 0
fi

for f in "${files[@]}"; do
  tag="$(basename "$f" .md)"
  echo "Updating release: $tag"
  gh release edit "$tag" --notes-file "$f"
done

echo "完了"
