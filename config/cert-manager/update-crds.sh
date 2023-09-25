#!/bin/bash

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"

set -e pipefail

chartName=cert-manager
chartVersion=$(yq ".helmCharts[] | select(.name == \"$chartName\")".version ${SCRIPT_DIR}/kustomization.yaml)

curl -L -o $SCRIPT_DIR/crd/latest/cert-manager.crds.yaml https://github.com/cert-manager/cert-manager/releases/download/$chartVersion/cert-manager.crds.yaml
