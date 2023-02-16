#!/bin/bash

LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/.setupEnv
source "${LOCAL_SETUP_DIR}"/.clusterUtils

set -e pipefail

KIND_CLUSTER_PREFIX="mctc-"
KIND_CLUSTER_CONTROL_PLANE="${KIND_CLUSTER_PREFIX}control-plane"

makeSecretForCluster $KIND_CLUSTER_CONTROL_PLANE $(kubectl config current-context) |
setNamespacedName mctc-system control-plane-cluster |
setLabel argocd.argoproj.io/secret-type cluster > config/agent/secret.yaml


