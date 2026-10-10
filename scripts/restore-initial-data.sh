#!/usr/bin/env bash
# 展開先だけを変更する。DBへの投入・サービス操作は行わない。
set -euo pipefail
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
[[ $# -le 1 ]] || { echo '使い方: restore-initial-data.sh [展開先]' >&2; exit 1; }
destination=${1:-"$repo"}
mkdir -p -- "$destination"
destination=$(cd -- "$destination" && pwd -P)
cd -- "$repo/baseline/data"
sha256sum --check parts.sha256
# 既存ファイルのある場所には展開しない。復元は新規ディレクトリを推奨。
while read -r hash path; do
  [[ ! -e "$destination/$path" && ! -L "$destination/$path" ]] || {
    printf '既存ファイルを上書きできません: %s\n' "$destination/$path" >&2
    exit 1
  }
done < original.sha256
cat initial-data.tar.gz.part* | tar --keep-old-files -xzf - -C "$destination"
cd -- "$destination"
sha256sum --check "$repo/baseline/data/original.sha256"
printf '初期データの展開・検証完了: %s\n' "$destination"
