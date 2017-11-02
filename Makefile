.PHONY: all
all:
	go build ./...

.PHONY: test
test:
	go test ./...

.PHONY: genmock
genmock:
	go generate ./...
