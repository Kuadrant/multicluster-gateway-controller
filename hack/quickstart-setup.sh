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

if [[ "${MGC_BRANCH}" != "main" ]]; then
  echo "setting MGC_REPO to use branch ${MGC_BRANCH}"
  QUICK_START_HUB_KUSTOMIZATION=${QUICK_START_HUB_KUSTOMIZATION}?ref=${MGC_BRANCH}
  QUICK_START_SPOKE_KUSTOMIZATION=${QUICK_START_SPOKE_KUSTOMIZATION}?ref=${MGC_BRANCH}
  echo "set QUICK_START_HUB_KUSTOMIZATION to ${QUICK_START_HUB_KUSTOMIZATION}"
  echo "set QUICK_START_SPOKE_KUSTOMIZATION to ${QUICK_START_SPOKE_KUSTOMIZATION}"
fi  

setupAWSProvider() {
  local namespace="$1"
  if [ -z "$1" ]; then
    namespace="multi-cluster-gateways"
  fi
  if [ "$KUADRANT_AWS_ACCESS_KEY_ID" == "" ]; then
    echo "KUADRANT_AWS_ACCESS_KEY_ID is not set"
    exit 1
  fi

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${KIND_CLUSTER_PREFIX}aws-credentials
  namespace: ${namespace}
type: "kuadrant.io/aws"
stringData:
  AWS_ACCESS_KEY_ID: ${KUADRANT_AWS_ACCESS_KEY_ID}
  AWS_SECRET_ACCESS_KEY: ${KUADRANT_AWS_SECRET_ACCESS_KEY}
  AWS_REGION: ${KUADRANT_AWS_REGION}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${KIND_CLUSTER_PREFIX}controller-config
  namespace: ${namespace}
data:
  AWS_DNS_PUBLIC_ZONE_ID: ${KUADRANT_AWS_DNS_PUBLIC_ZONE_ID}
  ZONE_ROOT_DOMAIN: ${KUADRANT_ZONE_ROOT_DOMAIN}
  LOG_LEVEL: "${LOG_LEVEL}"
---
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: ${KIND_CLUSTER_PREFIX}dev-mz
  namespace: ${namespace}
spec:
  id: ${KUADRANT_AWS_DNS_PUBLIC_ZONE_ID}
  domainName: ${KUADRANT_ZONE_ROOT_DOMAIN}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: ${KIND_CLUSTER_PREFIX}aws-credentials
EOF
}

setupGCPProvider() {
  local namespace="$1"
  if [ -z "$1" ]; then
    namespace="multi-cluster-gateways"
  fi
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${KIND_CLUSTER_PREFIX}gcp-credentials
  namespace: ${namespace}
type: "kuadrant.io/gcp"
stringData:
  GOOGLE: '${GOOGLE}'
  PROJECT_ID: ${PROJECT_ID}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${KIND_CLUSTER_PREFIX}controller-config
  namespace: ${namespace}
data:
  ZONE_DNS_NAME: ${ZONE_DNS_NAME}
  ZONE_NAME: ${ZONE_NAME}
  LOG_LEVEL: "${LOG_LEVEL}"
---
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: ${KIND_CLUSTER_PREFIX}dev-mz
  namespace: ${namespace}
spec:
  id: ${ZONE_NAME}
  domainName: ${ZONE_DNS_NAME}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: ${KIND_CLUSTER_PREFIX}gcp-credentials
EOF
}

postDeployMGCHub() {
    clusterName=${1}
    namespace=${2}
    kubectl config use-context kind-${clusterName}
    echo "Running post MGC deployment setup on ${clusterName}"

    case $DNS_PROVIDER in
      aws)
          echo "Setting up an AWS Route 53 DNS provider"
          setupAWSProvider ${namespace}
          ;;
      gcp)
          echo "Setting up a Google Cloud DNS provider"
          setupGCPProvider ${namespace}
          ;;
      *)
        echo "Unknown DNS provider"
        exit
        ;;
    esac
}

set -e pipefail

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

# shellcheck disable=SC2154
kindSetupMGCClusters ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD} ${port80} ${port443} ${MGC_WORKLOAD_CLUSTERS_COUNT}

# Apply Cluster Configurations to Control cluster

# Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy MGC and its dependencies to a hub cluster
deployMGCHub ${KIND_CLUSTER_CONTROL_PLANE}

# Post MGC deployment tasks, adds managedezones, dns providers etc..
postDeployMGCHub ${KIND_CLUSTER_CONTROL_PLANE}

# Configure MetalLb
# shellcheck disable=SC2154
configureMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}

# Configure spoke clusters if MGC_WORKLOAD_CLUSTERS_COUNT environment variable is set
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    deployOCMSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
    deployMGCSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
    configureClusterAsIngress ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
    deployOLM ${KIND_CLUSTER_WORKLOAD}-${i}
    configureManagedAddon ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
    configureMetalLB ${KIND_CLUSTER_WORKLOAD}-${i} $((${metalLBSubnetStart} + ${i}))
  done
fi

# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}

echo ""
echo "What's next...

      Now that you have 2 kind clusters configured and with multicluster-gateway-controller installed you are ready to begin creating gateways
      Visit https://docs.kuadrant.io/multicluster-gateway-controller/docs/how-to/multicluster-gateways-walkthrough/ for next steps"
