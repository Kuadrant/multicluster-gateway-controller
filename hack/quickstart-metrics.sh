#!/bin/bash

#
# Copyright 2023 Red Hat, Inc.
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

export KFILT="$CONTAINER_RUNTIME_BIN run --rm -i ryane/kfilt"

METRICS_FEDERATION=true

if [ -z $MGC_BRANCH ]; then
  MGC_BRANCH=${MGC_BRANCH:="main"}
fi
if [ -z $MGC_ACCOUNT ]; then
  MGC_ACCOUNT=${MGC_ACCOUNT:="kuadrant"}
fi

source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.quickstartEnv)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.kindUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.cleanupUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.deployUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.startUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/${MGC_ACCOUNT}/multicluster-gateway-controller/${MGC_BRANCH}/hack/.setupEnv)"

mkdir -p ${TMP_DIR}

MGC_REPO=${MGC_REPO:="github.com/${MGC_ACCOUNT}/multicluster-gateway-controller.git"}
PROMETHEUS_DIR=${MGC_REPO}/config/prometheus
INGRESS_NGINX_DIR=${MGC_REPO}/config/ingress-nginx
PROMETHEUS_FOR_FEDERATION_DIR=${MGC_REPO}/config/prometheus-for-federation
THANOS_DIR=${MGC_REPO}/config/thanos

set -e pipefail

# Prompt user for any required env vars that have not been set
requiredENV

if [[ -z "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  MGC_WORKLOAD_CLUSTERS_COUNT=1
fi

# Deploy ingress controller
deployIngressController ${KIND_CLUSTER_CONTROL_PLANE} ${INGRESS_NGINX_DIR}

# Deploy Prometheus in the hub too
deployPrometheusForFederation ${KIND_CLUSTER_CONTROL_PLANE} ${PROMETHEUS_FOR_FEDERATION_DIR}?ref=${MGC_BRANCH}

# Deploy Thanos components in the hub
deployThanos ${KIND_CLUSTER_CONTROL_PLANE} ${THANOS_DIR}

# Deploy Prometheus components in the hub
deployPrometheus ${KIND_CLUSTER_CONTROL_PLANE}

# Apply Cluster Configurations to Workload clusters
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    deployPrometheusForFederation ${KIND_CLUSTER_WORKLOAD}-${i} ${PROMETHEUS_FOR_FEDERATION_DIR}?ref=${MGC_BRANCH}
  done
fi

# Restarts the metrics to make sure all resources exists before ksm starts.
# https://github.com/kubernetes/kube-state-metrics/issues/2142
kubectl delete pods -n monitoring -l app.kubernetes.io/name=kube-state-metrics

# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}