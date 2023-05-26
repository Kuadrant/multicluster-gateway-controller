##@ Syncer

SYNCER_IMG ?= syncer:$(TAG)

.PHONY: build-syncer
build-syncer: manifests generate fmt vet ## Build syncer binary.
	go build -o bin/syncer ./cmd/syncer/main.go

METRICS_PORT ?= 8086
HEALTH_PORT ?= 8087
.PHONY: run-syncer
run-syncer: manifests generate fmt vet install
	go run ./cmd/syncer/main.go \
	    --metrics-bind-address=:${METRICS_PORT} \
	    --health-probe-bind-address=:${HEALTH_PORT}\
	    --control-plane-config-name=control-plane-cluster \
	    --control-plane-config-namespace=mgc-system \
	    --synced-resources=gateways.v1beta1.gateway.networking.k8s.io \
	    --synced-resources=ratelimitpolicies.v1beta1.kuadrant.io \
	    --synced-resources=secrets.v1

.PHONY: docker-build-syncer
docker-build-syncer: ## Build docker image with the syncer.
	docker build --target syncer -t ${SYNCER_IMG} .
	docker image prune -f --filter label=stage=mgc-builder

.PHONY: kind-load-syncer
kind-load-syncer: docker-build-syncer
	kind load docker-image ${SYNCER_IMG} --name mgc-workload-1  --nodes mgc-workload-1-control-plane

.PHONY: docker-push-syncer
docker-push-syncer: ## Push docker image with the syncer.
	docker push ${SYNCER_IMG}

.PHONY: deploy-syncer
deploy-syncer: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/syncer && $(KUSTOMIZE) edit set image syncer=${SYNCER_IMG}
	$(KUSTOMIZE) build config/syncer | kubectl apply -f -

.PHONY: undeploy-syncer
undeploy-syncer: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/syncer | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: redeploy-syncer
redeploy-syncer: undeploy-syncer deploy-syncer

.PHONY: restart-syncer
restart-syncer:
	kubectl rollout restart deploy sync-agent -n mgc-system
