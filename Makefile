.PHONY: all
all:
	go build ./cmd/k8cc-api

.PHONY: test
test:
	go test ./...
	gometalinter --vendor ./...

.PHONY: genmock
genmock:
	go generate ./...
