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
source "${LOCAL_SETUP_DIR}"/.clusterUtils
source "${LOCAL_SETUP_DIR}"/.argocdUtils
source "${LOCAL_SETUP_DIR}"/.cleanupUtils

KIND_CLUSTER_PREFIX="mgc-"
KIND_CLUSTER_CONTROL_PLANE="${KIND_CLUSTER_PREFIX}control-plane"
KIND_CLUSTER_WORKLOAD="${KIND_CLUSTER_PREFIX}workload"

INGRESS_NGINX_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/ingress-nginx
METALLB_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/metallb
CERT_MANAGER_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/cert-manager
EXTERNAL_DNS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/external-dns
ARGOCD_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/argocd
ISTIO_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/istio/istio-operator.yaml
GATEWAY_API_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/gateway-api
REDIS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/kuadrant/redis
LIMITADOR_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/kuadrant/limitador
THANOS_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/thanos
PROMETHEUS_FOR_FEDERATION_KUSTOMIZATION_DIR=${LOCAL_SETUP_DIR}/../config/prometheus-for-federation

TLS_CERT_PATH=${LOCAL_SETUP_DIR}/../config/webhook-setup/control/tls

set -e pipefail

deployIngressController () {
  clusterName=${1}
  kubectl config use-context kind-${clusterName}
  echo "Deploying Ingress controller to ${clusterName}"
  ${KUSTOMIZE_BIN} build ${INGRESS_NGINX_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
  echo "Waiting for deployments to be ready ..."
  kubectl -n ingress-nginx wait --timeout=600s --for=condition=Available deployments --all
}

deployMetalLB () {
  clusterName=${1}
  metalLBSubnet=${2}

  kubectl config use-context kind-${clusterName}
  echo "Deploying MetalLB to ${clusterName}"
  ${KUSTOMIZE_BIN} build ${METALLB_KUSTOMIZATION_DIR} | kubectl apply -f -
  echo "Waiting for deployments to be ready ..."
  kubectl -n metallb-system wait --for=condition=ready pod --selector=app=metallb --timeout=300s
  echo "Creating MetalLB AddressPool"
  cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 172.32.${metalLBSubnet}.0/24
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
EOF
}

deployCertManager() {
  clusterName=${1}
  echo "Deploying Cert Manager to (${clusterName})"

  kubectl config use-context kind-${clusterName}

  ${KUSTOMIZE_BIN} build ${CERT_MANAGER_KUSTOMIZATION_DIR} --enable-helm --helm-command ${HELM_BIN} | kubectl apply -f -
  echo "Waiting for Cert Manager deployments to be ready..."
  kubectl -n cert-manager wait --timeout=300s --for=condition=Available deployments --all

  kubectl delete validatingWebhookConfiguration mgc-cert-manager-webhook
  kubectl delete mutatingWebhookConfiguration mgc-cert-manager-webhook
  # Apply the default glbc-ca issuer
  kubectl create -n cert-manager -f ./config/default/issuer.yaml
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

deployIstio() {
  clusterName=${1}
  echo "Deploying Istio to (${clusterName})"

  kubectl config use-context kind-${clusterName}
  ${ISTIOCTL_BIN} operator init
	kubectl apply -f  ${ISTIO_KUSTOMIZATION_DIR}
}

installGatewayAPI() {
  clusterName=${1}
  kubectl config use-context kind-${clusterName}
  echo "Installing Gateway API in ${clusterName}"

  ${KUSTOMIZE_BIN} build ${GATEWAY_API_KUSTOMIZATION_DIR} | kubectl apply -f -
}

deployOLM(){
  clusterName=${1}
  
  kubectl config use-context kind-${clusterName}
  echo "Installing OLM in ${clusterName}"
  
  ${OPERATOR_SDK_BIN} olm install --timeout 6m0s
}

deployRedis(){
  clusterName=${1}

  kubectl config use-context kind-${clusterName}
  echo "Installing Redis in kind-${clusterName}"
  ${KUSTOMIZE_BIN} build ${REDIS_KUSTOMIZATION_DIR} | kubectl apply -f -
}

deployDashboard() {
  clusterName=${1}
  portOffset=${2}

  echo "Deploying Kubernetes Dashboard to (${clusterName})"

  kubectl config use-context kind-${clusterName}

  kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml
  ${KUSTOMIZE_BIN} build config/dashboard | kubectl apply -f -

  kubectl wait --timeout=-30s --for=condition=Available deployment kubernetes-dashboard -n kubernetes-dashboard

  token=$(kubectl get secret/admin-user-token -n kubernetes-dashboard -o go-template="{{.data.token | base64decode}}")

  port=$((proxyPort + portOffset))

  kubectl proxy --context kind-${clusterName} --port ${port} &
  proxyPID=$!
  echo $proxyPID >> /tmp/dashboard_pids

  echo -ne "\n\n\tAccess Kubernetes Dashboard\n\n"
  echo -ne "\t\t\t* The dashboard is available at http://localhost:$port/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/\n"
  echo -ne "\t\tAccess the dashboard using the following Bearer Token: $token\n"
}

deployAgentSecret() {
  clusterName=${1}
  localAccess=${2:=LOCAL_ACCESS}
  if [ $localAccess == "true" ]; then
    secretName=control-plane-cluster
  else
    secretName=control-plane-cluster-internal
  fi
  echo "Deploying the agent secret to (${clusterName})"

  kubectl config use-context kind-${clusterName}

  kubectl create namespace mgc-system || true

  makeSecretForCluster $KIND_CLUSTER_CONTROL_PLANE $clusterName $localAccess |
  setNamespacedName mgc-system ${secretName} |
  setLabel argocd.argoproj.io/secret-type cluster |
  kubectl apply -f -
}

deployOCMHub(){
  clusterName=${1}
  echo "installing the hub cluster in kind-(${clusterName}) "

  ${CLUSTERADM_BIN} init --bundle-version='0.11.0' --wait --context kind-${clusterName}
  echo "PATCHING CLUSTERMANAGER: placement image patch to use amd64 image - See https://kubernetes.slack.com/archives/C01GE7YSUUF/p1685016272443249"
  kubectl patch clustermanager cluster-manager --type='merge' -p '{"spec":{"placementImagePullSpec":"quay.io/open-cluster-management/placement:v0.11.0-amd64"}}'
  echo "checking if cluster is single or multi"
  if [[ -n "${OCM_SINGLE}" ]]; then
    clusterName=kind-${KIND_CLUSTER_CONTROL_PLANE}
    echo "Found single cluster installing hub and spoke on the one cluster (${clusterName})"
    join=$(${CLUSTERADM_BIN} get token --context ${clusterName} |  grep -o  'clusteradm.*--cluster-name')
    ${BIN_DIR}/${join} ${clusterName} --bundle-version='0.11.0' --feature-gates=RawFeedbackJsonString=true --force-internal-endpoint-lookup --context ${clusterName} | grep clusteradm
    echo "accepting OCM spoke cluster invite"
  
    max_retry=18
    counter=0
    until ${CLUSTERADM_BIN} accept --clusters ${clusterName}
    do
      sleep 10
      [[ counter -eq $max_retry ]] && echo "Failed!" && exit 1
      echo "Trying again. Try #$counter"
      ((++counter))
    done
    deployOLM ${KIND_CLUSTER_CONTROL_PLANE}
    deployIstio ${KIND_CLUSTER_CONTROL_PLANE}
  fi
  echo "Installing Redis in kind-mgc-control-plane"
  ${KUSTOMIZE_BIN} build ${REDIS_KUSTOMIZATION_DIR} | kubectl apply -f -
  
}
deployOCMSpoke(){
  
  clusterName=${1}
  echo "joining the spoke cluster to the hub cluster kind-(${KIND_CLUSTER_CONTROL_PLANE}),"
  kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
  join=$(${CLUSTERADM_BIN} get token --context kind-${KIND_CLUSTER_CONTROL_PLANE} |  grep -o  'clusteradm.*--cluster-name')
  kubectl config use-context kind-${clusterName}
  ${BIN_DIR}/${join} kind-${clusterName} --bundle-version='0.11.0' --feature-gates=RawFeedbackJsonString=true --force-internal-endpoint-lookup --context kind-${clusterName} | grep clusteradm
  echo "accepting OCM spoke cluster invite"
  kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
  
  max_retry=18
  counter=0
  until ${CLUSTERADM_BIN} accept --clusters kind-${clusterName}
  do
     sleep 10
     [[ counter -eq $max_retry ]] && echo "Failed!" && exit 1
     echo "Trying again. Try #$counter"
     ((++counter))
  done

}

initController() {
    clusterName=${1}
    kubectl config use-context kind-${clusterName}
    echo "Initialize local dev setup for the controller on ${clusterName}"

    # Add the mgc CRDs
    ${KUSTOMIZE_BIN} build config/crd | kubectl apply -f -
    # Create the mgc ns and dev managed zone
    ${KUSTOMIZE_BIN} --reorder none --load-restrictor LoadRestrictionsNone build config/local-setup/controller | kubectl apply -f -
}

deploySubmarinerBroker() {
  clusterName=${1}
  if [[ -n "${SUBMARINER}" ]]; then
    ${SUBCTL_BIN} deploy-broker --kubeconfig ./tmp/kubeconfigs/external/${clusterName}.kubeconfig
  fi
}

joinSubmarinerBroker() {
  clusterName=${1}
  if [[ -n "${SUBMARINER}" ]]; then
    ${SUBCTL_BIN} join --kubeconfig ./tmp/kubeconfigs/external/${clusterName}.kubeconfig broker-info.subm --clusterid ${clusterName} --natt=false --check-broker-certificate=false
  fi
}

deployThanos() {
  clusterName=${1}
  if [[ -n "${METRICS_FEDERATION}" ]]; then
    echo "Deploying Thanos in ${clusterName}"
    kubectl config use-context kind-${clusterName}
    ${KUSTOMIZE_BIN} build ${THANOS_KUSTOMIZATION_DIR} | kubectl apply -f -

    nodeIP=$(kubectl get nodes -o json | jq -r ".items[] | select(.metadata.name == \"$clusterName-control-plane\").status | .addresses[] | select(.type == \"InternalIP\").address")
    echo -ne "\n\n\tConnect to Thanos Query UI\n\n"
    echo -ne "\t\tURL : https://thanos-query.$nodeIP.nip.io\n\n\n"
    echo -ne "\n\n\tConnect to Grafana UI\n\n"
    echo -ne "\t\tURL : https://grafana.$nodeIP.nip.io\n\n\n"
  fi
}

deployPrometheusForFederation() {
  clusterName=${1}
  if [[ -n "${METRICS_FEDERATION}" ]]; then
    echo "Deploying Prometheus for federation in ${clusterName}"
    kubectl config use-context kind-${clusterName}
    # Use server-side apply to avoid below error if re-running apply
    #   'The CustomResourceDefinition "prometheuses.monitoring.coreos.com" is invalid: metadata.annotations: Too long: must have at most 262144 bytes'
    # Also need to apply the CRDs first to avoid the below error types that seem to be timing related
    #   'resource mapping not found for name: "alertmanager-main-rules" namespace: "monitoring" from "STDIN": no matches for kind "PrometheusRule" in version "monitoring.coreos.com/v1"''
    ${KUSTOMIZE_BIN} build ${PROMETHEUS_FOR_FEDERATION_KUSTOMIZATION_DIR} | ${KFILT} -i kind=CustomResourceDefinition | kubectl apply --server-side -f -
    # Apply remainder of resources
    ${KUSTOMIZE_BIN} build ${PROMETHEUS_FOR_FEDERATION_KUSTOMIZATION_DIR} | ${KFILT} -x kind=CustomResourceDefinition | kubectl apply -f -
  fi
}

cleanup

port80=9090
port443=8445
proxyPort=9200
metalLBSubnetStart=200

# Create network for the clusters
docker network create -d bridge --subnet 172.32.0.0/16 mgc --gateway 172.32.0.1 \
  -o "com.docker.network.bridge.enable_ip_masquerade"="true" \
  -o "com.docker.network.driver.mtu"="1500"

# Create Kind control plane cluster
kindCreateCluster ${KIND_CLUSTER_CONTROL_PLANE} ${port80} ${port443}

# Deploy the submariner broker to cluster 1
deploySubmarinerBroker ${KIND_CLUSTER_CONTROL_PLANE}

# Join cluster 1 to the submariner broker
joinSubmarinerBroker ${KIND_CLUSTER_CONTROL_PLANE}

# Install the Gateway API CRDs in the control cluster
installGatewayAPI ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy ingress controller
deployIngressController ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy cert manager
deployCertManager ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy argo cd
deployArgoCD ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Dashboard
deployDashboard $KIND_CLUSTER_CONTROL_PLANE 0

# Add the control plane cluster
argocdAddCluster ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_CONTROL_PLANE}

# Initialize local dev setup for the controller on the control-plane cluster
initController ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Redis
deployRedis ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy MetalLb
deployMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}

# Deploy Prometheus in the hub too
deployPrometheusForFederation ${KIND_CLUSTER_CONTROL_PLANE}

# Deploy Thanos components in the hub
deployThanos ${KIND_CLUSTER_CONTROL_PLANE}

# Add workload clusters if MGC_WORKLOAD_CLUSTERS_COUNT environment variable is set
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    kindCreateCluster ${KIND_CLUSTER_WORKLOAD}-${i} $((${port80} + ${i})) $((${port443} + ${i})) $((${i} + 1))
    joinSubmarinerBroker ${KIND_CLUSTER_WORKLOAD}-${i}
    deployIstio ${KIND_CLUSTER_WORKLOAD}-${i}
    installGatewayAPI ${KIND_CLUSTER_WORKLOAD}-${i}
    deployIngressController ${KIND_CLUSTER_WORKLOAD}-${i}
    deployMetalLB ${KIND_CLUSTER_WORKLOAD}-${i} $((${metalLBSubnetStart} + ${i}))
    deployOLM ${KIND_CLUSTER_WORKLOAD}-${i}
    deployDashboard ${KIND_CLUSTER_WORKLOAD}-${i} ${i}
    argocdAddCluster ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_WORKLOAD}-${i}
    deployAgentSecret ${KIND_CLUSTER_WORKLOAD}-${i} "true"
    deployAgentSecret ${KIND_CLUSTER_WORKLOAD}-${i} "false"
    deployOCMSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
    deployPrometheusForFederation ${KIND_CLUSTER_WORKLOAD}-${i}
  done
fi

# Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}

# Create configmap with gateway parameters for clusters
kubectl create configmap gateway-params \
  --from-file=params=config/samples/gatewayclass_params.json \
  -n multi-cluster-gateways