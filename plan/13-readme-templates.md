# README テンプレート

## README.md (English)

```markdown
# vrclog-go

A Go library and CLI for parsing and monitoring VRChat log files.

## Features

- Parse VRChat log files into structured events
- Real-time monitoring of log files
- Detect player joins, leaves, and world changes
- JSON Lines output for easy processing

## Installation

### As a Library

```bash
go get github.com/vrclog/vrclog-go/pkg/vrclog
```

### As a CLI Tool

```bash
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest
```

Or download pre-built binaries from the [Releases](https://github.com/vrclog/vrclog-go/releases) page.

## Usage

### CLI

```bash
# Monitor VRChat logs (auto-detect log directory)
vrclog tail

# Specify log directory
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# Filter specific event types
vrclog tail --types player_join,player_left

# Human-readable output
vrclog tail --format pretty

# Pipe to jq for filtering
vrclog tail | jq 'select(.player_name == "FriendName")'
```

### Library

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

    for {
        select {
        case event, ok := <-events:
            if !ok {
                return
            }
            switch event.Type {
            case vrclog.EventPlayerJoin:
                fmt.Printf("%s joined\n", event.PlayerName)
            case vrclog.EventPlayerLeft:
                fmt.Printf("%s left\n", event.PlayerName)
            case vrclog.EventWorldJoin:
                fmt.Printf("Joined world: %s\n", event.WorldName)
            }
        case err, ok := <-errs:
            if !ok {
                return
            }
            log.Printf("error: %v", err)
        }
    }
}
```

### Parsing a Single Line

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("parse error: %v", err)
} else if event != nil {
    fmt.Printf("Event: %s, Player: %s\n", event.Type, event.PlayerName)
}
```

## Event Types

| Type | Description |
|------|-------------|
| `world_join` | User joined a world/instance |
| `player_join` | Another player joined the instance |
| `player_left` | Another player left the instance |

## Output Format

### JSON Lines (default)

```json
{"type":"player_join","timestamp":"2024-01-15T23:59:59+09:00","player_name":"TestUser"}
{"type":"player_left","timestamp":"2024-01-16T00:00:05+09:00","player_name":"TestUser"}
```

### Pretty

```
[23:59:59] + TestUser joined
[00:00:05] - TestUser left
[00:01:00] > Joined world: Test World
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VRCLOG_LOGDIR` | Override VRChat log directory path |

## Platform Support

This tool is designed for Windows where VRChat PC runs. Log file paths are auto-detected from standard Windows locations:

- `%LOCALAPPDATA%Low\VRChat\VRChat\output_log_*.txt`
- `%LOCALAPPDATA%Low\VRChat\vrchat\output_log_*.txt`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Disclaimer

This is an unofficial tool and is not affiliated with, endorsed by, or connected to VRChat Inc. VRChat is a trademark of VRChat Inc.

Use this tool at your own risk. The log format may change without notice in future VRChat updates.
```

---

## README.ja.md (Japanese)

```markdown
# vrclog-go

VRChatのログファイルを解析・監視するためのGoライブラリとCLI。

> **Note**: この文書は参考訳です。正式な文書は英語版の [README.md](README.md) を参照してください。

## 機能

- VRChatログファイルを構造化されたイベントにパース
- ログファイルのリアルタイム監視
- プレイヤーの参加・退出、ワールド移動を検出
- 処理しやすいJSON Lines形式での出力

## インストール

### ライブラリとして

```bash
go get github.com/vrclog/vrclog-go/pkg/vrclog
```

### CLIツールとして

```bash
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest
```

または [Releases](https://github.com/vrclog/vrclog-go/releases) ページからビルド済みバイナリをダウンロード。

## 使い方

### CLI

```bash
# VRChatログを監視（ログディレクトリは自動検出）
vrclog tail

# ログディレクトリを指定
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# 特定のイベントタイプのみフィルタ
vrclog tail --types player_join,player_left

# 人間が読みやすい形式で出力
vrclog tail --format pretty

# jqと組み合わせてフィルタリング
vrclog tail | jq 'select(.player_name == "フレンド名")'
```

### ライブラリ

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    events, errs := vrclog.Watch(ctx, vrclog.WatchOptions{})

    for {
        select {
        case event, ok := <-events:
            if !ok {
                return
            }
            switch event.Type {
            case vrclog.EventPlayerJoin:
                fmt.Printf("%s が参加しました\n", event.PlayerName)
            case vrclog.EventPlayerLeft:
                fmt.Printf("%s が退出しました\n", event.PlayerName)
            case vrclog.EventWorldJoin:
                fmt.Printf("ワールドに入室: %s\n", event.WorldName)
            }
        case err, ok := <-errs:
            if !ok {
                return
            }
            log.Printf("エラー: %v", err)
        }
    }
}
```

### 単一行のパース

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined テストユーザー"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("パースエラー: %v", err)
} else if event != nil {
    fmt.Printf("イベント: %s, プレイヤー: %s\n", event.Type, event.PlayerName)
}
```

## イベントタイプ

| タイプ | 説明 |
|--------|------|
| `world_join` | ワールド/インスタンスに入室 |
| `player_join` | 他のプレイヤーがインスタンスに参加 |
| `player_left` | 他のプレイヤーがインスタンスから退出 |

## 出力形式

### JSON Lines（デフォルト）

```json
{"type":"player_join","timestamp":"2024-01-15T23:59:59+09:00","player_name":"テストユーザー"}
{"type":"player_left","timestamp":"2024-01-16T00:00:05+09:00","player_name":"テストユーザー"}
```

### Pretty

```
[23:59:59] + テストユーザー が参加しました
[00:00:05] - テストユーザー が退出しました
[00:01:00] > ワールドに入室: テストワールド
```

## 環境変数

| 変数 | 説明 |
|------|------|
| `VRCLOG_LOGDIR` | VRChatログディレクトリのパスを上書き |

## プラットフォームサポート

このツールはVRChat PCが動作するWindows向けに設計されています。ログファイルのパスはWindowsの標準的な場所から自動検出されます：

- `%LOCALAPPDATA%Low\VRChat\VRChat\output_log_*.txt`
- `%LOCALAPPDATA%Low\VRChat\vrchat\output_log_*.txt`

## コントリビューション

コントリビューションを歓迎します！お気軽にPull Requestを送ってください。

1. リポジトリをフォーク
2. フィーチャーブランチを作成 (`git checkout -b feature/amazing-feature`)
3. 変更をコミット (`git commit -m 'Add some amazing feature'`)
4. ブランチにプッシュ (`git push origin feature/amazing-feature`)
5. Pull Requestを開く

## ライセンス

MIT License - 詳細は [LICENSE](LICENSE) を参照してください。

## 免責事項

これは非公式ツールであり、VRChat Inc.とは関係がありません。VRChatはVRChat Inc.の商標です。

このツールは自己責任でご使用ください。ログ形式は将来のVRChatアップデートで予告なく変更される可能性があります。
```

---

## LICENSE

```
MIT License

Copyright (c) 2024 vrclog

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
