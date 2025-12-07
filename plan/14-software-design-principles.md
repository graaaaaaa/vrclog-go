# ソフトウェア設計原則

## 参考資料

- [SOLID Design Principles Explained - DigitalOcean](https://www.digitalocean.com/community/conceptual-articles/s-o-l-i-d-the-first-five-principles-of-object-oriented-design)
- [Clean Architecture - LinkedIn](https://www.linkedin.com/pulse/clean-architecture-principles-practices-sustainable-ribeiro-da-silva-retvf)
- [KISS YAGNI DRY Principles - workat.tech](https://workat.tech/machine-coding/tutorial/software-design-principles-dry-yagni-eytrxfhz1fla)
- [Code Smells - Refactoring.guru](https://refactoring.guru/refactoring/smells)

---

## SOLID原則

### 1. Single Responsibility Principle (SRP) - 単一責任の原則

**定義**: モジュールは変更の理由が1つだけであるべき

**vrclog-goへの適用**:
```
pkg/vrclog/event.go      → Event型の定義のみ
pkg/vrclog/parse.go      → パース機能のみ
pkg/vrclog/watch.go      → 監視機能のみ
internal/parser/         → パースロジックのみ
internal/logfinder/      → ファイル検出のみ
internal/tailer/         → ファイル追跡のみ
```

### 2. Open/Closed Principle (OCP) - 開放閉鎖の原則

**定義**: 拡張に対して開いており、修正に対して閉じている

**vrclog-goへの適用**:
- 新しいイベントタイプの追加は `EventType` 定数の追加のみ
- パーサーパターンの追加は `patterns.go` への追加のみ
- 既存のコードを変更せずに機能拡張可能

```go
// 新しいイベントタイプを追加（既存コード変更なし）
const (
    EventWorldJoin  EventType = "world_join"
    EventPlayerJoin EventType = "player_join"
    EventPlayerLeft EventType = "player_left"
    // 将来: EventScreenshot, EventAvatarChange など追加可能
)
```

### 3. Liskov Substitution Principle (LSP) - リスコフの置換原則

**定義**: 派生型はその基底型と置換可能でなければならない

**vrclog-goへの適用**:
- Goにはクラス継承がないため、インターフェースで適用
- すべてのイベントが同じ `Event` 構造体を使用
- `io.Reader` / `io.Writer` などの標準インターフェースに準拠

### 4. Interface Segregation Principle (ISP) - インターフェース分離の原則

**定義**: クライアントは使用しないインターフェースに依存すべきでない

**vrclog-goへの適用**:
```go
// Good: 小さなインターフェース
type lineSource interface {
    Lines() <-chan string
}

// Bad: 大きすぎるインターフェース
type allInOne interface {
    Lines() <-chan string
    Errors() <-chan error
    Stop() error
    Tell() (int64, error)
    // ... たくさんのメソッド
}
```

### 5. Dependency Inversion Principle (DIP) - 依存性逆転の原則

**定義**: 高レベルモジュールは低レベルモジュールに依存すべきでない

**vrclog-goへの適用**:
```
pkg/vrclog （高レベル）
    ↓ 依存
internal/parser （低レベル）
internal/tailer （低レベル）
internal/logfinder （低レベル）

高レベルはインターフェース経由で低レベルを使用
低レベルの実装詳細は隠蔽
```

---

## KISS, YAGNI, DRY

### KISS (Keep It Simple, Stupid)

**原則**: 不必要な複雑さを避け、シンプルに保つ

**vrclog-goでの適用**:

```go
// Good: シンプル
func ParseLine(line string) (*Event, error) {
    return parser.Parse(line)
}

// Bad: 不必要に複雑
func ParseLine(line string, opts ...ParseOption) (*Event, error) {
    config := defaultConfig()
    for _, opt := range opts {
        opt(config)
    }
    return parser.ParseWithConfig(line, config)
}
```

**適用場面**:
- 初期実装は最もシンプルな形で
- 複雑さが必要になったら追加
- リファクタリング時に「もっとシンプルにできないか」を考える

### YAGNI (You Aren't Gonna Need It)

**原則**: 今必要でない機能は実装しない

**vrclog-goでの適用**:

| 実装する | 実装しない（将来必要になれば） |
|----------|------------------------------|
| 3つのイベントタイプ | スクリーンショット検出 |
| JSON Lines出力 | CSV/XML出力 |
| シンプルなWatch API | イベントフィルタリングAPI |
| `WatchOptions` 構造体 | Functional Options パターン |

**チェックリスト**:
- [ ] 今この機能は必要か？
- [ ] 実際のユースケースがあるか？
- [ ] 仮定に基づいていないか？

### DRY (Don't Repeat Yourself)

**原則**: 同じコードを繰り返し書かない

**vrclog-goでの適用**:

```go
// Good: 共通化
func parseTimestamp(line string) (time.Time, error) {
    match := timestampPattern.FindStringSubmatch(line)
    if match == nil {
        return time.Time{}, fmt.Errorf("no timestamp")
    }
    return time.ParseInLocation(timestampLayout, match[1], time.Local)
}

// Bad: 各パース関数でタイムスタンプ処理を繰り返す
func parsePlayerJoin(line string) *Event {
    // タイムスタンプ処理をここでも書く
}
func parsePlayerLeft(line string) *Event {
    // タイムスタンプ処理をここでも書く
}
```

**注意**: 過度なDRYは避ける
- 2回程度の重複なら許容
- 3回以上なら共通化を検討
- 無理な共通化は可読性を下げる

---

## 高凝集・低結合 (High Cohesion, Low Coupling)

### 高凝集 (High Cohesion)

**定義**: モジュール内の要素が密接に関連している

**vrclog-goでの適用**:

```
internal/parser/
├── parser.go      # パースロジック
├── patterns.go    # 正規表現パターン
└── parser_test.go # パーサーテスト

→ すべてパースに関連、他の責務を持たない
```

### 低結合 (Low Coupling)

**定義**: モジュール間の依存が少ない

**vrclog-goでの適用**:

```
pkg/vrclog
    ↓ (Event型のみ)
internal/parser
    ↓ (なし)
internal/logfinder

internal/tailer
    ↓ (外部ライブラリ)
github.com/nxadm/tail
```

**結合度を下げるテクニック**:
- インターフェースを使う
- 具体的な型より抽象的な型を使う
- 直接依存より依存性注入

---

## Clean Architecture / Hexagonal Architecture

### vrclog-goへの適用度

このプロジェクトは**小規模ライブラリ**なので、完全なClean Architectureは**過剰**。

**採用する要素**:
- 依存性の方向（外→内）
- コア（Event型、パースロジック）とインフラ（tailer）の分離

**採用しない要素**:
- Use Case層
- Repository パターン
- 複雑なレイヤー構造

### 簡略化されたレイヤー

```
┌─────────────────────────────────────────┐
│           cmd/vrclog (CLI)              │ ← インターフェース層
├─────────────────────────────────────────┤
│           pkg/vrclog (API)              │ ← アプリケーション層
├─────────────────────────────────────────┤
│  internal/parser  internal/logfinder    │ ← ドメイン層
├─────────────────────────────────────────┤
│          internal/tailer                │ ← インフラ層
│         (nxadm/tail)                    │
└─────────────────────────────────────────┘

依存の方向: 上から下へ（外から内へ）
```

---

## vrclog-goへの具体的適用

### 設計決定

| 原則 | 適用 | 理由 |
|------|------|------|
| SRP | パッケージ分離 | 各パッケージは1つの責務 |
| OCP | EventType定数 | 拡張時に既存コード変更不要 |
| ISP | 小さなインターフェース | 必要最小限のメソッド |
| DIP | internal使用 | 実装詳細の隠蔽 |
| KISS | シンプルなAPI | 3つの公開関数のみ |
| YAGNI | 最小機能 | 3イベント、2出力形式 |
| DRY | 共通関数 | parseTimestamp等 |

### アンチパターンの回避

| アンチパターン | 回避方法 |
|---------------|---------|
| God Object | パッケージ分離 |
| Premature Optimization | シンプルな実装から開始 |
| Feature Creep | 仕様書の範囲を守る |
| Spaghetti Code | 明確な依存関係 |
| Copy-Paste Programming | 共通関数の抽出 |

---

## コードレビューチェックリスト

### 設計原則

- [ ] 単一責任の原則に従っているか？
- [ ] 拡張に対して開いているか？
- [ ] インターフェースは小さいか？
- [ ] 依存の方向は正しいか？

### シンプルさ

- [ ] より簡単な方法はないか？
- [ ] 今必要な機能だけ実装しているか？
- [ ] 過度な抽象化をしていないか？

### 重複

- [ ] 同じコードが複数箇所にないか？
- [ ] 共通化すべきか、許容すべきか？

### 結合度

- [ ] モジュール間の依存は最小か？
- [ ] 具体型より抽象型を使っているか？
