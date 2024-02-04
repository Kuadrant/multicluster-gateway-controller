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
source "${LOCAL_SETUP_DIR}"/.clusterUtils
source "${LOCAL_SETUP_DIR}"/.argocdUtils
source "${LOCAL_SETUP_DIR}"/.cleanupUtils
source "${LOCAL_SETUP_DIR}"/.deployUtils

export TMP_DIR=./tmp

INGRESS_NGINX_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/ingress-nginx
METALLB_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/metallb
CERT_MANAGER_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/cert-manager
EXTERNAL_DNS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/external-dns
ARGOCD_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/argocd
ISTIO_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/istio/istio-operator.yaml
GATEWAY_API_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/gateway-api
REDIS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/kuadrant/redis
THANOS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/thanos
PROMETHEUS_FOR_FEDERATION_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/prometheus-for-federation

set -e pipefail

cleanupMGC

# Deploy the submariner broker to cluster 1
#deploySubmarinerBroker ${KIND_CLUSTER_CONTROL_PLANE}

# Join cluster 1 to the submariner broker
#joinSubmarinerBroker ${KIND_CLUSTER_CONTROL_PLANE}

deployIstio ${KIND_CLUSTER_CONTROL_PLANE}

# Install the Gateway API CRDs in the control cluster
installGatewayAPI ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy ingress controller
deployIngressController ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy cert manager
deployCertManager ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy argo cd
#deployArgoCD ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Dashboard
#deployDashboard $KIND_CLUSTER_CONTROL_PLANE 0

# Add the control plane cluster
#argocdAddCluster ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_CONTROL_PLANE}

# Initialize local dev setup for the controller on the control-plane cluster
initController ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Redis
#deployRedis ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy MetalLb
deployMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}

# Deploy Prometheus in the hub too
#deployPrometheusForFederation ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Thanos components in the hub
#deployThanos ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy to workload clusters if MGC_WORKLOAD_CLUSTERS_COUNT environment variable is set
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
#    joinSubmarinerBroker ${KIND_CLUSTER_WORKLOAD}-${i}
#    deployIstio ${KIND_CLUSTER_WORKLOAD}-${i}
    installGatewayAPI ${KIND_CLUSTER_WORKLOAD}-${i}
    deployIngressController ${KIND_CLUSTER_WORKLOAD}-${i}
    deployMetalLB ${KIND_CLUSTER_WORKLOAD}-${i} $((${metalLBSubnetStart} + ${i}))
    deployOLM ${KIND_CLUSTER_WORKLOAD}-${i}
#    deployDashboard ${KIND_CLUSTER_WORKLOAD}-${i} ${i}
#    argocdAddCluster ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
#    deployAgentSecret ${KIND_CLUSTER_WORKLOAD}-${i} "true"
#    deployAgentSecret ${KIND_CLUSTER_WORKLOAD}-${i} "false"
    deployOCMSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
#    deployPrometheusForFederation ${KIND_CLUSTER_WORKLOAD}-${i}
    configureManagedAddon ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
  done
fi

# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}

# Create configmap with gateway parameters for clusters
kubectl create configmap gateway-params \
  --from-file=params=config/samples/gatewayclass_params.json \
  -n multi-cluster-gateways
