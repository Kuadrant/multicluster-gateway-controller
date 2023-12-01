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

if [ -z $MGC_BRANCH ]; then
  MGC_BRANCH=${MGC_BRANCH:="main"}
fi
if [ -z $MGC_ACCOUNT ]; then
MGC_ACCOUNT=${MGC_ACCOUNT:="kuadrant"}
fi

if [ -n "$MGC_LOCAL_QUICKSTART_SCRIPTS_MODE" ]; then
    echo "Loading quickstart scripts locally"
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    source "${SCRIPT_DIR}/.quickstartEnv"
    source "${SCRIPT_DIR}/.kindUtils"
    source "${SCRIPT_DIR}/.cleanupUtils"
    source "${SCRIPT_DIR}/.deployUtils"
    source "${SCRIPT_DIR}/.startUtils"
    source "${SCRIPT_DIR}/.setupEnv"
  else
    echo "Loading quickstart scripts from GitHub"
    source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.quickstartEnv)"
    source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.kindUtils)"
    source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.cleanupUtils)"
    source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.deployUtils)"
    source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.startUtils)"
    source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.setupEnv)"
fi


export OPERATOR_SDK_BIN=$(dockerBinCmd "operator-sdk")
export YQ_BIN=$(dockerBinCmd "yq")
export CLUSTERADM_BIN=$(dockerBinCmd "clusteradm")

MGC_REPO=${MGC_REPO:="github.com/${MGC_ACCOUNT}/multicluster-gateway-controller.git"}
QUICK_START_HUB_KUSTOMIZATION=${MGC_REPO}/config/quick-start/control-cluster
QUICK_START_SPOKE_KUSTOMIZATION=${MGC_REPO}/config/quick-start/workload-cluster

set -e pipefail

if [[ "${MGC_BRANCH}" != "main" ]]; then
  echo "setting MGC_REPO to use branch ${MGC_BRANCH}"
  QUICK_START_HUB_KUSTOMIZATION=${QUICK_START_HUB_KUSTOMIZATION}?ref=${MGC_BRANCH}
  QUICK_START_SPOKE_KUSTOMIZATION=${QUICK_START_SPOKE_KUSTOMIZATION}?ref=${MGC_BRANCH}
  echo "set QUICK_START_HUB_KUSTOMIZATION to ${QUICK_START_HUB_KUSTOMIZATION}"
  echo "set QUICK_START_SPOKE_KUSTOMIZATION to ${QUICK_START_SPOKE_KUSTOMIZATION}"

fi  


# Prompt user for any required env vars that have not been set
requiredENV

# Default config
if [[ -z "${LOG_LEVEL}" ]]; then
  LOG_LEVEL=1
fi
if [[ -z "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  MGC_WORKLOAD_CLUSTERS_COUNT=2
fi

# Make temporary directory for kubeconfig
mkdir -p ${TMP_DIR}

cleanupKind

kindSetupMGCClusters ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD} ${port80} ${port443} ${MGC_WORKLOAD_CLUSTERS_COUNT}

# Apply Cluster Configurations to Control cluster
# Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE} "minimal"
# Deploy Quick start kustomize
deployQuickStartControl ${KIND_CLUSTER_CONTROL_PLANE}
# Initialize local dev setup for the controller on the control-plane cluster
configureController ${KIND_CLUSTER_CONTROL_PLANE}
# Deploy MetalLb
configureMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}


# Apply Cluster Configurations to Workload clusters
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    deployQuickStartWorkload ${KIND_CLUSTER_WORKLOAD}-${i}
    configureMetalLB ${KIND_CLUSTER_WORKLOAD}-${i} $((${metalLBSubnetStart} + ${i}))
    deployOLM ${KIND_CLUSTER_WORKLOAD}-${i}
    deployOCMSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
    configureManagedAddon ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
    configureClusterAsIngress ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
  done
fi

kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}


echo ""
echo "What's next...

      Now that you have 2 kind clusters configured and with multicluster-gateway-controller installed you are ready to begin creating gateways
      Visit https://docs.kuadrant.io/multicluster-gateway-controller/docs/how-to/multicluster-gateways-walkthrough/ for next steps"