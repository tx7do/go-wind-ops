<div align="center">

# GoWind Ops

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Kratos](https://img.shields.io/badge/Kratos-2.9-00ADD8?logo=go&logoColor=white)](https://go-kratos.dev/)
[![React](https://img.shields.io/badge/React-19.x-61DAFB?logo=react&logoColor=white)](https://react.dev/)

[English](./README.en-US.md) | [中文](./README.md) | **日本語**

</div>

---

GoWind Ops は、Golang フルスタックで構築されたマルチノード運用プラットフォームです。業務ドメインは [Jpom](https://gitee.com/dromara/Jpom) を参考に、シグナリングと大容量ファイル転送のアーキテクチャは cloud-system を参考にしています。プロジェクト全体は [go-wind-cms](../go-wind-cms) をベースに簡素化したものであり、組織・権限、監査ログ、データ辞書などのシステム機能を引き継ぎつつ、「マシンの管理・プロジェクトの実行・スクリプト配布・ビルドとデプロイ」に集中しています。

**主な特徴：**

- **4 サービスアーキテクチャ** — core（ドメインリソース層）/ admin / front（BFF）/ agent（エッジプロキシ）、単方向のレイヤリング、BFF は DB 非接続
- **MQTT シグナリングの常時接続** — agent は EMQX への純アウトバウンド接続（着信ポートなし）、`$share` 共有サブスクリプションでマルチレプリカ負荷分散、QoS 1 + request_id による冪等な突合
- **大容量ファイルはシグナリングを経由しない** — MinIO 事前署名 URL による直接アップロード/ダウンロード、ストリーミング + SHA-256 検証
- **コマンドの完全監査** — すべてのコマンドを台帳に記録（PENDING → 最終状態 / タイムアウト判定）、実行結果・所要時間・失敗理由を追跡可能
- **ビルドとデプロイのクローズドループ** — Git リポジトリ → ビルド（成果物を MinIO へ）→ 対象プロジェクトディレクトリへ配布 → 連動再起動、全体をステートマシンで追跡
- **ベース同一由来の管理フロントエンド** — React 19 + Ant Design 6 + ProComponents、go-wind-cms admin から派生し、ログイン/権限/監査/辞書がそのまま使える

## 技術スタック

### バックエンド

| レイヤー | 技術 | 説明 |
|:-------|:-------------------------------------------------------------------------------------|:------------|
| 言語     | [Go 1.26+](https://go.dev/)                                                          | 高性能コンパイル言語    |
| フレームワーク | [go-kratos](https://go-kratos.dev/)                                                  | Bilibili 製マイクロサービスフレームワーク   |
| DI     | 手書きアセンブリ（wiring.go）                                                                  | DI フレームワークなし、`make register` が登録を支援 |
| ORM    | [Ent](https://entgo.io/)                                                             | Go エンティティフレームワーク     |
| データベース | [PostgreSQL](https://www.postgresql.org/) / [MySQL](https://www.mysql.com/)          | リレーショナルデータベース      |
| キャッシュ/キュー | [Redis](https://redis.io/) + [Asynq](https://github.com/hibiken/asynq)               | キャッシュと非同期タスク     |
| オブジェクトストレージ | [MinIO](https://min.io/)                                                             | S3 互換オブジェクトストレージ、事前署名 URL 発行 |
| シグナリング | [MQTT](https://mqtt.org/)（[EMQX](https://www.emqx.io/)）                              | agent の認証/ハートビート/コマンド、QoS 1 + JSON |
| サービス登録 | [Etcd](https://etcd.io/)                                                             | サービスディスカバリ（agent を除く：純アウトバウンドで未登録） |
| 分散トレース | [Jaeger](https://www.jaegertracing.io/) + [OpenTelemetry](https://opentelemetry.io/) | 分散オブザーバビリティ      |
| API 定義 | [Protobuf](https://protobuf.dev/) + [buf.build](https://buf.build/)                  | 契約優先のインターフェース      |
| 認可     | kratos-authn（JWT）/ kratos-authz（Casbin / OPA）                                        | ポリシードリブンな認可      |

### 管理フロントエンド

| 技術                                            | 説明         |
|:----------------------------------------------|:-----------|
| [React 19](https://react.dev/)                | プログレッシブ UI フレームワーク    |
| [TypeScript](https://www.typescriptlang.org/) | 型安全       |
| [Ant Design 6](https://ant.design/)           | エンタープライズ UI コンポーネントライブラリ |
| [ProComponents](https://procomponents.ant.design/) | 管理画面向けコンポーネント群 |
| [Vite 6](https://vite.dev/)                   | ビルドツール（dev :15999） |

## 主な機能

### ノードとプロジェクト

| 機能    | 説明                                      |
|:------|:----------------------------------------|
| ノード管理 | agent のマシンフィンガープリント認証（TOFU 初回接続登録、認証まで自動リトライ）、30 秒ごとのハートビートでメトリクス報告、オンライン状態、改名/ラベル付け、ping コマンド送信 |
| プロジェクト管理 | プロジェクト = ノード + ディレクトリ + 起動/停止コマンド。ライフサイクル操作 START/STOP/RESTART/STATUS、agent 側の許可ディレクトリ検証（`AGENT_ALLOWED_DIRS`）+ pid ファイル管理 |
| コマンド台帳 | すべてのコマンドを監査記録（結果/終了コード/所要時間/失敗理由）、タイムアウトは自動で FAILED 判定 |
| 実行ログ   | プロセス出力を `<プロジェクトディレクトリ>/.ops-project.log` に保存、テイル参照（既定 64KB/上限 256KB）とクリア |

### スクリプト・ビルド・デプロイ

| 機能    | 説明                                          |
|:------|:--------------------------------------------|
| スクリプト管理 | スクリプト CRUD（内容 ≤256KB）。リモート実行：内容を MQTT で配信、一時ファイル経由でプラットフォームのシェルで実行、出力/終了コード/所要時間を台帳に記録、タイムアウト時はプロセスツリーを kill |
| ソースリポジトリ | HTTPS Git / ローカルパス。パスワードは AES-GCM で暗号化して保存、表示はマスキング       |
| ビルド   | ビルド設定（リポジトリ/ブランチ/ビルドコマンド/成果物パス）+ 非同期トリガー（ビルド番号自動採番、同時実行の重複排除、中断時の判定）。成果物は glob 収集してパッキングし MinIO `artifacts` バケットへ、事前署名ダウンロード |
| デプロイ  | ビルド成果物を対象プロジェクトディレクトリへ配布（zip 展開は zip slip 対策済み、clearOld 任意、配布後の再起動/起動と連動）、配布レコードはステートマシンで追跡 |

### システム・運用（ベースから継承）

| 機能           | 説明                                    |
|:-------------|:--------------------------------------|
| 組織と権限        | ユーザー/ロール/メニュー/部門/API 権限管理、ボタンレベルの権限制御（go-wind-cms ベースから継承） |
| 監査と辞書        | ログインログ、操作ログ、データ辞書、言語管理などのシステム機能              |

## プロジェクト構成

```
go-wind-ops/
├── backend/                        # バックエンドサービス
│   ├── api/                        # Protobuf API 定義 & 生成コード
│   │   ├── protos/                 # .proto ソース（ops 業務ドメイン + ベース継承ドメイン）
│   │   └── gen/                    # 生成コード (Go / TypeScript / OpenAPI)、手動編集禁止
│   ├── app/
│   │   ├── admin/service/          # 管理 BFF (REST :6600、gRPC → core、DB 非接続)
│   │   ├── front/service/          # フロント BFF (REST :6700、gRPC → core、DB 非接続)
│   │   ├── core/service/           # ドメインリソース層 (Ent スキーマ/リポジトリ/業務ロジック + MQTT コントローラ + Asynq worker)
│   │   └── agent/service/          # エッジプロキシ (純アウトバウンド MQTT、DB/サービスディスカバリ/REST なし)
│   ├── pkg/                        # 共有パッケージ (signal/topic/serviceid/oss/filetransfer/認証ミドルウェア...)
│   └── scripts/                    # デプロイスクリプト (Docker など)
└── frontend/
    └── admin/                      # 管理フロントエンド (React 19 + Ant Design 6 + ProComponents + Vite 6)
```

## クイックスタート

### 環境要件

- Go 1.26+
- Docker & Docker Compose
- Node.js 20+ & pnpm
- buf / protoc ツールチェーン（proto 変更時のみ、`cd backend && make init` で一括インストール）

### 1. 依存サービスの起動

```bash
cd backend
docker compose -f docker-compose.libs.yaml up -d   # PG/Redis/MinIO/etcd/EMQX/Jaeger
```

### 2. バックエンドサービスの起動

```bash
cd backend
go build -o bin/core ./app/core/service/cmd/server && ./bin/core -c app/core/service/configs
go build -o bin/agent ./app/agent/service/cmd/server && ./bin/agent -c app/agent/service/configs
```

agent は初回起動時に作業ディレクトリへ `auth.token` を生成し、MQTT 経由で core に登録します（TOFU、認証されるまで 5 秒ごとに再試行）。その後 30 秒ごとにハートビートでメトリクスを報告します。管理対象マシンで agent を実行する前に、許可ディレクトリ（環境変数 `AGENT_ALLOWED_DIRS`、プラットフォームのパス区切りで列挙）を設定してください。プロジェクトディレクトリがその内側にない場合、agent は実行を拒否します。

### 3. 管理フロントエンドの起動

```bash
cd frontend/admin
pnpm install
pnpm dev        # http://localhost:15999、/admin → 127.0.0.1:6600 にプロキシ
```

デフォルトアカウントは `admin / admin`（開発環境では `ADMIN_CAPTCHA_DISABLED=true` でキャプチャをスキップ可能。ログインパスワードはフロントエンドの規約に従い AES-GCM で暗号化する必要があります。詳細は `backend/tools/pass-enc` を参照）。

### よく使うコマンド

```bash
cd backend

# Ent ORM コード生成
make ent

# Protobuf API Go コード生成
make api

# 管理フロントエンド用 TypeScript クライアント生成
cd api && buf generate --template buf.admin.typescript.gen.yaml

# 全コード生成 (ent + api + openapi)
make gen

# 全サービスのビルド
make build

# テスト実行
make test
```

> 管理 REST エンドポイント一覧（ノード/プロジェクト/スクリプト/リポジトリ/ビルド/デプロイ/台帳）は [backend/README.md](./backend/README.md)、バックエンド開発規約とアーキテクチャ詳細は [backend/AGENTS.md](./backend/AGENTS.md) を参照してください。

## 開発ロードマップ

- [x] ベースの確立：4 サービス分割（core/front/admin/agent）、CMS 業務ドメインの簡素化、MQTT シグナリングと MinIO 転送インフラ
- [x] ノード管理：proto 化された REST（一覧/詳細/改名・ラベル/削除/ping 送信）、認証リトライ、コマンド結果の突合
- [x] プロジェクト管理 + コマンド台帳：プロジェクト CRUD とライフサイクル操作（起動/停止/再起動/状態、agent 側の許可ディレクトリ検証 + pid ファイル管理）、コマンド監査とタイムアウト判定
- [x] スクリプト管理：スクリプト CRUD + リモート実行（内容は MQTT で配信、一時ファイルで実行、出力/終了コード/所要時間を台帳へ、タイムアウトでプロセスツリー kill）
- [x] ビルドモジュール v1：ソースリポジトリ管理（認証情報の暗号化）、ビルド設定とトリガー（ビルド番号自動採番/同時実行の重複排除/中断時の判定）、ビルド履歴と成果物の MinIO 格納、事前署名ダウンロード
- [x] 実行ログ：プロセス出力のディスク保存（`<プロジェクトディレクトリ>/.ops-project.log`、メモリリーク修正）、テイル参照（同期コマンドで受領）とクリア
- [x] 管理フロントエンド基盤：cms admin フロントエンドのコピー裁断（ログイン/レイアウト/権限/監査/辞書スケルトン + TS API 再生成）、最初の ops ページ（ノード管理：一覧/ping 送信/削除）
- [x] 業務ページ 7 連発：ノード/プロジェクト（ログ表示・ライフサイクル操作）/スクリプト/リポジトリ/ビルド/デプロイ/コマンド台帳
- [x] デプロイ v1：配布設定（clearOld/配布後アクション）とトリガー、zip slip 対策済み展開、配布レコードのステートマシン
- [ ] 業務モジュール：マルチターゲット配布（ノードグループ）、監視・アラート（Monitor）
- [ ] 認証強化：EMQX webhook 認証/ACL、STS 短命トークンローテーション
- [ ] ビルド強化：定時ビルド（cron）、ビルドログのライブテイル
- [ ] ログ強化：WebSocket ライブテイル、ログファイルのダウンロード/バックアップ、コンソール文字コード変換（Windows GBK 出力はバイトのまま保存）

## コントリビューション

以下のような、あらゆる形での貢献を歓迎します：

- [Issue](../../issues) によるバグ報告や機能提案
- [Pull Request](../../pulls) による修正や新機能の追加
- ドキュメントの改善
- 使用体験の共有

提出前に [CONTRIBUTING.md](./CONTRIBUTING.md) で開発規約とゲートをご確認ください。

## ライセンス

本プロジェクトは [MIT License](./LICENSE) の下で公開されています。

## 謝辞

- [go-wind-cms](../go-wind-cms) — プロジェクトのベース（Kratos + Ent + tx7do ツールキットと admin フロントエンドはすべてベースから派生）
- [Jpom](https://gitee.com/dromara/Jpom) — 業務ドメインモデルの参考
- cloud-system — MQTT シグナリングと大容量ファイル転送アーキテクチャの参考
