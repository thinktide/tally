VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X github.com/thinktide/tally/internal/cli.Version=$(VERSION)"

.PHONY: all build install clean test lint

all: build

build:
	go build $(LDFLAGS) -o bin/tally ./cmd/tally

install:
	go install $(LDFLAGS) ./cmd/tally

clean:
	rm -rf bin/
	rm -rf dist/

test:
	go test -v ./...

lint:
	golangci-lint run

# Development helpers
.PHONY: run dev

run: build
	./bin/tally

dev:
	go run $(LDFLAGS) ./cmd/tally

# Cross-compilation targets
.PHONY: build-all

build-all:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/tally-darwin-amd64 ./cmd/tally
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/tally-darwin-arm64 ./cmd/tally
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/tally-linux-amd64 ./cmd/tally

# Release (requires goreleaser)
.PHONY: release snapshot

release:
	goreleaser release --clean

snapshot:
	goreleaser release --snapshot --clean
