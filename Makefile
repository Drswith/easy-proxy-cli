.PHONY: build install test fmt vet clean

BINARY := ezp
OUT := bin/$(BINARY)
VERSION ?= 0.1.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X github.com/drswith/easy-proxy-cli/cmd.version=$(VERSION) -X github.com/drswith/easy-proxy-cli/cmd.commit=$(COMMIT)"

build:
	mkdir -p bin
	go build $(LDFLAGS) -o $(OUT) .

install:
	go install $(LDFLAGS) .

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin

# Cross-compile release artifacts
release:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .
