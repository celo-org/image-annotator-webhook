cluster_name = cluster-dev

.PHONY: test
test:
	@echo "\n🛠️  Running unit tests..."
	go test ./...

.PHONY: build
build:
	@echo "\n🔧  Building Go binaries..."
	GOOS=darwin GOARCH=amd64 go build -o bin/image-annotator-webhook-darwin-amd64 .
	GOOS=linux GOARCH=amd64 go build -o bin/image-annotator-webhook-linux-amd64 .

.PHONY: docker-build
docker-build:
	@echo "\n📦 Building simple-kubernetes-webhook Docker image..."
	docker build -t image-annotator-webhook:latest .

.PHONY: cluster
cluster:
	@echo "\n🔧 Creating Kubernetes cluster..."
	kind create cluster --name $(cluster_name)

.PHONY: delete-cluster
delete-cluster:
	@echo "\n♻️  Deleting Kubernetes cluster..."
	kind delete cluster --name $(cluster_name)

.PHONY: push
push: docker-build
	@echo "\n📦 Pushing admission-webhook image into Kind's Docker daemon..."
	kind load docker-image image-annotator-webhook:latest --name $(cluster_name)

.PHONY: deploy-webhook
deploy-webhook:
	@echo "\n🪝 Deploying webhook manifests to Kind cluster..."
	kubectl apply -f k8s-manifests/webhook/

.PHONY: push-deploy
push-deploy: push
	@echo "\n Redeploying webhook pod"
	kubectl delete pod -n image-annotator -l app.kubernetes.io/name=image-annotator-webhook

.PHONY: deploy-testing
deploy-testing: push-deploy
	@echo "\n📦 Deploying webhook manifests to Kind cluster..."
	kubectl delete ns testing || true
	kubectl apply -f k8s-manifests/testing/

.PHONY: logs-webhook
logs-webhook:
	kubectl logs -n image-annotator -l app.kubernetes.io/name=image-annotator-webhook
	kubectl events -n testing
