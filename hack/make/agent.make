AGENT_IMG ?= agent:$(TAG)

.PHONY: build-agent
build-agent: manifests generate fmt vet ## Build agent binary.
	go build -o bin/agent ./cmd/agent/main.go

.PHONY: run-agent
run-agent: manifests generate fmt vet install
	go run ./cmd/agent/main.go \
	    --metrics-bind-address=:8082 \
	    --health-probe-bind-address=:8083 \
	    --control-plane-config-namespace=mctc-system

.PHONY: docker-build-agent
docker-build-agent: test ## Build docker image with the agent.
	docker build --target agent -t ${AGENT_IMG} .
	docker image prune -f --filter label=stage=mctc-builder

.PHONY: kind-load-agent
kind-load-agent: docker-build-agent
	kind load docker-image ${AGENT_IMG} --name mctc-workload-1  --nodes mctc-workload-1-control-plane

.PHONY: docker-push-agent
docker-push-agent: ## Push docker image with the agent.
	docker push ${AGENT_IMG}

.PHONY: deploy-agent
deploy-agent: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/agent && $(KUSTOMIZE) edit set image agent=${AGENT_IMG}
	$(KUSTOMIZE) build config/agent | kubectl apply -f -

.PHONY: undeploy-agent
undeploy-agent: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/agent | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

