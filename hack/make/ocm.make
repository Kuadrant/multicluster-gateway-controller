OCM_ADDON_IMG ?= ocm:$(TAG)

.PHONY: build-ocm
build-ocm: manifests generate fmt vet ## Build ocm binary.
	go build -o bin/ocm ./cmd/ocm/main.go

.PHONY: run-ocm
run-ocm: manifests generate fmt vet  install
	go run ./cmd/ocm/main.go 
	   

.PHONY: docker-build-ocm
docker-build-ocm: test ## Build docker image with the ocm.
	docker build --target ocm -t ${OCM_ADDON_IMG} .
	docker image prune -f --filter label=stage=mctc-builder

.PHONY: kind-load-ocm
kind-load-ocm: docker-build-ocm
	kind load docker-image ${OCM_ADDON_IMG} --name mctc-control-plane  --nodes mctc-control-plane-control-plane

.PHONY: docker-push-ocm
docker-push-ocm: ## Push docker image with the ocm.
	docker push ${OCM_ADDON_IMG}

    .PHONY: deploy-ocm
deploy-ocm: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/ocm && $(KUSTOMIZE) edit set image ocm=${OCM_IMG}
	$(KUSTOMIZE) build config/ocm | kubectl apply -f -