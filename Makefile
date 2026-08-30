BINARY := agent-motion
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOCACHE ?= $(CURDIR)/.cache/go-build

build:
	GOCACHE=$(GOCACHE) go build -buildvcs=false -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-motion

# Renders every evaluation scenario.
fixtures:
	@$(MAKE) fixture SCENARIO=reference
	@$(MAKE) fixture SCENARIO=defect
	@$(MAKE) fixture SCENARIO=player

test:
	GOCACHE=$(GOCACHE) go test ./... -count=1

test-short:
	GOCACHE=$(GOCACHE) go test ./... -count=1 -short

# Renders an evaluation scenario. Needs FFmpeg. SCENARIO=reference|defect.
SCENARIO ?= reference
fixture:
	@mkdir -p .cache/eval
	GOCACHE=$(GOCACHE) go run ./tools/genfixture -scenario $(SCENARIO) \
		-video .cache/eval/$(SCENARIO).mp4 -truth .cache/eval/$(SCENARIO)-truth.json

vet:
	GOCACHE=$(GOCACHE) go vet ./...

tidy:
	GOCACHE=$(GOCACHE) go mod tidy

lint:
	golangci-lint run ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

dev:
	GOCACHE=$(GOCACHE) go run ./cmd/agent-motion $(ARGS)

clean:
	rm -f $(BINARY)
	rm -rf dist/

.PHONY: build test test-short fixture fixtures vet tidy lint fmt dev clean
