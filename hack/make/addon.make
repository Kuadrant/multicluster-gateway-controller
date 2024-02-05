OCM_ADDON_IMG ?= quay.io/kuadrant/addon-manager:v0.0.1

.PHONY: build-addon-manager
build-addon-manager: manifests generate fmt vet ## Build ocm binary.
	go build -o bin/addon-manager ./cmd/ocm/main.go

.PHONY: run-addon-manager
run-addon-manager: manifests generate fmt vet
	go run ./cmd/ocm/main.go 
	   

.PHONY: docker-build-add-on-manager
docker-build-add-on-manager: ## Build docker image with the add-on manager.
	docker build --target add-on-manager -t ${OCM_ADDON_IMG} .
	docker image prune -f --filter label=stage=mgc-builder

.PHONY: kind-load-add-on-manager
kind-load-add-on-manager: docker-build-ocm
	kind load docker-image ${OCM_ADDON_IMG} --name mgc-control-plane  --nodes mgc-control-plane-control-plane

.PHONY: docker-push-add-on-manager
docker-push-ocm: ## Push docker image with the ocm.
	docker push ${OCM_ADDON_IMG}

    .PHONY: deploy-add-on-manager
deploy-add-on-manager:  ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	kubectl apply -f config/ocm 