
# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.0

# REGISTRY defines the image registry name for the bundle and catalog
# Update this value to what image registry 
REGISTRY = quay.io

# ORG defines the image registy organization name for the bundle and catalog image repos
ORG ?= kuadrant

#IMAGE_TAG_BASE defines the image registry
IMAGE_TAG_BASE ?= $(REGISTRY)/$(ORG)/multicluster-gateway-controller


ifeq (0.0.0,$(VERSION))
IMAGE_TAG ?= latest
else
IMAGE_TAG ?= v$(VERSION)
endif

# Image URL to use all building/pushing image targets
IMG = ${IMAGE_TAG_BASE}:${IMAGE_TAG}

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:$(IMAGE_TAG)

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version --crds $(VERSION) $(BUNDLE_METADATA_OPTS)

## Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests operator-sdk kustomize  

	$(OPERATOR_SDK) generate kustomize manifests -q --apis-dir pkg/apis/v1alpha1
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle -q $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle
	$(MAKE) bundle-ignore-createdAt

# Since operator-sdk 1.26.0, `make bundle` changes the `createdAt` field from the bundle
# even if it is patched:
#   https://github.com/operator-framework/operator-sdk/pull/6136
# This code checks if only the createdAt field. If is the only change, it is ignored.
# Else, it will do nothing.
# https://github.com/operator-framework/operator-sdk/issues/6285#issuecomment-1415350333
# https://github.com/operator-framework/operator-sdk/issues/6285#issuecomment-1532150678
.PHONY: bundle-ignore-createdAt
bundle-ignore-createdAt:
	git diff --quiet -I'^    createdAt: ' ./bundle && git checkout ./bundle || true

## Build the bundle image.
.PHONY: bundle-build
bundle-build: 
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

## Push the bundle image.
.PHONY: bundle-push
bundle-push: 
	 docker push $(BUNDLE_IMG)

.PHONY: bundle-build-push	 
bundle-build-push: bundle bundle-build bundle-push

## Download opm locally if necessary.
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

# Build the docker image
.PHONY: docker-build
docker-build:
	docker build  --target controller . -t ${IMAGE_TAG_BASE:}${IMG}

# Login to the registry
.PHONY: docker-login
docker-login:
	echo "$(QUAY_TOKEN)" | docker --config="${DOCKER_CONFIG}" login -u "${QUAY_USER}" quay.io --password-stdin

# Push the docker image
.PHONY: docker-push
docker-push:
	docker push ${IMAGE_TAG_BASE:}${IMG}

.PHONY: catalog-build-push
catalog-build-push: catalog-build catalog-push

