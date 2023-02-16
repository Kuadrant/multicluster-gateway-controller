
# Image URL to use all building/pushing image targets
TAG ?= latest
CONTROLLER_IMG ?= controller:$(TAG)
AGENT_IMG ?= agent:$(TAG)

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.25.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: clean
clean: ## Clean up temporary files.
	-rm -rf ./bin/*
	-rm -rf ./tmp

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint against code.
	golangci-lint run ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: local-setup
local-setup: kind kustomize helm yq dev-tls ## Setup multi cluster traffic controller locally using kind.
	./hack/local-setup.sh

.PHONY: local-cleanup
local-cleanup: kind clear-dev-tls  ## Cleanup resources created by local-setup
	./hack/local-cleanup.sh
	$(MAKE) clean

##@ Build

.PHONY: build
build: build-controller build-agent ## Build all binaries.

.PHONY: build-controller
build-controller: manifests generate fmt vet ## Build controller binary.
	go build -o bin/controller ./cmd/controller/main.go

.PHONY: run-controller
run-controller: manifests generate fmt vet install ## Run controller from your host.
	go run ./cmd/controller/main.go

build-agent: manifests generate fmt vet ## Build agent binary.
	go build -o bin/agent ./cmd/agent/main.go

.PHONY: run-agent
run-agent: manifests generate fmt vet install ## Run agent from your host.
	go run ./cmd/agent/main.go --control-plane-config-namespace=mctc-system

.PHONY: docker-build-controller
docker-build-controller: test ## Build docker image with the controller.
	docker build --target controller -t ${CONTROLLER_IMG} .

.PHONY: kind-load-controller
kind-load-controller: docker-build-controller
	kind load docker-image ${CONTROLLER_IMG} --name mctc-control-plane  --nodes mctc-control-plane-control-plane

.PHONY: docker-push-controller
docker-push-controller: ## Push docker image with the controller.
	docker push ${CONTROLLER_IMG}

.PHONY: docker-build-agent
docker-build-agent: test ## Build docker image with the agent.
	docker build --target agent -t ${AGENT_IMG} .
	docker image prune -f --filter label=stage=mctc-builder

.PHONY: docker-push-agent
docker-push-agent: ## Push docker image with the controller.
	docker push ${AGENT_IMG}

.PHONY: kind-load-agent
kind-load-agent: docker-build-agent
	kind load docker-image ${AGENT_IMG} --name mctc-workload-1  --nodes mctc-workload-1-control-plane


##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy-controller
deploy-controller: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${CONTROLLER_IMG}
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/deploy/local | kubectl apply -f -

.PHONY: agent-secret
agent-secret: kustomize kind yq
	./hack/gen-agent-secret.sh
	
.PHONY: deploy-agent
deploy-agent: manifests kustomize kind yq agent-secret ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/agent && $(KUSTOMIZE) edit set image agent=${AGENT_IMG}
	$(KUSTOMIZE) build config/agent | kubectl apply -f -

.PHONY: undeploy-controller
undeploy-controller: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -


.PHONY: undeploy-agent
undeploy-agent: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/agent | kubectl delete --ignore-not-found=$(ignore-not-found) -f -


.PHONY: deploy-sample-applicationset
deploy-sample-applicationset:
	kubectl apply -f ./samples/argocd-applicationset/echo-applicationset.yaml

DEV_TLS_DIR = config/webhook-setup/control/tls
DEV_TLS_CRT ?= $(DEV_TLS_DIR)/tls.crt
DEV_TLS_KEY ?= $(DEV_TLS_DIR)/tls.key

.PHONY: dev-tls
dev-tls: $(DEV_TLS_CRT) ## Generate dev tls webhook cert if necessary.
$(DEV_TLS_CRT):
	openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout $(DEV_TLS_KEY) -out $(DEV_TLS_CRT) -subj "/C=IE/O=Red Hat Ltd/OU=HCG/CN=webhook.172.32.0.2.nip.io" -addext "subjectAltName = DNS:webhook.172.32.0.2.nip.io"

.PHONY: clear-dev-tls
clear-dev-tls:
	-rm -f $(DEV_TLS_CRT)
	-rm -f $(DEV_TLS_KEY)

.PHONY: webhook-proxy
webhook-proxy: kustomize
	$(KUSTOMIZE) --load-restrictor LoadRestrictionsNone build config/webhook-setup/proxy | kubectl apply -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
KIND ?= $(LOCALBIN)/kind
HELM ?= $(LOCALBIN)/helm

## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.4
CONTROLLER_TOOLS_VERSION ?= v0.10.0
KIND_VERSION ?= v0.14.0
HELM_VERSION ?= v3.10.0
YQ_VERSION ?= v4.30.8

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: kind
kind: $(KIND) ## Download kind locally if necessary.
$(KIND):
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/kind@$(KIND_VERSION)

HELM_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3"
.PHONY: helm
helm: $(HELM)
$(HELM):
	curl -s $(HELM_INSTALL_SCRIPT) | HELM_INSTALL_DIR=$(LOCALBIN) PATH=$$PATH:$$HELM_INSTALL_DIR bash -s -- --no-sudo --version $(HELM_VERSION)

.PHONY: yq
yq: $(YQ)
	test -s $(LOCALBIN)/yq || GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_VERSION)

