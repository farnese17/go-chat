BINARY="gochat"
VERSION=`cat VERSION`

.PHONY: all deps build build-darwin build-linux

all: build

deps:
		@echo "Downloading dependencies..."
		go mod download

build: deps
		@echo "Building for current platform..."
		mkdir -p bin
		go build -o bin/$(BINARY) -ldflags "-X github.com/farnese17/chat/cli.Version=$(VERSION)" ./main.go

build-darwin: deps
		@echo "Building for macOS..."
		mkdir -p bin
		GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY)-darwin -ldflags "-X github.com/farnese17/chat/cli.Version=$(VERSION)" ./main.go

build-linux: deps
		@echo "Building for Linux..."
		mkdir -p bin
		GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux -ldflags "-X github.com/farnese17/chat/cli.Version=$(VERSION)" ./main.go
