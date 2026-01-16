package wasm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// skipIfNoWasm はテスト用Wasmがない場合にスキップ
func skipIfNoWasm(t *testing.T, wasmName string) string {
	t.Helper()
	wasmPath := filepath.Join("testdata", wasmName)
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skipf("Wasm file %s not found. Run 'make build-test-wasm' first.", wasmName)
	}
	return wasmPath
}

// ========================================================================
// Load関連テスト
// ========================================================================

func TestLoad_Success(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// abiVersionは同一パッケージからアクセス可能（unexported field）
	if parser.abiVersion != ExpectedABIVersion {
		t.Errorf("ABI version = %d, want %d", parser.abiVersion, ExpectedABIVersion)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	ctx := context.Background()
	_, err := Load(ctx, "testdata/nonexistent.wasm", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	// エラーメッセージにファイル関連のエラーが含まれることを確認
	if !strings.Contains(err.Error(), "failed to") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidWasm(t *testing.T) {
	// 一時ファイルに不正なWasmバイナリを書き込む
	tmpDir := t.TempDir()
	invalidWasm := filepath.Join(tmpDir, "invalid.wasm")
	if err := os.WriteFile(invalidWasm, []byte("not a wasm file"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := Load(ctx, invalidWasm, nil)
	if err == nil {
		t.Fatal("expected error for invalid wasm")
	}
}

// TestLoad_ABIVersionMismatch はスキップ
// 理由: ABIバージョン不一致のWasmを作成するにはテストプラグインの
// abi_version()関数を変更してビルドする必要があり、コストに見合わない。
// 実際のABI検証ロジックはLoad()のコードレビューで確認済み。

// ========================================================================
// ParseLine関連テスト
// ========================================================================

func TestParseLine_NoMatch(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	result, err := parser.ParseLine(ctx, "any line")
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	// minimal.wasmは常にマッチなしを返す
	if result.Matched {
		t.Error("expected Matched=false for minimal.wasm")
	}
	if len(result.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(result.Events))
	}
}

func TestParseLine_Match(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "echo.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	result, err := parser.ParseLine(ctx, "test input line")
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	// echo.wasmは常に1つのtest_echoイベントを返す
	if !result.Matched {
		t.Error("expected Matched=true for echo.wasm")
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].Type != "test_echo" {
		t.Errorf("event type = %q, want %q", result.Events[0].Type, "test_echo")
	}
}

func TestParseLine_Timeout(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "slow.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// 短いタイムアウトを設定
	parser.SetTimeout(10 * time.Millisecond)

	_, err = parser.ParseLine(ctx, "test line")
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestParseLine_HostFunctions(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "regex.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// regex.wasmは test_(\w+) パターンにマッチする
	// 入力 "test_hello" は "hello" をキャプチャ
	result, err := parser.ParseLine(ctx, "test_hello")
	if err != nil {
		t.Fatalf("ParseLine failed: %v", err)
	}

	if !result.Matched {
		t.Error("expected Matched=true for regex pattern match")
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].Type != "test_regex" {
		t.Errorf("event type = %q, want %q", result.Events[0].Type, "test_regex")
	}
}

func TestParseLine_EmptyInput(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// 空文字列でもエラーにならないことを確認
	result, err := parser.ParseLine(ctx, "")
	if err != nil {
		t.Fatalf("ParseLine failed for empty input: %v", err)
	}

	if result.Matched {
		t.Error("expected Matched=false for empty input")
	}
}

func TestParseLine_LargeInput(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// INPUT_REGION_SIZE (8192) を超える入力
	largeInput := strings.Repeat("a", INPUT_REGION_SIZE+100)
	_, err = parser.ParseLine(ctx, largeInput)
	if err == nil {
		t.Fatal("expected error for input exceeding INPUT_REGION_SIZE")
	}
	if !strings.Contains(err.Error(), "input too large") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseLine_MultiByte(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "echo.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// 日本語を含む入力
	result, err := parser.ParseLine(ctx, "プレイヤー 田中太郎 が参加しました")
	if err != nil {
		t.Fatalf("ParseLine failed for multibyte input: %v", err)
	}

	// echo.wasmは入力をそのまま返す
	if !result.Matched {
		t.Error("expected Matched=true")
	}
}

// ========================================================================
// Close関連テスト
// ========================================================================

func TestClose(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Close()がエラーなく完了することを確認
	if err := parser.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestClose_Multiple(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// 1回目のClose()
	if err := parser.Close(); err != nil {
		t.Errorf("first Close failed: %v", err)
	}
	// 2回目のClose()もエラーなく完了することを確認
	if err := parser.Close(); err != nil {
		t.Errorf("second Close failed: %v", err)
	}
}

// ========================================================================
// SetTimeout関連テスト
// ========================================================================

func TestSetTimeout(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	// デフォルトタイムアウトからの変更
	parser.SetTimeout(100 * time.Millisecond)

	// タイムアウトが変更されたことを確認（atomic.Int64から読み取り）
	got := time.Duration(parser.timeout.Load())
	want := 100 * time.Millisecond
	if got != want {
		t.Errorf("timeout = %v, want %v", got, want)
	}
}

// ========================================================================
// 並行性テスト
// ========================================================================

func TestParseLine_Concurrent(t *testing.T) {
	wasmPath := skipIfNoWasm(t, "minimal.wasm")

	ctx := context.Background()
	parser, err := Load(ctx, wasmPath, nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	defer parser.Close()

	var wg sync.WaitGroup
	errCh := make(chan error, 100) // errorsパッケージとの名前衝突を回避

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := parser.ParseLine(ctx, fmt.Sprintf("line %d", n))
			if err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}
