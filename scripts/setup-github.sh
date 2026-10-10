#!/usr/bin/env bash
# isucon ユーザーで実行: ./scripts/setup-github.sh [クローン先]
set -euo pipefail
umask 077

die() { printf 'エラー: %s\n' "$*" >&2; exit 1; }

[[ $(id -un) == isucon ]] || die 'isucon ユーザーで実行してください。'
[[ $# -le 1 ]] || die '使い方: setup-github.sh [クローン先]'
for command in git ssh ssh-keygen; do
  command -v "$command" >/dev/null || die "$command が必要です。"
done

repo='git@github.com:bianchiidentity/isucon12.git'
destination=${1:-"$HOME/isucon12"}
mkdir -p -- "$(dirname -- "$destination")"
destination="$(cd -- "$(dirname -- "$destination")" && pwd -P)/$(basename -- "$destination")"
if [[ -e "$destination" ]]; then
  [[ -d "$destination/.git" ]] || die "クローン先が既に存在します: $destination"
  [[ $(git -C "$destination" remote get-url origin) == "$repo" ]] || die 'クローン先の origin が対象リポジトリと異なります。'
fi

ssh_dir="$HOME/.ssh"
key="$ssh_dir/id_ed25519_github_isucon"
config="$ssh_dir/config_github_isucon"
mkdir -p -- "$ssh_dir"
chmod 700 "$ssh_dir"
if [[ ! -e "$key" ]]; then
  [[ ! -e "$key.pub" ]] || die "公開鍵だけが存在します: $key.pub"
  # パスフレーズは ssh-keygen のプロンプトで指定する。
  ssh-keygen -t ed25519 -C "isucon@$(hostname)-github" -f "$key"
fi
chmod 600 "$key"
if [[ ! -e "$key.pub" ]]; then
  ssh-keygen -y -f "$key" > "$key.pub"
fi
ssh-keygen -l -f "$key.pub" >/dev/null

# 通常の ~/.ssh/config から分離し、GitHub 専用鍵だけを使用する。
if [[ ! -e "$config" ]]; then
  cat > "$config" <<EOF
Host github.com
    HostName github.com
    User git
    IdentityFile "$key"
    IdentitiesOnly yes
    StrictHostKeyChecking ask
EOF
fi
chmod 600 "$config"

printf '\n次の公開鍵を GitHub に登録してください:\n\n'
cat "$key.pub"
printf '\nアカウントの SSH keys: https://github.com/settings/ssh/new\n'
printf 'このリポジトリだけに限定する場合は Deploy keys: https://github.com/bianchiidentity/isucon12/settings/keys\n'
printf 'Deploy key で push する場合は Allow write access を有効にしてください。\n'
printf '初回接続時のホスト鍵は GitHub 公式の fingerprint と照合してください:\n'
printf 'https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/githubs-ssh-key-fingerprints\n\n'
read -r -p '登録済みなら Enter を押してください（中止: Ctrl-C）: '

# GitHub は認証成功時も ssh -T の終了コードが 1 となる。
status=0
ssh -F "$config" -T git@github.com || status=$?
[[ $status -le 1 ]] || die 'SSH 接続に失敗しました。'

printf -v ssh_command 'ssh -F %q' "$config"
# 実際のリポジトリへのアクセスで認証と権限を確認する。
git -c core.sshCommand="$ssh_command" ls-remote "$repo" >/dev/null
if [[ ! -e "$destination" ]]; then
  git -c core.sshCommand="$ssh_command" clone -- "$repo" "$destination"
fi
git -C "$destination" config --local core.sshCommand "$ssh_command"
printf '\n設定完了: %s\nこのリポジトリの pull / push は専用 SSH 鍵を使用します。\n' "$destination"
