# ADR 0009 - WebAssembly Plugin System Architecture

## Status

Accepted

## Date

2026-01-16

## Context and Problem Statement

The existing parser extensibility options (YAML patterns via `RegexParser`, Go code via `Parser` interface) require either runtime compilation or application rebuild. Users need a way to extend log parsing without:

1. Modifying the core vrclog-go codebase
2. Recompiling the CLI
3. Writing YAML patterns (limited to simple regex matching)

The solution must be secure (untrusted plugin execution), performant (log parsing is latency-sensitive), and support multiple programming languages.

## Decision Drivers

- **Extensibility**: Support complex parsing logic beyond regex
- **Security**: Sandboxed execution for untrusted plugins
- **Performance**: Minimize per-call overhead while enforcing safety limits
- **Developer Experience**: Support Go (TinyGo), Rust, and other WASM-targeting languages
- **Portability**: Cross-platform support (Windows, macOS, Linux)
- **No CGO**: Avoid CGO dependencies for simpler builds and deployment

## Considered Options

### Runtime Options

1. **Lua scripting** - Simple embedding, but limited ecosystem
2. **JavaScript (V8/QuickJS)** - Popular but heavy dependency
3. **WebAssembly (wazero)** - Sandboxed, fast, multi-language support
4. **Go plugins** - OS-dependent, requires matching Go version
5. **gRPC/IPC** - Process isolation but complex deployment

### ABI Options

1. **JSON-based protocol** - Simple, debuggable, human-readable
2. **Protobuf** - Efficient but requires code generation
3. **FlatBuffers** - Zero-copy but complex
4. **Custom binary protocol** - Minimal overhead but error-prone

## Decision Outcome

Chose **WebAssembly with wazero runtime** and **JSON-based ABI**.

### Runtime: wazero v1.11.0

```go
import "github.com/tetratelabs/wazero"
```

**Rationale**:
- **Pure Go**: No CGO required, simplifies cross-compilation
- **WASI Support**: `wasi_snapshot_preview1` enables TinyGo, Rust, Zig plugins
- **AOT Compilation**: Pre-compiled modules with disk caching for fast startup
- **Deterministic**: Suitable for sandboxed, reproducible execution
- **Active Maintenance**: Well-maintained by Tetrate

### ABI: JSON Protocol (Version 1)

**Plugin Required Exports**:
| Export | Signature | Description |
|--------|-----------|-------------|
| `abi_version` | `() -> i32` | Must return `1` |
| `alloc` | `(size: u32) -> u32` | Allocate memory for output |
| `free` | `(ptr: u32, len: u32)` | Free allocated memory |
| `parse_line` | `(ptr: u32, len: u32) -> u64` | Parse input, return `(out_len << 32) \| out_ptr` |

**Memory Layout**:
| Region | Offset | Size | Description |
|--------|--------|------|-------------|
| INPUT_REGION | `0x10000` (64KB) | 8KB | Host writes input JSON here |
| Plugin heap | `0x20000`+ | Variable | Plugin's own heap space |

**JSON Input/Output**:
```go
// Input (written by host to INPUT_REGION)
type inputData struct {
    Line string `json:"line"`
}

// Output (returned by plugin via alloc'd memory)
type outputData struct {
    Ok     bool          `json:"ok"`
    Events []event.Event `json:"events"`
    Error  *string       `json:"error,omitempty"`
    Code   *string       `json:"code,omitempty"`
}
```

**Rationale for JSON**:
- Human-readable for debugging
- Language-agnostic (no code generation)
- Schema evolution via optional fields

### Host Functions

Plugins import these functions from the `"env"` module:

| Function | Signature | Description |
|----------|-----------|-------------|
| `regex_match` | `(str_ptr, str_len, re_ptr, re_len) -> i32` | Returns 1 if match, 0 otherwise |
| `regex_find_submatch` | `(str_ptr, str_len, re_ptr, re_len, out_ptr, out_max) -> i32` | Returns bytes written, 0 if no match, 0xFFFFFFFF if buffer too small |
| `log` | `(level, ptr, len)` | Levels: 0=debug, 1=info, 2=warn, 3=error |
| `now_ms` | `() -> i64` | Returns Unix time in milliseconds |

**Rationale**:
- Regex in host is faster than WASM regex implementations
- Shared LRU regex cache (100 patterns, thread-safe with `sync.RWMutex`)
- Rate-limited logging prevents log spam
- `now_ms()` provides wall clock without complex WASI clock imports

### Security Measures

| Protection | Limit | Implementation |
|------------|-------|----------------|
| WASM file size | 10MB | `os.Stat()` check before reading |
| Input size | 8KB | `INPUT_REGION_SIZE` constant |
| Execution timeout | 50ms (default, configurable) | `context.WithTimeout()` |
| Regex pattern length | 512 bytes | Checked in `regexCache.Get()` |
| Regex execution timeout | 5ms | Goroutine with select/timeout |
| Log rate limit | 10/sec | `golang.org/x/time/rate.Limiter` |
| Log message size | 256 bytes | Truncated with "[truncated]" suffix |
| WASI sandboxing | No preopens | No filesystem/network access provided |
| UTF-8 sanitization | All log messages | `strings.ToValidUTF8()` |

**WASI Sandboxing Note**: The implementation uses `wasi_snapshot_preview1.Instantiate()` but does not configure preopened directories or network capabilities. This provides implicit sandboxing - WASI filesystem/network functions will fail if called by plugins.

### Compilation Cache

WASM modules are AOT-compiled and cached to disk:

- Location: `$XDG_CACHE_HOME/vrclog/wasm` or `~/.cache/vrclog/wasm`
- Permissions: `0700` (user-only access)
- XDG Base Directory Specification compliant

### Error Types

```go
// Sentinel errors
var (
    ErrInvalidWasm        = errors.New("invalid wasm file")
    ErrABIVersionMismatch = errors.New("abi version mismatch")
    ErrMissingExport      = errors.New("missing required export function")
    ErrPluginPanic        = errors.New("plugin panicked")
    ErrTimeout            = errors.New("plugin timeout")
    ErrFileTooLarge       = errors.New("wasm file too large")
)

// Structured errors
type ABIError struct { Function, Reason string }
type PluginError struct { Code, Message string }
type WasmRuntimeError struct { Operation string; Err error }
```

### Resource Management

`WasmParser` implements both `vrclog.Parser` and `io.Closer`:

```go
type WasmParser struct {
    compiled      *CompiledWasm
    timeout       atomic.Int64      // Timeout in nanoseconds (thread-safe)
    moduleCounter atomic.Uint64     // For unique module names (see ADR 0008)
    // ... (logger, abiVersion omitted for brevity)
}

func (p *WasmParser) Close() error              // Safe to call multiple times
func (p *WasmParser) SetTimeout(d time.Duration) // Thread-safe
```

`ParserChain.Close()` propagates to all closeable parsers.

### Context Cancellation

Following the pattern established in ADR 0008:
- `context.DeadlineExceeded` → converted to `ErrTimeout`
- `context.Canceled` → returned directly (not wrapped)
- `defer mod.Close(context.Background())` for cleanup

### TinyGo Plugin Development

Recommended build flags:
```bash
tinygo build -target=wasi -no-debug -scheduler=none -gc=leaking -o plugin.wasm
```

## Consequences

### Positive

- **Language flexibility**: Plugins can be written in TinyGo, Rust, Zig, AssemblyScript
- **Security**: WASI sandbox prevents filesystem/network access
- **Performance**: AOT compilation + disk caching enables fast loading
- **Deployment**: Single binary + WASM files, no runtime dependencies
- **Debugging**: JSON protocol is human-readable

### Negative

- **Plugin size**: TinyGo binaries are ~100KB-1MB (includes runtime)
- **Toolchain requirement**: TinyGo or Rust required for plugin development
- **Debugging complexity**: WASM stack traces less readable than native
- **Memory overhead**: Each ParseLine creates new module instance (see ADR 0008)

### Performance Characteristics

| Operation | Typical Time |
|-----------|--------------|
| Module instantiation | ~100μs |
| JSON parse/serialize | ~50μs |
| Regex (cache hit) | ~10μs |
| Total ParseLine overhead | ~200-500μs |

Note: Default timeout is 50ms, providing safety margin for complex parsing logic.

## More Information

- wazero documentation: https://wazero.io/
- WASI specification: https://wasi.dev/
- Example plugin: `examples/plugins/vrpoker/`
- Implementation: `internal/wasm/`
- Related ADR: ADR 0008 (WasmParser Concurrent Execution Safety)
- CLI integration: `cmd/vrclog/parser.go` (`buildParser()` function)
