BINARY := agent-motion
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOCACHE ?= $(CURDIR)/.cache/go-build

build:
	GOCACHE=$(GOCACHE) go build -buildvcs=false -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-motion

test:
	GOCACHE=$(GOCACHE) go test ./... -count=1

test-short:
	GOCACHE=$(GOCACHE) go test ./... -count=1 -short

# Renders the reference scenario used for evaluation. Needs FFmpeg.
fixture:
	@mkdir -p .cache/eval
	GOCACHE=$(GOCACHE) go run ./tools/genfixture \
		-video .cache/eval/reference.mp4 -truth .cache/eval/reference-truth.json

vet:
	GOCACHE=$(GOCACHE) go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

dev:
	GOCACHE=$(GOCACHE) go run ./cmd/agent-motion $(ARGS)

clean:
	rm -f $(BINARY)
	rm -rf dist/

.PHONY: build test test-short fixture vet lint fmt dev clean
