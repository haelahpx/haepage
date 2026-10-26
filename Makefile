BINARY  := haepage
PKG     := github.com/haelahpx/haepage/internal/version
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

.PHONY: all web build dev test lint bench clean

all: build

web/node_modules: web/package.json
	npm --prefix web install
	@touch $@

## web: build the frontend into web/dist (fails above the size budgets)
web: web/node_modules
	npm --prefix web run build

## build: build the frontend, then the static binary with it embedded
build: web
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/haepage

## dev: rebuild the frontend and run the server from disk without caching
dev: web
	go run ./cmd/haepage --dev --log-level debug

## test: Go and frontend tests
test: web/node_modules
	go test -race ./...
	npm --prefix web test

## lint: formatting, vet, and schema validation
lint: web/node_modules
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "run gofmt -w ." && exit 1)
	go vet ./...
	npm --prefix web run validate

## bench: idle memory and CPU of the built binary
bench: build
	tools/bench/bench.sh ./$(BINARY)

clean:
	rm -f $(BINARY)
	find web/dist -mindepth 1 ! -name .gitkeep -delete
