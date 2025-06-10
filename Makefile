.PHONY: build test lint fmt clean run docker-build helm-lint operator-build operator-run install-crds deploy-operator

# Server binaries
BINARY_NAME=jit-server
OPERATOR_BINARY_NAME=jit-operator
DOCKER_IMAGE=jit-bot
OPERATOR_IMAGE=jit-operator
VERSION?=latest

# Build targets
build:
	go build -o $(BINARY_NAME) cmd/jit-server/main.go

operator-build:
	go build -o $(OPERATOR_BINARY_NAME) cmd/operator/main.go

build-all: build operator-build

test:
	go test -v ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

clean:
	go clean
	rm -f $(BINARY_NAME) $(OPERATOR_BINARY_NAME)

run: build
	./$(BINARY_NAME) serve

operator-run: operator-build
	./$(OPERATOR_BINARY_NAME)

# Docker targets
docker-build:
	docker build -t $(DOCKER_IMAGE):$(VERSION) .

operator-docker-build:
	docker build -t $(OPERATOR_IMAGE):$(VERSION) -f cmd/operator/Dockerfile .

# Kubernetes deployment targets
install-crds:
	kubectl apply -f manifests/crds/

uninstall-crds:
	kubectl delete -f manifests/crds/ --ignore-not-found=true

deploy-operator: install-crds
	kubectl apply -f manifests/operator/

undeploy-operator:
	kubectl delete -f manifests/operator/ --ignore-not-found=true

deploy-all: install-crds deploy-operator

undeploy-all: undeploy-operator uninstall-crds

# Development tools
helm-lint:
	helm lint charts/jit-bot

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

mod-download:
	go mod download

mod-tidy:
	go mod tidy