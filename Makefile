TAGS ?= sqlite3
GOFLAGS = -tags "$(TAGS)"

all: build

build:
	go build $(GOFLAGS) ./cmd/feditext

dev:
	go build -race $(GOFLAGS) ./cmd/feditext

run:
	go run -race $(GOFLAGS) ./cmd/feditext

tidy:
	go clean
	go mod tidy

check:
	go test ./...

.PHONY: run tidy
