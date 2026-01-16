# ADR 0008 - WasmParser Concurrent Execution Safety

## Status

Accepted

## Date

2026-01-16

## Context and Problem Statement

`WasmParser.ParseLine()` は goroutine-safe であることがドキュメントで宣言されているが、
実装批判的レビューで以下の問題が発見された：

### 問題1: wazeroモジュール名衝突

wazero の `Runtime.InstantiateModule()` は、Runtime 内でモジュール名の一意性を要求する。
元のコードでは固定名 `"plugin"` を使用していたため、並行呼び出し時に2つ目以降が失敗する：

```go
// Before: 並行呼び出しで失敗
modConfig := wazero.NewModuleConfig().WithName("plugin")
mod, err := p.compiled.runtime.InstantiateModule(ctx, p.compiled.compiled, modConfig)
```

### 問題2: キャンセル済みcontextでのClose

タイムアウト発生時、`ctx` はキャンセル済みだが `defer mod.Close(ctx)` でそのまま使用していた。
`loader.go` のエラーパスでは `context.Background()` を使用しており、一貫性がなかった。

### 問題3: context.Canceledの未処理

ユーザーが明示的にコンテキストをキャンセルした場合、`context.Canceled` が
`WasmRuntimeError` でラップされて返され、`errors.Is(err, context.Canceled)` で
検出できなかった。

## Decision Drivers

- **Goroutine安全性**: `ParseLine()` は並行呼び出しで安全でなければならない
- **パフォーマンス**: 追加オーバーヘッドを最小限に抑える
- **一貫性**: loader.go のパターンと整合性を保つ
- **エラー透過性**: contextエラーはラップせず直接返す

## Considered Options

### モジュール名衝突の解決

1. **atomic.Uint64 カウンター** - インクリメンタルな一意名生成
2. **UUID** - ランダムな一意名生成
3. **sync.Mutex** - 排他制御でインスタンスを再利用
4. **モジュールプール** - 事前に複数インスタンスを作成

### contextエラー処理

1. **ctx.Err() を直接返す** - errors.Is() で検出可能
2. **カスタムエラーでラップ** - 追加情報を付与
3. **すべて ErrTimeout で返す** - 簡略化

## Decision Outcome

### 解決策1: atomic.Uint64 カウンター

```go
type WasmParser struct {
    // ...
    moduleCounter atomic.Uint64 // Counter for unique module names
}

func (p *WasmParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
    name := fmt.Sprintf("plugin-%d", p.moduleCounter.Add(1))
    modConfig := wazero.NewModuleConfig().WithName(name)
    // ...
}
```

**選択理由**:
- UUIDより高速（crypto/rand不要）
- オーバーヘッドは atomic 操作1回 + fmt.Sprintf のみ
- uint64 のオーバーフローは実質的に到達不可能（毎秒100万回で58万年）
- sync.Mutex は不要な排他制御を追加してしまう

### 解決策2: context.Background() でClose

```go
defer mod.Close(context.Background())
```

**選択理由**:
- loader.go のエラーパスと一貫性がある
- wazero の Close() はキャンセル済みcontextでも動作するが、明示的に安全側に倒す
- Close 操作にタイムアウトは不要

### 解決策3: ctx.Err() を直接返す

```go
if err != nil {
    if ctx.Err() != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return vrclog.ParseResult{}, ErrTimeout
        }
        return vrclog.ParseResult{}, ctx.Err()  // context.Canceled
    }
    return vrclog.ParseResult{}, &WasmRuntimeError{...}
}
```

**選択理由**:
- `errors.Is(err, context.Canceled)` で検出可能
- 標準ライブラリの慣例に従う
- DeadlineExceeded のみ ErrTimeout に変換（ドメイン固有エラー）

## Consequences

### Positive

- **Goroutine安全性**: 並行呼び出しが安全に動作する
- **テスト可能**: `TestParseLine_Concurrent` で100並行呼び出しをテスト済み
- **一貫性**: loader.go のパターンと整合
- **エラー透過性**: contextエラーが正しく伝播

### Negative

- **メモリオーバーヘッド**: モジュール名文字列（"plugin-123"）の生成
- **wazero依存**: この設計は wazero v1.11.0 のモジュール名一意性仕様に依存（将来のバージョンで変更の可能性あり）

### Performance Impact

- atomic.Uint64.Add(): ~1ns
- fmt.Sprintf(): ~100ns
- 合計: ParseLine 全体（~1ms）の0.01%未満

## More Information

- wazero ドキュメント: https://wazero.io/
- wazero バージョン: v1.11.0
- 関連テスト: `internal/wasm/parser_test.go` の `TestParseLine_Concurrent`
- 実装ファイル: `internal/wasm/parser.go`
- 関連ADR: なし（Phase 2 新規実装）
