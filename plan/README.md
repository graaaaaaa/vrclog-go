# vrclog-go 設計ドキュメント

このディレクトリには vrclog-go プロジェクトの設計ドキュメントが含まれています。
実装時にはこれらのファイルを参照してください。

## ドキュメント一覧

### 概要・仕様

| ファイル | 内容 |
|----------|------|
| [00-overview.md](00-overview.md) | プロジェクト概要、目的、スコープ |
| [01-vrchat-log-format.md](01-vrchat-log-format.md) | VRChatログフォーマットの詳細仕様 |

### 設計

| ファイル | 内容 |
|----------|------|
| [02-directory-structure.md](02-directory-structure.md) | ディレクトリ構成、各ディレクトリの役割 |
| [03-public-api-design.md](03-public-api-design.md) | 公開API（Event, Watch, ParseLine）の設計 |
| [04-internal-packages.md](04-internal-packages.md) | 内部パッケージ（logfinder, parser, tailer）の設計 |
| [05-cli-design.md](05-cli-design.md) | CLI（cobra）の設計、コマンド仕様 |

### 実装ガイド

| ファイル | 内容 |
|----------|------|
| [06-testing-strategy.md](06-testing-strategy.md) | テスト戦略、テーブル駆動テスト |
| [07-error-handling.md](07-error-handling.md) | エラーハンドリング方針 |
| [08-go-best-practices.md](08-go-best-practices.md) | Goベストプラクティスのまとめ |
| [09-implementation-steps.md](09-implementation-steps.md) | 実装手順、チェックリスト |

### 設計原則・パターン

| ファイル | 内容 |
|----------|------|
| [14-software-design-principles.md](14-software-design-principles.md) | SOLID、KISS、YAGNI、DRY、Clean Architecture |
| [15-go-idioms-patterns.md](15-go-idioms-patterns.md) | Goイディオム、Pipeline、Context、チャネルパターン |
| [16-advanced-patterns.md](16-advanced-patterns.md) | シグナル処理、goroutineリーク防止、セマンティックバージョニング |

### 参考資料・テンプレート

| ファイル | 内容 |
|----------|------|
| [10-reference-implementations.md](10-reference-implementations.md) | 参考となる実装、ライブラリ一覧 |
| [11-code-templates.md](11-code-templates.md) | コアパッケージのコードテンプレート |
| [12-cli-code-templates.md](12-cli-code-templates.md) | CLIとテストのコードテンプレート |
| [13-readme-templates.md](13-readme-templates.md) | README、LICENSEのテンプレート |

## 読む順序

### 初めて読む場合

1. [00-overview.md](00-overview.md) - プロジェクト全体像を把握
2. [01-vrchat-log-format.md](01-vrchat-log-format.md) - ログフォーマットを理解
3. [02-directory-structure.md](02-directory-structure.md) - コード構成を把握
4. [03-public-api-design.md](03-public-api-design.md) - 公開APIを理解

### 実装時

1. [09-implementation-steps.md](09-implementation-steps.md) - 実装手順を確認
2. 各ステップで対応するコードテンプレート（11, 12）を参照
3. [08-go-best-practices.md](08-go-best-practices.md) - ベストプラクティスを確認
4. [14-software-design-principles.md](14-software-design-principles.md) - 設計原則を確認
5. [15-go-idioms-patterns.md](15-go-idioms-patterns.md) - Goパターンを確認
6. [16-advanced-patterns.md](16-advanced-patterns.md) - 高度なパターンを確認

### デバッグ・トラブルシューティング

1. [01-vrchat-log-format.md](01-vrchat-log-format.md) - ログパターンを確認
2. [07-error-handling.md](07-error-handling.md) - エラーハンドリングを確認
3. [10-reference-implementations.md](10-reference-implementations.md) - 他の実装を参照

## クイックリファレンス

### イベントタイプ

| タイプ | 説明 | ログパターン |
|--------|------|-------------|
| `world_join` | ワールド入室 | `[Behaviour] Entering Room:` |
| `player_join` | プレイヤー参加 | `[Behaviour] OnPlayerJoined` |
| `player_left` | プレイヤー退出 | `[Behaviour] OnPlayerLeft` |

### 主要API

```go
// ログ監視
events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

// 単行パース
event, err := vrclog.ParseLine(line)
```

### 依存関係

```
github.com/spf13/cobra     # CLI
github.com/nxadm/tail      # ファイル監視
```

### ログファイル位置

```
%LOCALAPPDATA%Low\VRChat\VRChat\output_log_*.txt
%LOCALAPPDATA%Low\VRChat\vrchat\output_log_*.txt
```

## 更新履歴

- 2024-XX-XX: 初版作成

## 参考リンク

- [VRCX](https://github.com/vrcx-team/VRCX) - ログパースパターンの参考
- [Google Go Style Guide](https://google.github.io/styleguide/go/best-practices.html)
- [nxadm/tail](https://pkg.go.dev/github.com/nxadm/tail)
