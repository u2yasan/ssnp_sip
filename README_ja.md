[English](README.md) | 日本語

# SSNP SIP ドラフト

このリポジトリには次を含む。

- SSNP の SIP と設計ドキュメント
- `agent`: Program Agent stub
- `portal`: portal/API stub

## 入口

- リポジトリ概要と共通 smoke 実行: `make smoke`
- agent の使い方: `agent/README.md`
- portal の使い方: `portal/README.md`
- testing ガイド: `docs/testing.md`
- 設計概要: `docs/specs/00_project_overview.md`
- 未解決論点: `docs/specs/10_open_questions.md`

## 最低限の検証

リポジトリ root で次を使う。

```sh
make smoke
make test
make build
```

コマンドの役割:

- `make test`: リポジトリ全体の回帰検知
- `make build`: 両サービスの build
- `make smoke`: 最低限動作の正規チェック。Go の end-to-end smoke フローを実行する

smoke 用 seed データの説明は `testdata/smoke/README.md` を参照。

## 位置づけ

SSNP は次として扱う。

- ネットワーク安定化支援プログラム
- コンセンサス変更ではない
- 既存ハーベスト報酬の削減ではない
- 報酬配布が有効になる前でも意味があるべきもの
