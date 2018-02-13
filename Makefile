.PHONY: all
all:
	go build ./cmd/k8cc-controller
	go build ./cmd/k8cc-api

.PHONY: test
test:
	go test ./...
	./hack/verify-codegen.sh
	gometalinter --vendor --skip=pkg/client --skip=pkg/apis ./...

.PHONY: gen
gen:
	./hack/update-codegen.sh
	go generate ./...
