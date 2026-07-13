.PHONY: build install test fmt vet clean release

BINARY := ezp
OUT := bin/$(BINARY)
VERSION ?= 0.1.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X github.com/drswith/easy-proxy-cli/cmd.version=$(VERSION) -X github.com/drswith/easy-proxy-cli/cmd.commit=$(COMMIT)"

build:
	mkdir -p bin
	go build $(LDFLAGS) -o $(OUT) ./cmd/ezp

install:
	go install $(LDFLAGS) ./cmd/ezp

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin dist

# Cross-compile release artifacts (consumed by scripts/install.sh / install.ps1)
release:
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 ./cmd/ezp
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 ./cmd/ezp
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 ./cmd/ezp
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 ./cmd/ezp
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/ezp
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-arm64.exe ./cmd/ezp
