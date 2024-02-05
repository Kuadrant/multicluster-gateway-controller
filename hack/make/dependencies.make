##@ Build Dependencies

## system information
ARCH ?= amd64
OS ?= $(shell uname)

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
ISTIOCTL ?= $(LOCALBIN)/istioctl
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
CLUSTERADM ?= $(LOCALBIN)/clusteradm
SUBCTL ?= $(LOCALBIN)/subctl
GINKGO ?= $(LOCALBIN)/ginkgo
YQ ?= $(LOCALBIN)/yq
OPENSHIFT_GOIMPORTS ?= $(LOCALBIN)/openshift-goimports

## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.4
CONTROLLER_TOOLS_VERSION ?= v0.10.0
KIND_VERSION ?= v0.17.0
HELM_VERSION ?= v3.10.0
YQ_VERSION ?= v4.30.8
ISTIOVERSION ?= 1.20.0
OPERATOR_SDK_VERSION ?= 1.28.0
CLUSTERADM_VERSION ?= 0.6.0
SUBCTL_VERSION ?= release-0.15
GINKGO_VERSION ?= v2.13.2
OPENSHIFT_GOIMPORTS_VERSION ?= c70783e636f2213cac683f6865d88c5edace3157

.PHONY: dependencies
dependencies: kustomize operator-sdk controller-gen envtest kind helm yq istioctl clusteradm subctl ginkgo
	@echo "dependencies installed successfully"
	@echo "consider running `export PATH=$PATH:$(pwd)/bin` if you haven't already done"

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(KUSTOMIZE) && ! $(KUSTOMIZE) version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(KUSTOMIZE) version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(KUSTOMIZE); \
	fi
	test -s $(KUSTOMIZE) || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK)
$(OPERATOR_SDK): $(LOCALBIN)
	@if test -x ${OPERATOR_SDK} && ! ${OPERATOR_SDK} version | grep -q ${OPERATOR_SDK_VERSION}; then \
		echo "${OPERATOR_SDK} version is not expected ${OPERATOR_SDK_VERSION}. Removing it before installing."; \
		rm -rf ${OPERATOR_SDK}; \
	fi
	test -s ${OPERATOR_SDK} || \
	( curl -LO https://github.com/operator-framework/operator-sdk/releases/download/v${OPERATOR_SDK_VERSION}/operator-sdk_${OS}_${ARCH} && \
	chmod +x operator-sdk_${OS}_${ARCH} && \
	mv operator-sdk_${OS}_${ARCH} $(OPERATOR_SDK) )

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(ENVTEST) || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

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
$(YQ):
	test -s $(YQ) || GOBIN=$(LOCALBIN) go install github.com/mikefarah/yq/v4@$(YQ_VERSION)

.PHONY: istioctl
istioctl: $(ISTIOCTL)
$(ISTIOCTL):
	$(eval ISTIO_TMP := $(shell mktemp -d))
	cd $(ISTIO_TMP); curl -sSL https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIOVERSION) sh -
	cp $(ISTIO_TMP)/istio-$(ISTIOVERSION)/bin/istioctl ${ISTIOCTL}
	-rm -rf $(ISTIO_TMP)

.PHONY: clusteradm
clusteradm: $(CLUSTERADM)
$(CLUSTERADM):
	test -s $(CLUSTERADM)|| curl -sL https://raw.githubusercontent.com/open-cluster-management-io/clusteradm/main/install.sh | INSTALL_DIR=$(LOCALBIN) bash -s -- $(CLUSTERADM_VERSION)

.PHONY: subctl
subctl: $(SUBCTL)
$(SUBCTL):
	test -s $(SUBCTL) || curl https://get.submariner.io | DESTDIR=$(LOCALBIN) VERSION=$(SUBCTL_VERSION) bash

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary
$(GINKGO):
	test -s $(GINKGO) || GOBIN=$(LOCALBIN) go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)

.PHONY: openshift-goimports
openshift-goimports: $(OPENSHIFT_GOIMPORTS) ## Download openshift-goimports locally if necessary
$(OPENSHIFT_GOIMPORTS):
	GOBIN=$(LOCALBIN) go install github.com/openshift-eng/openshift-goimports@$(OPENSHIFT_GOIMPORTS_VERSION)
