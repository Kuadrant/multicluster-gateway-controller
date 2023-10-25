##@ Controller

CONTROLLER_IMG ?= controller:$(TAG)
LOG_LEVEL ?= 3

.PHONY: build-gateway-controller
build-gateway-controller: manifests generate fmt vet ## Build controller binary.
	go build -o bin/controller ./cmd/gateway_controller/main.go

.PHONY: run-gateway-controller
run-gateway-controller: manifests generate fmt vet  install
	go run ./cmd/gateway_controller/main.go \
	    --metrics-bind-address=:8080 \
	    --health-probe-bind-address=:8081 \
	    --zap-log-level=$(LOG_LEVEL)

.PHONY: docker-build-gateway-controller
docker-build-gateway-controller: ## Build docker image with the controller.
	docker build --target controller -t ${CONTROLLER_IMG} .
	docker image prune -f --filter label=stage=mgc-builder

.PHONY: kind-load-gateway-controller
kind-load-gateway-controller: docker-build-gateway-controller
	kind load docker-image ${CONTROLLER_IMG} --name mgc-control-plane  --nodes mgc-control-plane-control-plane

.PHONY: docker-push-gateway-controller
docker-push-gateway-controller: ## Push docker image with the controller.
	docker push ${CONTROLLER_IMG}

.PHONY: deploy-gateway-controller
deploy-gateway-controller: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/deploy/local | kubectl apply -f -
	@if [ "$(METRICS)" = "true" ]; then\
		$(KUSTOMIZE) build config/prometheus | kubectl apply -f -;\
	fi

.PHONY: undeploy-gateway-controller
undeploy-gateway-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@if [ $(METRICS) = "true" ]; then\
		$(KUSTOMIZE) build config/prometheus | kubectl delete --ignore-not-found=$(ignore-not-found) -f -;\
	fi
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/deploy/local | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: restart-gateway-controller
restart-gateway-controller:
	kubectl rollout restart deployment mgc-controller-manager -n multicluster-gateway-controller-system
	
.PHONY: tail-gateway-controller-logs
tail-gateway-controller-logs:
	kubectl logs -f deployment/mgc-controller-manager -n multicluster-gateway-controller-system