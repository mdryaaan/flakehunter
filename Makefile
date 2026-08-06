BINARY  := flakehunter
PKG     := github.com/mdryaaan/flakehunter
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(PKG)/pkg/version.Version=$(VERSION) \
	-X $(PKG)/pkg/version.Commit=$(COMMIT) \
	-X $(PKG)/pkg/version.BuildDate=$(DATE)

.PHONY: build test cover eval eval-baseline lint fmt vet tidy install clean demo pipeline

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./main.go

test:
	go test ./... -cover

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

## eval runs the accuracy harness against a local Ollama model.
## Requires `ollama serve` and `ollama pull llama3`.
eval:
	go run main.go eval --provider ollama --model llama3 --verbose

## eval-baseline needs nothing installed: it scores the deterministic
## rule-based classifier, which is the floor an LLM has to beat.
eval-baseline:
	go run main.go eval --provider deterministic --verbose

lint:
	golangci-lint run

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

install:
	go install -ldflags "$(LDFLAGS)" ./...

clean:
	rm -rf bin/ coverage.out scan.json classified.json

## demo runs detection against the bundled fixtures — no token, no network.
demo:
	go run main.go scan --offline --fixtures ./testdata/fixtures

## pipeline runs the whole flow offline and prints a markdown report.
pipeline:
	go run main.go scan --offline --fixtures ./testdata/fixtures --output scan.json
	go run main.go classify --input scan.json --provider deterministic \
		--offline --fixtures ./testdata/fixtures --output classified.json
	go run main.go report --input classified.json --format markdown
