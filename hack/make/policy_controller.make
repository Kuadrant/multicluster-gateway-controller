##@ PolicyController

POLICY_CONTROLLER_IMG ?= policy-controller:$(TAG)
LOG_LEVEL ?= 3

.PHONY: build-policy-controller
build-policy-controller: manifests generate fmt vet ## Build controller binary.
	go build -o bin/policy_controller ./cmd/policy_controller/main.go

.PHONY: run-policy-controller
run-policy-controller: manifests generate fmt vet  install
	go run ./cmd/policy_controller/main.go \
	    --metrics-bind-address=:8090 \
	    --health-probe-bind-address=:8091 \
	    --zap-log-level=$(LOG_LEVEL)

.PHONY: docker-build-policy-controller
docker-build-policy-controller: ## Build docker image with the controller.
	docker build --target policy-controller -t ${POLICY_CONTROLLER_IMG} .
	docker image prune -f --filter label=stage=mgc-builder

.PHONY: kind-load-policy-controller
kind-load-policy-controller: docker-build-policy-controller
	kind load docker-image ${POLICY_CONTROLLER_IMG} --name mgc-control-plane  --nodes mgc-control-plane-control-plane

.PHONY: docker-push-policy-controller
docker-push-policy-controller: ## Push docker image with the controller.
	docker push ${POLICY_CONTROLLER_IMG}

.PHONY: deploy-policy-controller
deploy-policy-controller: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/policy-controller && $(KUSTOMIZE) edit set image policy-controller=${POLICY_CONTROLLER_IMG}
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/deploy/local | kubectl apply -f -
	@if [ "$(METRICS)" = "true" ]; then\
		$(KUSTOMIZE) build config/prometheus | kubectl apply -f -;\
	fi

.PHONY: undeploy-policy-controller
undeploy-policy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	@if [ $(METRICS) = "true" ]; then\
		$(KUSTOMIZE) build config/prometheus | kubectl delete --ignore-not-found=$(ignore-not-found) -f -;\
	fi
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/deploy/local | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: restart-policy-controller
restart-policy-controller:
	kubectl rollout restart deployment policy-controller-manager -n multicluster-gateway-controller-system
	
.PHONY: tail-policy-controller-logs
tail-policy-controller-logs:
	kubectl logs -f deployment/policy-controller-manager -n multicluster-gateway-controller-system