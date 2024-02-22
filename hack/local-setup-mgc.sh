#!/bin/bash

#
# Copyright 2022 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/.binEnv
source "${LOCAL_SETUP_DIR}"/.setupEnv
source "${LOCAL_SETUP_DIR}"/.startUtils
source "${LOCAL_SETUP_DIR}"/.cleanupUtils
source "${LOCAL_SETUP_DIR}"/.deployUtils

export TMP_DIR=./tmp

# shellcheck disable=SC2034
QUICK_START_HUB_KUSTOMIZATION=config/quick-start/control-cluster
# shellcheck disable=SC2034
QUICK_START_SPOKE_KUSTOMIZATION=config/quick-start/workload-cluster

postDeployMGCHub() {
    clusterName=${1}
    kubectl config use-context kind-${clusterName}

    echo "Running post MGC deployment setup on ${clusterName}"

    # Bit hacky, but ... delete the MGC deployment we just created so local development can work as normal
    kubectl delete deployments/mgc-controller-manager -n multicluster-gateway-controller-system
    kubectl wait --for=delete deployments/mgc-controller-manager -n multicluster-gateway-controller-system

    ${KUSTOMIZE_BIN} build config/local-setup/controller/ | kubectl apply -f -
    if [[ -f "controller-config.env" && -f "gcp-credentials.env" ]]; then
      ${KUSTOMIZE_BIN} --reorder none --load-restrictor LoadRestrictionsNone build config/local-setup/controller/gcp | kubectl apply -f -
    fi
    if [[ -f "controller-config.env" && -f "aws-credentials.env" ]]; then
      ${KUSTOMIZE_BIN} --reorder none --load-restrictor LoadRestrictionsNone build config/local-setup/controller/aws | kubectl apply -f -
    fi
}

set -e pipefail

cleanupMGC

# Apply Cluster Configurations to Control cluster

# Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy MGC and its dependencies to a hub cluster
deployMGCHub ${KIND_CLUSTER_CONTROL_PLANE}

# Post MGC deployment tasks, adds managedezones, dns providers etc..
postDeployMGCHub ${KIND_CLUSTER_CONTROL_PLANE}

# Setup Hub as Spoke if using a single kind cluster
if [[ -n "${OCM_SINGLE}" ]]; then
  deployOCMSpoke ${KIND_CLUSTER_CONTROL_PLANE}
  deployMGCSpoke ${KIND_CLUSTER_CONTROL_PLANE}
  configureClusterAsIngress ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_CONTROL_PLANE}
fi

# Configure MetalLb
# shellcheck disable=SC2154
configureMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}

### --- Metrics Start --- ###

# Deploy ingress controller
deployIngressController ${KIND_CLUSTER_CONTROL_PLANE} ${INGRESS_NGINX_DIR}

# Deploy Prometheus in the hub too
deployPrometheusForFederation ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Thanos components in the hub
deployThanos ${KIND_CLUSTER_CONTROL_PLANE}

### --- Metrics End --- ###

# Configure spoke clusters if MGC_WORKLOAD_CLUSTERS_COUNT environment variable is set
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    deployOCMSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
    deployMGCSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
    configureClusterAsIngress ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
    deployOLM ${KIND_CLUSTER_WORKLOAD}-${i}
    configureManagedAddon ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
    configureMetalLB ${KIND_CLUSTER_WORKLOAD}-${i} $((${metalLBSubnetStart} + ${i}))
    deployPrometheusForFederation ${KIND_CLUSTER_WORKLOAD}-${i}
  done
fi

# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
