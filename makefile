VERSION := $(shell if test -n "$$(git status --porcelain)"; then printf dirty; else git describe --tags --exact-match 2>/dev/null || git rev-parse --short HEAD; fi)
GOPATH ?= $(shell go env GOPATH)
BINDIR ?= $(GOPATH)/bin

.PHONY: test build install

test:
	go test ./...

build:
	mkdir -p bin
	go build -ldflags "-X main.version=$(VERSION)" -o bin/mailbox-org-cli .

install: build
	install -d "$(DESTDIR)$(BINDIR)"
	install -m 755 bin/mailbox-org-cli "$(DESTDIR)$(BINDIR)/mailbox-org-cli"
