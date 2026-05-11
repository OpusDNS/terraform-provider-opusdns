default: build

build:
	go build -v ./...

test:
	go test -v ./...

install: build
	go install ./...

.PHONY: build test install
