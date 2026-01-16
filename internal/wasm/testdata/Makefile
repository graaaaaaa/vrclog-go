.PHONY: all clean

TINYGO := tinygo
TINYGO_FLAGS := -target=wasi -no-debug -scheduler=none -gc=leaking

TARGETS := minimal.wasm echo.wasm regex.wasm slow.wasm

all: $(TARGETS)

minimal.wasm: minimal/main.go
	$(TINYGO) build $(TINYGO_FLAGS) -o $@ $<

echo.wasm: echo/main.go
	$(TINYGO) build $(TINYGO_FLAGS) -o $@ $<

regex.wasm: regex/main.go
	$(TINYGO) build $(TINYGO_FLAGS) -o $@ $<

slow.wasm: slow/main.go
	$(TINYGO) build $(TINYGO_FLAGS) -o $@ $<

clean:
	rm -f $(TARGETS)
