.PHONY: all
all:
	go build ./cmd/k8cc-api

.PHONY: test
test:
	go test ./...

.PHONY: genmock
genmock:
	go generate ./...
