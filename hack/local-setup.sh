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

KIND_CLUSTER_PREFIX="mctc-"
KIND_CLUSTER_CONTROL_PLANE="${KIND_CLUSTER_PREFIX}control-plane"

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

#Delete existing kind clusters
clusterCount=$(${KIND_BIN} get clusters | grep ${KIND_CLUSTER_PREFIX} | wc -l)
if ! [[ $clusterCount =~ "0" ]] ; then
  echo "Deleting previous kind clusters."
  ${KIND_BIN} get clusters | grep ${KIND_CLUSTER_PREFIX} | xargs ${KIND_BIN} delete clusters
fi

port80=8082
port443=8445

#1. Create Kind control plane cluster
kindCreateCluster $KIND_CLUSTER_CONTROL_PLANE $port80 $port443
#2. Deploy ingress controller
deployIngressController $KIND_CLUSTER_CONTROL_PLANE
#3. Deploy cert manager
deployCertManager $KIND_CLUSTER_CONTROL_PLANE
#4. Deploy argo cd
deployArgoCD $KIND_CLUSTER_CONTROL_PLANE
