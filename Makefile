# TODO: Add unit tests
# .PHONY: test
# test:
# 	@echo "\n🛠️  Running unit tests..."
# 	go test ./...

.PHONY: build
build:
	@echo "\n🔧  Building Go binaries..."
	GOOS=darwin GOARCH=amd64 go build -o bin/image-annotator-webhook-darwin-amd64 .
	GOOS=linux GOARCH=amd64 go build -o bin/image-annotator-webhook-linux-amd64 .

.PHONY: docker-build
docker-build:
	@echo "\n📦 Building simple-kubernetes-webhook Docker image..."
	docker build -t image-annotator-webhook:latest .

# # From this point `kind` is required
# .PHONY: cluster
# cluster:
# 	@echo "\n🔧 Creating Kubernetes cluster..."
# 	kind create cluster --config dev/manifests/kind/kind.cluster.yaml

# .PHONY: delete-cluster
# delete-cluster:
# 	@echo "\n♻️  Deleting Kubernetes cluster..."
# 	kind delete cluster

