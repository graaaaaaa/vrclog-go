# 実装手順

## 概要

この文書は vrclog-go プロジェクトの実装を段階的に行うためのガイドです。
各ステップで作成するファイル、その内容、テストの順序を示します。

---

## Step 1: プロジェクト初期化

### 1.1 Go モジュール初期化

```bash
cd vrclog-go
go mod init github.com/vrclog/vrclog-go
```

### 1.2 ディレクトリ構造作成

```bash
mkdir -p cmd/vrclog
mkdir -p pkg/vrclog
mkdir -p internal/logfinder
mkdir -p internal/parser
mkdir -p internal/tailer
mkdir -p testdata/logs
```

### 1.3 .gitignore 作成

```gitignore
# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
/vrclog

# Test binary
*.test

# Output of go coverage tool
*.out
coverage.html

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Build output
/dist/
```

### 1.4 依存関係追加

```bash
go get github.com/spf13/cobra@latest
go get github.com/nxadm/tail@latest
```

---

## Step 2: 型定義とパーサー（コア）

### 2.1 pkg/vrclog/event.go

Event型とEventTypeの定義。JSONタグ付き。

**作成内容**:
- `EventType` 型と定数
- `Event` 構造体

**参照**: `plan/03-public-api-design.md`

### 2.2 pkg/vrclog/errors.go

センチネルエラーの定義。

**作成内容**:
- `ErrLogDirNotFound`
- `ErrNoLogFiles`

**参照**: `plan/07-error-handling.md`

### 2.3 pkg/vrclog/doc.go

パッケージドキュメント。

**作成内容**:
- パッケージの説明
- 使用例

### 2.4 internal/parser/patterns.go

正規表現パターンの定義。

**作成内容**:
- `timestampLayout` 定数
- `timestampPattern` 正規表現
- `playerJoinPattern` 正規表現
- `playerLeftPattern` 正規表現
- `enteringRoomPattern` 正規表現
- `joiningPattern` 正規表現
- `exclusionPatterns` スライス

**参照**: `plan/04-internal-packages.md`

### 2.5 internal/parser/parser.go

パース実装。

**作成内容**:
- `Parse(line string) (*vrclog.Event, error)` 関数
- `parseTimestamp()` ヘルパー
- `parsePlayerJoin()` ヘルパー
- `parsePlayerLeft()` ヘルパー
- `parseWorldJoin()` ヘルパー

### 2.6 testdata/logs/sample.txt

テスト用サンプルログ。

**作成内容**:
- 各種イベントパターンを含むサンプル
- 匿名化されたデータ

### 2.7 internal/parser/parser_test.go

パーサーのテーブル駆動テスト。

**作成内容**:
- 正常系テスト
- スキップ系テスト
- 境界値テスト
- ヘルパー関数

**テスト実行**:
```bash
go test -v ./internal/parser/
```

### 2.8 pkg/vrclog/parse.go

公開API ParseLine の実装。

**作成内容**:
- `ParseLine(line string) (*Event, error)` 関数
- internal/parser への委譲

---

## Step 3: ログファイル検出

### 3.1 internal/logfinder/finder.go

ログディレクトリ・ファイル検出の実装。

**作成内容**:
- `EnvLogDir` 定数
- `DefaultLogDirs()` 関数
- `FindLogDir(explicit string) (string, error)` 関数
- `FindLatestLogFile(dir string) (string, error)` 関数
- `isValidLogDir()` ヘルパー

**参照**: `plan/04-internal-packages.md`

### 3.2 internal/logfinder/finder_test.go

ファイル検出のテスト。

**作成内容**:
- `TestFindLatestLogFile`
- `TestFindLatestLogFile_NoFiles`
- `TestFindLogDir_EnvVar`
- `TestFindLogDir_Explicit`

**テスト実行**:
```bash
go test -v ./internal/logfinder/
```

---

## Step 4: ファイルtail実装

### 4.1 internal/tailer/tailer.go

nxadm/tail のラッパー実装。

**作成内容**:
- `Tail` 構造体
- `Config` 構造体
- `DefaultConfig()` 関数
- `New(filepath string, cfg Config) (*Tail, error)` 関数
- `Lines()` メソッド
- `Stop()` メソッド
- `run()` 内部goroutine

**参照**: `plan/04-internal-packages.md`

---

## Step 5: Watch API

### 5.1 pkg/vrclog/watch.go

Watch API の実装。

**作成内容**:
- `WatchOptions` 構造体
- `Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error)` 関数
- 内部の監視goroutine
- ファイルローテーション検出

**参照**: `plan/03-public-api-design.md`

### 5.2 pkg/vrclog/vrclog_test.go

公開APIのテスト。

**作成内容**:
- `TestParseLine`
- Example関数

**テスト実行**:
```bash
go test -v ./pkg/vrclog/
```

---

## Step 6: CLI

### 6.1 cmd/vrclog/main.go

CLI エントリポイント。

**作成内容**:
- `rootCmd` 定義
- グローバルフラグ（`--verbose`）
- `main()` 関数
- サブコマンド登録

**参照**: `plan/05-cli-design.md`

### 6.2 cmd/vrclog/tail.go

tail サブコマンド。

**作成内容**:
- `tailCmd` 定義
- フラグ（`--log-dir`, `--format`, `--types`, `--raw`）
- `runTail()` 関数
- `outputEvent()` 関数
- `outputJSON()` 関数
- `outputPretty()` 関数
- シグナルハンドリング

**ビルドとテスト**:
```bash
go build -o vrclog ./cmd/vrclog
./vrclog --help
./vrclog tail --help
```

---

## Step 7: ドキュメント

### 7.1 README.md

英語のメインドキュメント。

**構成**:
1. プロジェクト概要
2. インストール方法
3. 使用例（ライブラリ & CLI）
4. API リファレンス
5. コントリビューションガイド
6. ライセンス
7. 免責事項（VRChat非公式）

### 7.2 README.ja.md

日本語ドキュメント。

**構成**:
- 冒頭に「英語版が正」の注記
- README.md の翻訳

---

## 全体テスト

### テスト実行

```bash
# 全テスト
go test ./...

# 詳細出力
go test -v ./...

# race detector
go test -race ./...

# カバレッジ
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### ビルド確認

```bash
# ビルド
go build ./...

# クロスコンパイル（Windows用）
GOOS=windows GOARCH=amd64 go build -o vrclog.exe ./cmd/vrclog

# 実行可能ファイルのテスト（Windowsで）
.\vrclog.exe tail --help
```

---

## チェックリスト

### Step 1: プロジェクト初期化
- [ ] `go mod init`
- [ ] ディレクトリ構造
- [ ] `.gitignore`
- [ ] 依存関係追加

### Step 2: 型定義とパーサー
- [ ] `pkg/vrclog/event.go`
- [ ] `pkg/vrclog/errors.go`
- [ ] `pkg/vrclog/doc.go`
- [ ] `internal/parser/patterns.go`
- [ ] `internal/parser/parser.go`
- [ ] `testdata/logs/sample.txt`
- [ ] `internal/parser/parser_test.go`
- [ ] `pkg/vrclog/parse.go`
- [ ] テスト: `go test ./internal/parser/`

### Step 3: ログファイル検出
- [ ] `internal/logfinder/finder.go`
- [ ] `internal/logfinder/finder_test.go`
- [ ] テスト: `go test ./internal/logfinder/`

### Step 4: ファイルtail
- [ ] `internal/tailer/tailer.go`

### Step 5: Watch API
- [ ] `pkg/vrclog/watch.go`
- [ ] `pkg/vrclog/vrclog_test.go`
- [ ] テスト: `go test ./pkg/vrclog/`

### Step 6: CLI
- [ ] `cmd/vrclog/main.go`
- [ ] `cmd/vrclog/tail.go`
- [ ] ビルド: `go build ./cmd/vrclog`

### Step 7: ドキュメント
- [ ] `README.md`
- [ ] `README.ja.md`

### 最終確認
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go build ./...`
- [ ] Windows でのビルド確認
