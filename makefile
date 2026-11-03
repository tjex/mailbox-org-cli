VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: test build install

test:
	go test ./...

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION)" -o bin/mailbox-org-cli .

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 755 bin/mailbox-org-cli "$(DESTDIR)$(BINDIR)/mailbox-org-cli"
