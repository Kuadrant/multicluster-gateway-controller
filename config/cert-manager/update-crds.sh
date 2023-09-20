#!/bin/bash

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"

set -e pipefail

chartName=cert-manager
chartVersion=$(yq ".helmCharts[] | select(.name == \"$chartName\")".version ${SCRIPT_DIR}/kustomization.yaml)

mkdir -p $SCRIPT_DIR/crd/$chartVersion
curl -L -o $SCRIPT_DIR/crd/$chartVersion/cert-manager.crds.yaml https://github.com/cert-manager/cert-manager/releases/download/$chartVersion/cert-manager.crds.yaml

# Update any reference to the crds in tests i.e. test/integration/suite_test.go, to use the new version.
