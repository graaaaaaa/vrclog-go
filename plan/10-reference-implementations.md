# 参考実装・ライブラリ

## VRChatログパーサー実装

### VRCX (C#)

**リポジトリ**: https://github.com/vrcx-team/VRCX

**特徴**:
- 最も活発なVRChatコンパニオンツール
- C#/.NETで実装
- 包括的な機能（フレンド管理、ワールド履歴など）

**参考になる点**:
- `Dotnet/LogWatcher.cs` - ログパースの実装
- イベント検出パターン
- 除外パターン（OnPlayerLeftRoom, Joining or Creating など）

**ログパースパターン（VRCXより抽出）**:

```csharp
// プレイヤー参加
"[Behaviour] OnPlayerJoined" && !line.Contains("] OnPlayerJoined:")

// プレイヤー退出
"[Behaviour] OnPlayerLeft" && !line.Contains("] OnPlayerLeftRoom")

// ワールド入室
"[Behaviour] Entering Room: "

// インスタンス参加
"[Behaviour] Joining " // 除外: "Joining or Creating", "Joining friend"

// タイムスタンプ形式
"yyyy.MM.dd HH:mm:ss"
```

---

### vrc-tail (JavaScript/TypeScript)

**リポジトリ**: https://github.com/Narazaka/vrc-tail

**特徴**:
- VRChat ログの tail -f 実装
- 複数ログファイルの監視
- ログファイル選択アルゴリズム

**参考になる点**:
- 30秒以内に作成されたログファイルの選択
- カラー出力

---

### vrc-log-relay-server (TypeScript)

**リポジトリ**: https://github.com/kurotori4423/vrc-log-relay-server

**特徴**:
- vrc-tail のアルゴリズムを参考に実装
- WebSocket でイベントを中継
- Web UI付き

**参考になる点**:
- 設定ファイル構造
- イベント構造体の設計

---

### XSOverlay-VRChat-Parser (C#)

**リポジトリ**: https://github.com/nnaaa-vr/XSOverlay-VRChat-Parser

**特徴**:
- XSOverlay との連携
- 通知表示

**検出イベント**:
- Player Joined
- Player Left
- Portal Dropped
- World Changed
- Shader Keywords Exceeded

---

### VRChat-Log-Monitor (Python)

**リポジトリ**: https://github.com/Kavex/VRChat-Log-Monitor

**特徴**:
- Python での実装
- Discord 連携
- カスタマイズ可能なイベント設定

**参考になる点**:
- イベント設定ファイルの構造
- Discord Webhook 連携

---

### vrchat-log-rs (Rust)

**リポジトリ**: https://github.com/sksat/vrchat-log-rs

**特徴**:
- Rust での実装
- ライブラリとして使用可能

**参考になる点**:
- ログ行のパース構造
- タイムスタンプ抽出（最初の19文字）
- ログ種別の判定（Log, Warning, Error, Exception）

---

## Go ライブラリ

### spf13/cobra

**リポジトリ**: https://github.com/spf13/cobra
**ドキュメント**: https://pkg.go.dev/github.com/spf13/cobra

**用途**: CLI フレームワーク

**採用理由**:
- サブコマンド構造のサポート
- 自動ヘルプ生成
- シェル補完生成
- kubectl, gh, hugo などで採用

**使用パターン**:

```go
var rootCmd = &cobra.Command{
    Use:   "vrclog",
    Short: "VRChat log parser and monitor",
}

var tailCmd = &cobra.Command{
    Use:   "tail",
    Short: "Monitor VRChat logs",
    RunE:  runTail,
}

func init() {
    rootCmd.AddCommand(tailCmd)
    tailCmd.Flags().StringVarP(&logDir, "log-dir", "d", "", "Log directory")
}
```

---

### nxadm/tail

**リポジトリ**: https://github.com/nxadm/tail
**ドキュメント**: https://pkg.go.dev/github.com/nxadm/tail

**用途**: ファイル tail 実装

**採用理由**:
- クロスプラットフォーム（Windows 対応）
- ログローテーション対応（ReOpen）
- ファイル位置追跡（Tell）
- 実績あり（hpcloud/tail のフォーク）

**主要な設定**:

```go
tail.Config{
    Follow:    true,   // tail -f: ファイル成長を追跡
    ReOpen:    true,   // tail -F: ファイル再作成を追跡
    Poll:      false,  // inotify/ReadDirectoryChangesW を使用
    MustExist: true,   // ファイルが存在しない場合はエラー
    Location:  &tail.SeekInfo{
        Offset: 0,
        Whence: 2,     // io.SeekEnd: ファイル末尾から開始
    },
}
```

---

### fsnotify/fsnotify

**リポジトリ**: https://github.com/fsnotify/fsnotify
**ドキュメント**: https://pkg.go.dev/github.com/fsnotify/fsnotify

**用途**: ファイルシステム監視

**注意点**:
- nxadm/tail が内部で使用
- Windows では ReadDirectoryChangesW を使用
- NFS/SMB では動作しない
- 個別ファイル監視は推奨されない（ディレクトリを監視すべき）

**直接使用しない理由**:
- nxadm/tail がより高レベルの抽象化を提供
- ログローテーション対応が既に実装済み

---

## ドキュメント・ガイド

### Google Go Style Guide

**URL**: https://google.github.io/styleguide/go/

**主なトピック**:
- Best practices
- Style decisions
- Core naming
- Documentation
- Error handling

---

### Effective Go

**URL**: https://go.dev/doc/effective_go

**主なトピック**:
- Formatting
- Commentary
- Names
- Control structures
- Data
- Initialization
- Methods
- Interfaces
- Concurrency
- Errors
- A web server

---

### Go Wiki

**URL**: https://go.dev/wiki/

**主なページ**:
- TableDrivenTests
- CodeReviewComments
- CommonMistakes

---

### Dave Cheney's Blog

**URL**: https://dave.cheney.net/

**主な記事**:
- "Don't just check errors, handle them gracefully"
- "Prefer table driven tests"
- "What is the zero value, and why is it useful?"

---

## VRChat 公式情報

### ログファイルの場所

**URL**: https://help.vrchat.com/hc/en-us/articles/9521522810899

**内容**:
- Windows: `%LOCALAPPDATA%Low\VRChat\vrchat\`
- ログファイル名: `output_log_*.txt`

---

### VRChat Wiki (非公式)

**URL**: http://vrchat.wikidot.com/worlds:guides:log

**内容**:
- ログの種類
- イベントの説明
- デバッグ方法

---

## コミュニティ記事

### VRChatのログファイルで情報を得る

**URL**: https://note.com/kyanaru_vrc/n/nd46456751b59

**内容**:
- ログファイルの場所
- イベントパターンの説明
- 正規表現の例

---

## まとめ

### 必須参照

1. **VRCX LogWatcher.cs** - ログパースパターンの正確な実装
2. **Google Go Style Guide** - コーディング規約
3. **nxadm/tail ドキュメント** - tail 実装の詳細

### 補足参照

1. **vrc-tail** - ファイル選択アルゴリズム
2. **vrchat-log-rs** - 別言語での実装例
3. **VRChat公式ヘルプ** - ログファイルの場所

### 設計の注意点

1. VRChat のログフォーマットは変更される可能性がある
2. VRCXなど活発なプロジェクトの更新を追跡
3. 正規表現パターンはハードコードせず、更新しやすい構造に
