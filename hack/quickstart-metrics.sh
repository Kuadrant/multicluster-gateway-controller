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

export KFILT="docker run --rm -i ryane/kfilt"

METRICS_FEDERATION=true

source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.quickstartEnv)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.kindUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.cleanupUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.deployUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.startUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.setupEnv)"

mkdir -p ${TMP_DIR}

MGC_REPO="github.com/kuadrant/multicluster-gateway-controller.git"
PROMETHEUS_DIR=${MGC_REPO}/config/prometheus
INGRESS_NGINX_DIR=${MGC_REPO}/config/ingress-nginx
PROMETHEUS_FOR_FEDERATION_DIR=${MGC_REPO}/config/prometheus-for-federation
THANOS_DIR=${MGC_REPO}/config/thanos

set -e pipefail

# Prompt user for any required env vars that have not been set
requiredENV

# Deploy ingress controller
deployIngressController ${KIND_CLUSTER_CONTROL_PLANE} ${INGRESS_NGINX_DIR}

# Deploy Prometheus in the hub too
deployPrometheusForFederation ${KIND_CLUSTER_CONTROL_PLANE} ${PROMETHEUS_FOR_FEDERATION_DIR}

# Deploy Thanos components in the hub
deployThanos ${KIND_CLUSTER_CONTROL_PLANE} ${THANOS_DIR}

# Deploy Prometheus components in the hub
${KUSTOMIZE_BIN} build ${PROMETHEUS_DIR} | kubectl apply -f -;\

# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}