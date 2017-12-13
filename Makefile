.PHONY: all
all:
	go build ./cmd/k8cc-api

.PHONY: test
test:
	go test ./...
	gometalinter --vendor --skip=pkg/client --skip=pkg/apis ./...
	./hack/verify-codegen.sh

.PHONY: gen
gen:
	go generate ./...
	./hack/update-codegen.sh
