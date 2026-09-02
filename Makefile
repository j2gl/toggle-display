BINARY ?= toggle-display
PREFIX ?= $(HOME)/.local/bin

.PHONY: build install install-completion test clean

build:
	go build -o $(BINARY) ./cmd/toggle-display

install:
	mkdir -p "$(PREFIX)"
	go build -o "$(PREFIX)/$(BINARY)" ./cmd/toggle-display

ZSH_COMPLETION_DIR ?= $(HOME)/.zfunc

install-completion: install
	mkdir -p "$(ZSH_COMPLETION_DIR)"
	"$(PREFIX)/$(BINARY)" --completion zsh > "$(ZSH_COMPLETION_DIR)/_toggle-display"
	@echo "Installed zsh completion to $(ZSH_COMPLETION_DIR)/_toggle-display"

test:
	go test ./...

clean:
	rm -f $(BINARY)
