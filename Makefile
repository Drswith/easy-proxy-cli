.PHONY: build install test test-unit test-e2e test-all test-docker fmt vet clean release

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

test: test-unit

test-unit:
	go test ./internal/... ./cmd/... -count=1

test-e2e:
	go test ./e2e/... -count=1 -timeout 5m

test-all: vet test-unit test-e2e

test-docker:
	chmod +x scripts/test-docker.sh
	./scripts/test-docker.sh all

fmt:
	gofmt -w ./cmd ./internal ./e2e
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
	cd dist && sha256sum $(BINARY)-* > checksums.txt || shasum -a 256 $(BINARY)-* > checksums.txt
