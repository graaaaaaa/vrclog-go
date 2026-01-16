.PHONY: all clean

TINYGO := tinygo
TINYGO_FLAGS := -target=wasi -no-debug -scheduler=none -gc=leaking

all: vrpoker.wasm

vrpoker.wasm: main.go
	$(TINYGO) build $(TINYGO_FLAGS) -o $@ $<

clean:
	rm -f vrpoker.wasm
