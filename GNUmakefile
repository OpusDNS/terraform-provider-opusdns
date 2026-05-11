default: build

build:
	go build ./...

install:
	go install ./...

test:
	go test ./... -v -count=1 -timeout 120s

vet:
	go vet ./...

fmt:
	gofmt -s -w .

.PHONY: build install test vet fmt
