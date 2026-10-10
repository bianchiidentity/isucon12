---
name: bench-review
description: ベンチ完了後にpprotein・最新ログ・稼働コードを照合し、GitHub Kanbanの未着手issueを負荷順に並べ替え、変更理由をissueコメントへ追記する。「ベンチしました」「pproteinを見て優先順位を見直して」「ベンチ後レビュー」で使用する。
---

# Bench review

[共通手順](../../../docs/bench-review/WORKFLOW.md) に従う。Codex・Claude共通。

対象ベンチを確定 → 同じ計測グループのHTTP・SQL・CPUを分析 → issueと実装を照合 → 未着手列を並べ替え → 変化のあるissueへ根拠をコメント → 読み戻して確認。

このSkillの実行依頼は、対象Projectの並べ替えとissueへのレビューコメントを含む。毎回の確認待ちは挟まない。Skillの作成・編集を頼まれただけなら、実際の並べ替え・コメント投稿は行わない。

ベンチ実行・再収集・実装・デプロイ・画像生成は行わない。元のissue本文を保持し、負荷が見えなくなっただけでCloseやDoneにしない。接続先・リポジトリ・Projectは今回の対象を使い、本番でも練習環境の値に固定しない。
