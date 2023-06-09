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


## Tool Versions
KUSTOMIZE_VERSION ?= v4.5.4
CONTROLLER_TOOLS_VERSION ?= v0.10.0
KIND_VERSION ?= v0.17.0
HELM_VERSION ?= v3.10.0
YQ_VERSION ?= v4.30.8
ISTIOVERSION ?= 1.17.0
OPERATOR_SDK_VERSION ?= 1.28.0
CLUSTERADM_VERSION ?= 0.5.1
SUBCTL_VERSION ?= release-0.15
GINKGO_VERSION ?= v2.6.1


KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: operator-sdk
operator-sdk:
	@if test -x ${LOCALBIN}/operator-sdk && ! ${LOCALBIN}/operator-sdk version | grep -q ${OPERATOR_SDK_VERSION}; then \
		echo "${OPERATOR_SDK} version is not expected ${OPERATOR_SDK_VERSION}. Removing it before installing."; \
		rm -rf ${OPERATOR_SDK}; \
	fi
ifeq ("$(shell ls ${OPERATOR_SDK})", "")
	curl -LO https://github.com/operator-framework/operator-sdk/releases/download/v${OPERATOR_SDK_VERSION}/operator-sdk_${OS}_${ARCH}
	chmod +x operator-sdk_${OS}_${ARCH}
	mv operator-sdk_${OS}_${ARCH} $(OPERATOR_SDK)
endif

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

.PHONY: istioctl
istioctl: $(ISTIOCTL)
$(ISTIOCTL):
	$(eval ISTIO_TMP := $(shell mktemp -d))
	cd $(ISTIO_TMP); curl -sSL https://istio.io/downloadIstio | ISTIO_VERSION=$(ISTIOVERSION) sh -
	cp $(ISTIO_TMP)/istio-$(ISTIOVERSION)/bin/istioctl ${ISTIOCTL}
	-rm -rf $(TMP)

.PHONY: clusteradm
clusteradm: $(CLUSTERADM)
$(CLUSTERADM):
	test -s $(LOCALBIN)/clusteradm || curl -sL https://raw.githubusercontent.com/open-cluster-management-io/clusteradm/main/install.sh | INSTALL_DIR=bin bash -s -- $(CLUSTERADM_VERSION)

.PHONY: subctl
subctl: $(SUBCTL)
$(SUBCTL):
	test -s $(LOCALBIN)/subctl || curl https://get.submariner.io | DESTDIR=$(LOCALBIN) VERSION=$(SUBCTL_VERSION) bash

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary
$(GINKGO):
	test -s $(GINKGO) || GOBIN=$(LOCALBIN) go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
