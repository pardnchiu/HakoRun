-include .env
export

.PHONY: run build test redis none

GO_TAGS := $(if $(filter redis,$(MAKECMDGOALS)),-tags redis,)

# * make build/run redis => use redis
# * make build/run => use toriidb
build:
	go build $(GO_TAGS) -o bin/go-faas ./cmd/api

run:
	go run $(GO_TAGS) ./cmd/api

test:
	go test ./...
