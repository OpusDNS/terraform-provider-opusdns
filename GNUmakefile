default: build

build:
	go build ./...

install:
	go install ./...

test:
	go test ./... -v -count=1 -timeout 120s

testacc:
	TF_ACC=1 go test ./internal/provider/ -v -count=1 -parallel=4 -timeout 30m

vet:
	go vet ./...

fmt:
	gofmt -s -w .

generate-docs:
	tfplugindocs generate --provider-name opusdns

.PHONY: build install test testacc vet fmt generate-docs
