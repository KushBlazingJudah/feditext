TAGS ?= sqlite3
GOFLAGS = -tags="$(TAGS)"
CGO_ENABLED = 1

all: build

build:
	go build $(GOFLAGS) -ldflags="-w -s" ./cmd/feditext

dev:
	go build -race $(GOFLAGS) ./cmd/feditext

run: dev
	./feditext

tidy:
	go clean
	go mod tidy

check:
	go test ./...

.PHONY: run tidy
