BINARY ?= toggle-display
PREFIX ?= $(HOME)/.local/bin

.PHONY: build install test clean

build:
	go build -o $(BINARY) ./cmd/toggle-display

install:
	mkdir -p $(PREFIX)
	go build -o $(PREFIX)/$(BINARY) ./cmd/toggle-display

test:
	go test ./...

clean:
	rm -f $(BINARY)
