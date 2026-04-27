[English](README.md) | 日本語

# SSNP SIP ドラフト

このリポジトリには次を含む。

- SSNP の SIP と設計ドキュメント
- `agent_py`: オペレーター向け Python Program Agent client
- `agent`: 廃止予定の Go 参照実装
- `portal`: portal/API stub
- `probe`: external probe worker stub

## 入口

- リポジトリ概要と共通 smoke 実行: `make smoke`
- Python agent の使い方: `agent/README_ja.md`
- portal の使い方: `portal/README.md`
- probe の使い方: `probe/README.md`
- testing ガイド: `docs/testing.md`
- testnet 運用手順: `docs/testnet_runbook_ja.md`
- Python agent 配布: `docs/agent_py_distribution.md`
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
- `make build`: portal / probe worker の build と Python agent client の構文確認
- `make smoke`: 最低限動作の正規チェック。Python agent client を使う end-to-end smoke フローを実行する

smoke 用 seed データの説明は `testdata/smoke/README.md` を参照。

## 位置づけ

SSNP は次として扱う。

- ネットワーク安定化支援プログラム
- コンセンサス変更ではない
- 既存ハーベスト報酬の削減ではない
- 報酬配布が有効になる前でも意味があるべきもの
