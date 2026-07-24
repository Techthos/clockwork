GO ?= go
BINARY := clockwork

.PHONY: build test lint clean install inspect

# Static binary under ./bin/ (see .claude/rules/makefile-rules.md).
build:
	CGO_ENABLED=0 $(GO) build -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	$(GO) test ./... -race -cover

lint:
	golangci-lint run

install:
	CGO_ENABLED=0 $(GO) install ./cmd/$(BINARY)

# Inspect MCP tools and MCP-Apps UI widgets (see .claude/rules/makefile-rules.md).
inspect: build
	npx @modelcontextprotocol/inspector ./bin/$(BINARY) mcp

clean:
	rm -rf bin
