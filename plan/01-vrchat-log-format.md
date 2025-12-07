# VRChat ログフォーマット詳細

## ログファイルの場所

### デフォルトパス

```
C:\Users\<Username>\AppData\LocalLow\VRChat\VRChat\output_log_*.txt
C:\Users\<Username>\AppData\LocalLow\VRChat\vrchat\output_log_*.txt
```

### ファイル名パターン

```
output_log_2024-01-15_23-59-59.txt
```

- 形式: `output_log_YYYY-MM-DD_HH-MM-SS.txt`
- VRChat起動ごとに新しいファイルが作成される
- 最新ファイルの判定: ファイルの更新時刻で比較（ファイル名より堅牢）

## ログ行の基本構造

### 形式

```
YYYY.MM.DD HH:MM:SS <LogLevel> - [Category] Message
```

### 例

```
2024.01.15 23:59:59 Log        - [Behaviour] OnPlayerJoined TestUser
2024.01.15 23:59:59 Warning    - [Network] Connection timeout
2024.01.15 23:59:59 Error      - [Avatar] Failed to load avatar
2024.01.15 23:59:59 Exception  - NullReferenceException: ...
```

### タイムスタンプ

- **位置**: 行の先頭19文字
- **形式**: `yyyy.MM.dd HH:mm:ss` (24時間制)
- **パース形式** (Go): `2006.01.02 15:04:05`
- **タイムゾーン**: ローカル時間（VRCXではUTCに変換している）

### ログレベル

| レベル | 説明 |
|--------|------|
| Log | 通常のログ |
| Warning | 警告 |
| Error | エラー |
| Exception | 例外 |

### カテゴリ

- `[Behaviour]` - ゲーム内イベント（最も重要）
- `[Network]` - ネットワーク関連
- `[Avatar]` - アバター関連
- `[Video Playback]` - ビデオプレイヤー
- `[VRC Camera]` - カメラ・スクリーンショット
- `[Player]` - プレイヤー初期化

## イベントパターン詳細

### プレイヤー参加 (PlayerJoin)

```
2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined DisplayName
2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined DisplayName (usr_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
```

**パースのポイント**:
- `OnPlayerJoined` の後にプレイヤー名
- オプションでUserID（`usr_`で始まる）が括弧内に含まれる
- `OnPlayerJoined:` を含む行は除外（別のログ）

**正規表現**:
```go
`\[Behaviour\] OnPlayerJoined (.+?)(?:\s+\((usr_[a-f0-9-]+)\))?$`
```

**補足イベント**（同時に出力される）:
```
[Behaviour] Initialized PlayerAPI "DisplayName" is remote
[Behaviour] OnPlayerJoinComplete DisplayName
```

### プレイヤー退出 (PlayerLeft)

```
2024.01.15 23:59:59 Log - [Behaviour] OnPlayerLeft DisplayName
```

**パースのポイント**:
- `OnPlayerLeft` の後にプレイヤー名
- `OnPlayerLeftRoom` を含む行は除外（自分が退出する場合）

**正規表現**:
```go
`\[Behaviour\] OnPlayerLeft (.+)$`
```

### ワールド入室 (WorldJoin)

#### 方法1: Entering Room

```
2024.01.15 23:59:59 Log - [Behaviour] Entering Room: World Name
```

**パースのポイント**:
- `Entering Room:` の後にワールド名
- ワールドIDは含まれない

**正規表現**:
```go
`\[Behaviour\] Entering Room: (.+)$`
```

#### 方法2: Joining (より詳細)

```
2024.01.15 23:59:59 Log - [Behaviour] Joining wrld_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx:12345~region(us)
```

**パースのポイント**:
- `Joining` の後にワールドID:インスタンスID
- `Joining or Creating` は除外
- `Joining friend` は除外

**正規表現**:
```go
`\[Behaviour\] Joining (wrld_[a-f0-9-]+):(.+)$`
```

### インスタンスID形式

```
12345~region(us)
12345~private(usr_xxx)~nonce(xxx)
12345~friends(usr_xxx)~nonce(xxx)
12345~hidden(usr_xxx)~nonce(xxx)
```

**構成要素**:
- 数字部分: インスタンス番号
- `~region(xx)`: リージョン（us, eu, jp など）
- `~private/friends/hidden`: アクセスタイプ
- `~nonce(xxx)`: 一意識別子

## その他の検出可能なイベント（将来拡張用）

### 退室

```
[Behaviour] OnLeftRoom
```

### スクリーンショット

```
[VRC Camera] Took screenshot to: C:\Users\xxx\Pictures\VRChat\xxx.png
```

### アバター変更

```
[Behaviour] Switching to avatar avtr_xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

### ビデオプレイヤー

```
[Video Playback] URL 'https://...' resolved to 'https://...'
[AVProVideo] Opening https://...
```

### ポータル生成

```
[Behaviour] Instantiated a (Clone) Portals/PortalInternalDynamic
```

### VRCX専用イベント

```
[VRCX] ...
```
ワールド内のVRCXギミックから出力されるカスタムイベント。

## ログの特殊ケース

### 複数行ログ

例外やスタックトレースは複数行にわたる:
```
2024.01.15 23:59:59 Exception - NullReferenceException: Object reference not set
  at VRC.Something.Method()
  at VRC.Other.Method()
```

### 空行

ログ内に空行が含まれることがある。

### エンコーディング

UTF-8（BOMなし）。日本語のワールド名やプレイヤー名も含まれる。

## VRChat バージョンによる変更履歴

### 2018年頃の変更

- `OnPlayerJoined` / `OnPlayerLeft` がログから削除された時期があった
- 代わりに `[Player] Initialized PlayerAPI "USERNAME" is remote` が追加
- 現在は両方が出力される

### 現在（2024年）

- `OnPlayerJoined` が復活
- 両方のイベントが出力される
- VRCXは両方に対応

## 参考リンク

- [VRChat公式ヘルプ - ログファイルの場所](https://help.vrchat.com/hc/en-us/articles/9521522810899)
- [VRCX LogWatcher.cs](https://github.com/vrcx-team/VRCX/blob/master/Dotnet/LogWatcher.cs)
- [VRChatログ解説記事](https://note.com/kyanaru_vrc/n/nd46456751b59)
