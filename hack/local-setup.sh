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

KIND_CLUSTER_PREFIX="mctc-"
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
  kubectl -n metallb-system wait --for=condition=ready pod --selector=app=metallb --timeout=90s
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

  kubectl delete validatingWebhookConfiguration mctc-cert-manager-webhook
  kubectl delete mutatingWebhookConfiguration mctc-cert-manager-webhook
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

deployKuadrant(){
  clusterName=${1}

  kubectl config use-context kind-mctc-control-plane
  echo "Installing Redis in kind-mctc-control-plane"
  ${KUSTOMIZE_BIN} build ${REDIS_KUSTOMIZATION_DIR} | kubectl apply -f -

  kubectl config use-context kind-${clusterName}
  echo "Installing Kuadrant in ${clusterName}"
  ${KUSTOMIZE_BIN} build config/kuadrant | kubectl apply -f -
  echo "Configuring Limitador in ${clusterName}"
  ${KUSTOMIZE_BIN} build ${LIMITADOR_KUSTOMIZATION_DIR} | kubectl apply -f -
  echo "Waiting for Kuadrant deployments to be ready..."
  kubectl -n kuadrant-system wait --timeout=300s --for=condition=Available deployments --all
  echo "Adding Kuadrant CR in ${clusterName}"
  ${KUSTOMIZE_BIN} build config/kuadrant/cr | kubectl apply -f -
  # This is adding the kuadrant CR before any gateways are created which is currently a known issue with kuadrant.
  # After adding a gateway is will necessary to force a reconcile of the kuadrant CR, this can be done by running:
  # kubectl annotate kuadrant/mctc -n kuadrant-system updateme=`date +%s` --overwrite
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

  kubectl create namespace mctc-system || true

  makeSecretForCluster $KIND_CLUSTER_CONTROL_PLANE $clusterName $localAccess |
  setNamespacedName mctc-system ${secretName} |
  setLabel argocd.argoproj.io/secret-type cluster |
  kubectl apply -f -
}

deployOCMHub(){
  clusterName=${1}
  echo "installing the hub cluster in kind-(${clusterName}) "

  ${CLUSTERADM_BIN} init --wait --context kind-${clusterName}
  echo "checking if cluster is single or multi"
  if [[ -n "${OCM_SINGLE}" ]]; then
    clusterName=kind-${KIND_CLUSTER_CONTROL_PLANE}
    echo "Found single cluster installing hub and spoke on the one cluster (${clusterName})"
    join=$(${CLUSTERADM_BIN} get token --context ${clusterName} |  grep -o  'clusteradm.*--cluster-name')
    ${BIN_DIR}/${join} ${clusterName} --force-internal-endpoint-lookup --context ${clusterName} | grep clusteradm
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
  
}
deployOCMSpoke(){
  
  clusterName=${1}
  echo "joining the spoke cluster to the hub cluster kind-(${KIND_CLUSTER_CONTROL_PLANE}),"
  kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
  join=$(${CLUSTERADM_BIN} get token --context kind-${KIND_CLUSTER_CONTROL_PLANE} |  grep -o  'clusteradm.*--cluster-name')
  kubectl config use-context kind-${clusterName}
  ${BIN_DIR}/${join} kind-${clusterName} --force-internal-endpoint-lookup --context kind-${clusterName} | grep clusteradm
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

    # Add the mctc CRDs
    ${KUSTOMIZE_BIN} build config/crd | kubectl apply -f -
    # Add the kuadrant.io ratelimitpolicies CRD
    ${KUSTOMIZE_BIN} build config/kuadrant/crd | kubectl apply -f -
    # Create the mctc ns and dev managed zone
    ${KUSTOMIZE_BIN} --reorder none --load-restrictor LoadRestrictionsNone build config/local-setup/controller | kubectl apply -f -
}

cleanup

port80=9090
port443=8445
proxyPort=9200
metalLBSubnetStart=200

# Create network for the clusters
docker network create -d bridge --subnet 172.32.0.0/16 mctc --gateway 172.32.0.1 \
  -o "com.docker.network.bridge.enable_ip_masquerade"="true" \
  -o "com.docker.network.driver.mtu"="1500"

#1. Create Kind control plane cluster
kindCreateCluster ${KIND_CLUSTER_CONTROL_PLANE} ${port80} ${port443}

#2. Install the Gateway API CRDs in the control cluster
installGatewayAPI ${KIND_CLUSTER_CONTROL_PLANE}

#3. Deploy ingress controller
deployIngressController ${KIND_CLUSTER_CONTROL_PLANE}

#4. Deploy cert manager
deployCertManager ${KIND_CLUSTER_CONTROL_PLANE}

#5. Deploy argo cd
deployArgoCD ${KIND_CLUSTER_CONTROL_PLANE}

#6. Deploy Dashboard
deployDashboard $KIND_CLUSTER_CONTROL_PLANE 0

#7. Add the control plane cluster
argocdAddCluster ${KIND_CLUSTER_CONTROL_PLANE} ${KIND_CLUSTER_CONTROL_PLANE}

#8. Initialize local dev setup for the controller on the control-plane cluster
initController ${KIND_CLUSTER_CONTROL_PLANE}

# 9. Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE}

deployMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}

#9. Add workload clusters if MCTC_WORKLOAD_CLUSTERS_COUNT environment variable is set
if [[ -n "${MCTC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MCTC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    kindCreateCluster ${KIND_CLUSTER_WORKLOAD}-${i} $((${port80} + ${i})) $((${port443} + ${i}))
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
  done
fi

# 10. Ensure the current context points to the control plane cluster
kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}

 # Create configmap with gateway parameters for clusters
kubectl create configmap gateway-params \
  --from-file=params=config/samples/gatewayclass_params.json \
  -n multi-cluster-gateways