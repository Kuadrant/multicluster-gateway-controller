
VERSION ?= 0.0.0
REGISTRY = quay.io
ORG ?= kuadrant

IMAGE_TAG_BASE ?= $(REGISTRY)/$(ORG)/multicluster-gateway-controller


ifeq (0.0.0,$(VERSION))
IMAGE_TAG ?= latest
else
IMAGE_TAG ?= v$(VERSION)
endif

IMG = ${IMAGE_TAG_BASE}:${IMAGE_TAG}


BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:$(IMAGE_TAG)

ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)


BUNDLE_GEN_FLAGS ?= -q --overwrite --version --crds $(VERSION) $(BUNDLE_METADATA_OPTS)

# Build the docker image
.PHONY: docker-build
docker-build:
	docker build --target controller -t ${IMG} .

# Login to the registry
.PHONY: docker-login
docker-login:
	echo "$(QUAY_TOKEN)" | docker --config="${DOCKER_CONFIG}" login -u "${QUAY_USER}" quay.io --password-stdin

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMG}

..PHONY: bundle
bundle: manifests operator-sdk kustomize  ## Generate bundle manifests and metadata, then validate generated files.

	$(OPERATOR_SDK) generate kustomize manifests  -q 
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .


.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	 docker push $(BUNDLE_IMG)

	 

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif



# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:$(IMAGE_TAG)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMG) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	docker push $(CATALOG_IMG)