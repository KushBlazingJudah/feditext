TAGS ?= sqlite3
GOFLAGS = -tags="$(TAGS)"
CGO_ENABLED = 1

all: build

build:
	go build $(GOFLAGS) -ldflags="-w -s" ./cmd/feditext

dev:
	go build $(GOFLAGS) ./cmd/feditext

run: dev
	./feditext

tidy:
	go clean
	go mod tidy
	go fmt ./...
	if command -v goimports >/dev/null 2>&1; then goimports -w .; fi

check:
	if command -v staticcheck >/dev/null 2>&1; then staticcheck -tags="$(TAGS)" ./...; fi
	go test $(GOFLAGS) -cover ./...

dist: build
	tar -c ./feditext ./views ./static | gzip -c > feditext.tar.gz

.PHONY: run tidy dist
