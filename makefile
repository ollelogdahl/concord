.PHONY: build

build:
	@go generate ./...
	@go build internal/test/bin/concord.go
