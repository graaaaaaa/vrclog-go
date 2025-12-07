# ディレクトリ構成

## 最終構成（Codex MCPレビュー反映版）

```
vrclog-go/
├── cmd/
│   └── vrclog/
│       ├── main.go           # CLIエントリポイント、rootコマンド定義
│       └── tail.go           # tailサブコマンド実装
├── pkg/
│   └── vrclog/
│       ├── event/            # Event型（import cycle回避のため独立）
│       │   └── event.go      # Event, Type 定義
│       ├── doc.go            # パッケージドキュメント
│       ├── types.go          # event パッケージの re-export
│       ├── parse.go          # ParseLine公開API
│       ├── watcher.go        # NewWatcher/Watch API
│       ├── errors.go         # センチネルエラー定義
│       └── vrclog_test.go    # 公開APIのテスト
├── internal/
│   ├── logfinder/
│   │   ├── finder.go         # ログディレクトリ・ファイル検出
│   │   └── finder_test.go    # finderのテスト
│   ├── parser/
│   │   ├── parser.go         # ログ行パース実装
│   │   ├── patterns.go       # 正規表現パターン定義
│   │   └── parser_test.go    # parserのテーブル駆動テスト
│   └── tailer/
│       ├── tailer.go         # nxadm/tailのラッパー
│       └── tailer_test.go    # tailerのテスト（新規）
├── testdata/
│   └── logs/
│       └── sample.txt        # テスト用サンプルログ（匿名化）
├── plan/
│   └── *.md                  # 設計ドキュメント（このディレクトリ）
├── go.mod                    # モジュール定義
├── go.sum                    # 依存関係チェックサム
├── .gitignore                # Git除外設定
├── README.md                 # 英語ドキュメント
└── README.ja.md              # 日本語ドキュメント
```

### Import Cycle回避

**問題**: `pkg/vrclog` と `internal/parser` の間でimport cycleが発生する

**解決策**: Event型を `pkg/vrclog/event/` サブパッケージに分離

```
pkg/vrclog/event/event.go    # Event型の実体
    ↑
internal/parser/parser.go    # event パッケージをimport
    ↑
pkg/vrclog/watcher.go        # parser と event をimport

pkg/vrclog/types.go          # event パッケージを re-export（互換性維持）
```

## 各ディレクトリの役割

### cmd/

**役割**: 実行可能バイナリのエントリポイント

```
cmd/
└── vrclog/           # vrclog.exe としてビルドされる
    ├── main.go       # main()関数、rootコマンド
    └── tail.go       # tailサブコマンド
```

**設計原則**:
- `main.go` は最小限のコードのみ
- ビジネスロジックは `pkg/` または `internal/` に配置
- `pkg/vrclog` のみを import する（`internal/` は直接参照しない）

### pkg/

**役割**: 外部から import される公開API

```
pkg/
└── vrclog/
    ├── event/        # Event型サブパッケージ（import cycle回避）
    │   └── event.go  # Event, Type 定義
    ├── doc.go        # パッケージ全体のドキュメント
    ├── types.go      # event パッケージの re-export
    ├── parse.go      # ParseLine() 関数
    ├── watcher.go    # NewWatcher(), Watch() など
    ├── errors.go     # ErrLogDirNotFound など
    └── vrclog_test.go
```

**インポート方法**:
```go
// 推奨: メインパッケージ経由（re-export利用）
import "github.com/vrclog/vrclog-go/pkg/vrclog"
event := vrclog.Event{}

// 直接アクセスも可能
import "github.com/vrclog/vrclog-go/pkg/vrclog/event"
ev := event.Event{}
```

**設計原則**:
- 外部に公開する型・関数のみを定義
- 実装の詳細は `internal/` に委譲
- 安定したAPIを維持（v1以降は後方互換性を保証）
- Event型は独立パッケージでimport cycle回避

### internal/

**役割**: 非公開の実装詳細

```
internal/
├── logfinder/        # ログディレクトリ・ファイル検出
├── parser/           # ログ行パース
└── tailer/           # ファイル監視
```

**設計原則**:
- Go言語が強制するアクセス制御（外部からimport不可）
- 実装の詳細を隠蔽
- APIを壊さずに内部実装を変更可能

### testdata/

**役割**: テスト用データファイル

```
testdata/
└── logs/
    └── sample.txt    # 匿名化されたVRChatログサンプル
```

**設計原則**:
- `go test` で自動的に無視される命名規則
- 実際のログを匿名化して使用
- 各種イベントパターンを網羅

## pkg/ vs internal/ の使い分け

### なぜ pkg/ を使うか

Go コミュニティでは `pkg/` の使用について議論がある：

**賛成派**:
- 公開APIであることが明示的
- リポジトリ構造が一貫性を持つ
- ライブラリとしての用途が明確

**反対派**:
- import パスが長くなる
- Go公式は推奨していない

**このプロジェクトでの決定**: `pkg/` を使用

理由:
1. ライブラリとしての再利用を前提
2. 公開APIと内部実装の境界を明確化
3. 仕様書で指定されている

### internal/ の活用

```go
// pkg/vrclog/parse.go
package vrclog

import "github.com/vrclog/vrclog-go/internal/parser"

func ParseLine(line string) (*Event, error) {
    return parser.Parse(line)  // internalへ委譲
}
```

## ファイル命名規則

### Goファイル

| ファイル名 | 内容 |
|-----------|------|
| `xxx.go` | 実装 |
| `xxx_test.go` | テスト |
| `doc.go` | パッケージドキュメント |

### テストファイル

- `_test.go` サフィックスは必須
- テスト対象と同じディレクトリに配置
- パッケージ名は `xxx_test`（ブラックボックステスト）または `xxx`（ホワイトボックステスト）

## 将来の拡張

### 追加予定のディレクトリ

```
vrclog-go/
├── .github/
│   └── workflows/
│       └── ci.yml            # GitHub Actions CI
├── docs/
│   └── api.md                # API詳細ドキュメント
└── examples/
    └── basic/
        └── main.go           # 使用例
```

### 追加予定のファイル

```
cmd/vrclog/
├── version.go                # versionサブコマンド
└── completion.go             # シェル補完生成

pkg/vrclog/
└── option.go                 # Functional Options パターン（必要に応じて）
```
