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
source "${LOCAL_SETUP_DIR}"/.setupEnv
source "${LOCAL_SETUP_DIR}"/.kindUtils
source "${LOCAL_SETUP_DIR}"/.argocdUtils
source "${LOCAL_SETUP_DIR}"/.ocmUtils
source "${LOCAL_SETUP_DIR}"/.monitoringUtils

KIND_CLUSTER_PREFIX="mctc-"
KIND_CLUSTER_CONTROL_PLANE="${KIND_CLUSTER_PREFIX}control-plane"
KIND_WORKLOAD_CLUSTER_1="${KIND_CLUSTER_PREFIX}workload-cluster-1"

INGRESS_NGINX_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/ingress-nginx
CERT_MANAGER_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/cert-manager
EXTERNAL_DNS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/external-dns
ARGOCD_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/argocd

set -e pipefail

deployIngressController () {
  clusterName=${1}
  echo "Deploying Ingress controller to ${clusterName}"

  kubectl config use-context kind-${clusterName}

  ${KUSTOMIZE_BIN} build ${INGRESS_NGINX_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
  echo "Waiting for deployments to be ready ..."
  kubectl -n ingress-nginx wait --timeout=300s --for=condition=Available deployments --all
}

deployCertManager() {
  clusterName=${1}
  echo "Deploying Cert Manager to (${clusterName})"

  kubectl config use-context kind-${clusterName}

  ${KUSTOMIZE_BIN} build ${CERT_MANAGER_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
  echo "Waiting for Cert Manager deployments to be ready..."
  kubectl -n cert-manager wait --timeout=300s --for=condition=Available deployments --all
}

deployExternalDNS() {
  clusterName=${1}
  echo "Deploying ExternalDNS to (${clusterName})"

  kubectl config use-context kind-${clusterName}

  ${KUSTOMIZE_BIN} build ${EXTERNAL_DNS_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
  echo "Waiting for External DNS deployments to be ready..."
  kubectl -n external-dns wait --timeout=300s --for=condition=Available deployments --all
}

deployArgoCD() {
  clusterName=${1}
  echo "Deploying ArgoCD to (${clusterName})"

  kubectl config use-context kind-${clusterName}

  ${KUSTOMIZE_BIN} build ${ARGOCD_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
  echo "Waiting for ARGOCD deployments to be ready..."
  kubectl -n argocd wait --timeout=300s --for=condition=Available deployments --all

  ports=$(docker ps --format '{{json .}}' | jq "select(.Names == \"$clusterName-control-plane\").Ports")
  httpsport=$(echo $ports | sed -e 's/.*0.0.0.0\:\(.*\)->443\/tcp.*/\1/')
  argoPassword=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
  nodeIP=$(kubectl get nodes -o json | jq -r ".items[] | select(.metadata.name == \"$clusterName-control-plane\").status | .addresses[] | select(.type == \"InternalIP\").address")

  echo -ne "\n\n\tConnect to ArgoCD UI\n\n"
  echo -ne "\t\tLocal URL: https://argocd.127.0.0.1.nip.io:$httpsport\n"
  echo -ne "\t\tNode URL : https://argocd.$nodeIP.nip.io\n"
  echo -ne "\t\tUser     : admin\n"
  echo -ne "\t\tPassword : $argoPassword\n\n\n"
}

deployOCM() {
  hubClusterName=${1}
  managedClusterName=${2}
  echo "Deploying OCM hub to (${hubClusterName})"

  # Deploy the cluster manager
  ocmInitHub ${hubClusterName}

  # Register the control-plane cluster as a managed cluster and
  # and with ArgoCD
  ocmAddCluster ${hubClusterName} ${hubClusterName}
  argocdAddCluster ${hubClusterName} ${hubClusterName}

  # create a managed cluster with random mapped ports and
  # register it with OCM and ArgoCD
  kindCreateCluster ${KIND_WORKLOAD_CLUSTER_1} 0 0
  ocmAddCluster ${hubClusterName} ${managedClusterName}
  argocdAddCluster ${hubClusterName} ${managedClusterName}
}

deployMonitoring() {
  hubClusterName=${1}
  managedClusterName=${2}

	${KUSTOMIZE_BIN} build config/thanos | kubectl --context kind-${hubClusterName} apply -f -
  monitoringDeployKubePrometheus ${hubClusterName}
  monitoringDeployKubePrometheus ${managedClusterName}
}

deployMCR() {
  hubClusterName=${1}

  ${KUSTOMIZE_BIN} build config/multi-cluster-rollouts | kubectl --context kind-${hubClusterName} apply -f -
}

#Delete existing kind clusters
clusterCount=$(${KIND_BIN} get clusters | egrep "${KIND_CLUSTER_PREFIX}|${KIND_WORKLOAD_CLUSTER_1}" | wc -l)
if ! [[ $clusterCount =~ "0" ]] ; then
  echo "Deleting previous kind clusters."
  ${KIND_BIN} get clusters | egrep "${KIND_CLUSTER_PREFIX}|${KIND_WORKLOAD_CLUSTER_1}" | xargs ${KIND_BIN} delete clusters
fi

port80=8082
port443=8445

#1. Create Kind control plane cluster
kindCreateCluster $KIND_CLUSTER_CONTROL_PLANE $port80 $port443
#2. Deploy ingress controller
deployIngressController $KIND_CLUSTER_CONTROL_PLANE
#3. Deploy cert manager
deployCertManager $KIND_CLUSTER_CONTROL_PLANE
#4. Deploy external dns
deployExternalDNS $KIND_CLUSTER_CONTROL_PLANE
#5. Deploy argo cd
deployArgoCD $KIND_CLUSTER_CONTROL_PLANE
#6. Deploy OCM hub in the control plane cluster and add a managed kind cluster
deployOCM $KIND_CLUSTER_CONTROL_PLANE ${KIND_WORKLOAD_CLUSTER_1}
#7. Deploy the monitoring stack
deployMonitoring $KIND_CLUSTER_CONTROL_PLANE ${KIND_WORKLOAD_CLUSTER_1}
#8. Deploy the multi-cluster-rollouts controlelr
deployMCR $KIND_CLUSTER_CONTROL_PLANE
# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
