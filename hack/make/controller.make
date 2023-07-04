##@ Controller

CONTROLLER_IMG ?= controller:$(TAG)
LOG_LEVEL ?= 3

.PHONY: build-controller
build-controller: manifests generate fmt vet ## Build controller binary.
	go build -o bin/controller ./cmd/controller/main.go

.PHONY: run-controller
run-controller: manifests generate fmt vet  install
	go run ./cmd/controller/main.go \
	    --metrics-bind-address=:8080 \
	    --health-probe-bind-address=:8081 \
	    --zap-log-level=$(LOG_LEVEL)

.PHONY: docker-build-controller
docker-build-controller: ## Build docker image with the controller.
	docker build --target controller -t ${CONTROLLER_IMG} .
	docker image prune -f --filter label=stage=mgc-builder

.PHONY: kind-load-controller
kind-load-controller: docker-build-controller
	kind load docker-image ${CONTROLLER_IMG} --name mgc-control-plane  --nodes mgc-control-plane-control-plane

.PHONY: docker-push-controller
docker-push-controller: ## Push docker image with the controller.
	docker push ${CONTROLLER_IMG}

.PHONY: deploy-controller
deploy-controller: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/deploy/local | kubectl apply -f -

.PHONY: undeploy-controller
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: restart-controller
restart-controller:
	kubectl rollout restart deployment mgc-controller-manager -n multicluster-gateway-controller-system
	
.PHONY: tail-controller-logs
tail-controller-logs:
	kubectl logs -f deployment/mgc-controller-manager -n multicluster-gateway-controller-system