all:
	go build ./cmd/k8cc-controller
	go build ./cmd/k8cc-api

test:
	go test ./...
	./hack/verify-codegen.sh
	gometalinter --vendor --skip=pkg/client --skip=pkg/apis --deadline=5m ./...

gen:
	./hack/update-codegen.sh
	go generate ./...

docker:
	docker build -t "mbrt/k8cc-api:latest" -f Dockerfile.api .
	docker build -t "mbrt/k8cc-controller:latest" -f Dockerfile.ctrl .

.PHONY: all test gen docker
