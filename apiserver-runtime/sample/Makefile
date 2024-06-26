.PHONY: codegen fix fmt vet lint test tidy

GOBIN := $(shell go env GOPATH)/bin

all: codegen fix fmt vet lint test tidy

docker:
	GOOS=linux GOARCH=amd64 go build -o install/bin/apiserver
	docker build install --tag apiserver-runtime-sample:v0.0.0

install: docker
	kustomize build install | kubectl apply -f -

reinstall: docker
	kustomize build install | kubectl apply -f -
	kubectl delete pods -n sample-system --all

apiserver-logs:
	kubectl logs -l apiserver=true --container apiserver -n sample-system -f --tail 1000

codegen:
	(which apiserver-runtime-gen || go get "github.com/vine-io/kes/apiserver-runtime/tools/apiserver-runtime-gen")
	go generate

genclients:
	go run ../tools/apiserver-runtime-gen \
		-g client-gen \
		-g deepcopy-gen \
		-g go-to-protobuf \
		-g informer-gen \
		-g lister-gen \
		-g openapi-gen \
		--module "github.com/vine-io/kes/apiserver-runtime/sample" \
		--versions "github.com/vine-io/kes/apiserver-runtime/sample/pkg/apis/sample/v1alpha1"

	cd .. && \
	go-to-protobuf --apimachinery-packages "+k8s.io/apimachinery/pkg/util/intstr,+k8s.io/apimachinery/pkg/api/resource,+k8s.io/apimachinery/pkg/runtime/schema,+k8s.io/apimachinery/pkg/runtime,k8s.io/apimachinery/pkg/apis/meta/v1"  --output-base /Users/xingyys/project/gopath/src/ --go-header-file sample/hack/boilerplate.go.txt --proto-import=vendor --proto-import=vendor/k8s.io/kubernetes/third_party/protobuf --packages github.com/vine-io/kes/apiserver-runtime/sample/pkg/apis/sample/v1alpha1

fix:
	go fix ./...

fmt:
	test -z $(go fmt ./tools/...)

tidy:
	go mod tidy

lint:
	(which golangci-lint || go get github.com/golangci/golangci-lint/cmd/golangci-lint)
	$(GOBIN)/golangci-lint run ./...

test:
	go test -cover ./...

vet:
	go vet ./...

local-run:
	apiserver-boot run local --run=etcd,apiserver