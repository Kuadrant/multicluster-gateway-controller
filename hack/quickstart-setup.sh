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

export KIND_BIN=kind
export YQ_BIN=yq

source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.kindUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.clusterUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.cleanupUtils)"
source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/.startUtils)"


MGC_REPO="github.com/kuadrant/multicluster-gateway-controller.git"
QUICK_START_HUB_KUSTOMIZATION=${MGC_REPO}/config/quick-start/control-cluster
QUICK_START_SPOKE_KUSTOMIZATION=${MGC_REPO}/config/quick-start/workload-cluster

KIND_CLUSTER_PREFIX="mgc-"
KIND_CLUSTER_CONTROL_PLANE="${KIND_CLUSTER_PREFIX}control-plane"
KIND_CLUSTER_WORKLOAD="${KIND_CLUSTER_PREFIX}workload"

set -e pipefail


configureMetalLB () {
  metalLBSubnet=${2}
  echo "Creating MetalLB AddressPool"
  cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 172.31.${metalLBSubnet}.0/24
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
EOF
}





deployOLM(){
  clusterName=${1}
  
  kubectl config use-context kind-${clusterName}
  echo "Installing OLM in ${clusterName}"
  
  operator-sdk olm install --timeout 6m0s
}

deployOCMHub(){
  clusterName=${1}
  echo "installing the hub cluster in kind-(${clusterName}) "

  kubectl config use-context kind-${clusterName}
  clusteradm init --bundle-version='0.11.0' --wait --context kind-${clusterName}
  echo "PATCHING CLUSTERMANAGER: placement image patch to use amd64 image - See https://kubernetes.slack.com/archives/C01GE7YSUUF/p1685016272443249"
  kubectl patch clustermanager cluster-manager --type='merge' -p '{"spec":{"placementImagePullSpec":"quay.io/open-cluster-management/placement:v0.11.0-amd64"}}'
  echo "checking if cluster is single or multi"
  if [[ -n "${OCM_SINGLE}" ]]; then
    clusterName=kind-${KIND_CLUSTER_CONTROL_PLANE}
    echo "Found single cluster installing hub and spoke on the one cluster (${clusterName})"
    join=$(clusteradm get token --context ${clusterName} |  grep -o  'clusteradm.*--cluster-name')
    ${join} ${clusterName} --bundle-version='0.11.0' --feature-gates=RawFeedbackJsonString=true --force-internal-endpoint-lookup --context ${clusterName} | grep clusteradm
    echo "accepting OCM spoke cluster invite"
  
    max_retry=18
    counter=0
    until clusteradm accept --clusters ${clusterName}
    do
      sleep 10
      [[ counter -eq $max_retry ]] && echo "Failed!" && exit 1
      echo "Trying again. Try #$counter"
      ((++counter))
    done
    deployOLM ${KIND_CLUSTER_CONTROL_PLANE}
  fi
}

deployOCMSpoke(){
  clusterName=${1}
  echo "joining the spoke cluster to the hub cluster kind-(${KIND_CLUSTER_CONTROL_PLANE}),"
  kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
  join=$(clusteradm get token --context kind-${KIND_CLUSTER_CONTROL_PLANE} |  grep -o  'clusteradm.*--cluster-name')
  kubectl config use-context kind-${clusterName}
  ${join} kind-${clusterName} --bundle-version='0.11.0' --feature-gates=RawFeedbackJsonString=true --force-internal-endpoint-lookup --context kind-${clusterName} | grep clusteradm
  echo "accepting OCM spoke cluster invite"
  kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}
  
  max_retry=18
  counter=0
  until clusteradm accept --clusters kind-${clusterName}
  do
     sleep 10
     [[ counter -eq $max_retry ]] && echo "Failed!" && exit 1
     echo "Trying again. Try #$counter"
     ((++counter))
  done

}

configureController() {
    clusterName=${1}
    kubectl config use-context kind-${clusterName}
    echo "Initialize local dev setup for the controller on ${clusterName}"

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${KIND_CLUSTER_PREFIX}aws-credentials
  namespace: multi-cluster-gateways
type: "kuadrant.io/aws"
stringData:
  AWS_ACCESS_KEY_ID: ${MGC_AWS_ACCESS_KEY_ID}
  AWS_SECRET_ACCESS_KEY: ${MGC_AWS_SECRET_ACCESS_KEY}
  AWS_REGION: ${MGC_AWS_REGION}
EOF

    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${KIND_CLUSTER_PREFIX}controller-config
  namespace: multi-cluster-gateways
data:
  AWS_DNS_PUBLIC_ZONE_ID: ${MGC_AWS_DNS_PUBLIC_ZONE_ID}
  ZONE_ROOT_DOMAIN: ${MGC_ZONE_ROOT_DOMAIN}
  LOG_LEVEL: "${LOG_LEVEL}"
EOF

    cat <<EOF | kubectl apply -f -
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: ${KIND_CLUSTER_PREFIX}dev-mz
  namespace: multi-cluster-gateways
spec:
  id: ${MGC_AWS_DNS_PUBLIC_ZONE_ID}
  domainName: ${MGC_ZONE_ROOT_DOMAIN}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: ${KIND_CLUSTER_PREFIX}aws-credentials
    namespace: multi-cluster-gateways
    type: AWS
EOF
}

deployQuickStartControl() {
    clusterName=${1}
    kubectl config use-context kind-${clusterName}
    echo "Initialize quickstart setup on ${clusterName}"
    wait_for "kustomize --load-restrictor LoadRestrictionsNone build ${QUICK_START_HUB_KUSTOMIZATION} --enable-helm --helm-command helm | kubectl apply -f -" "${QUICK_START_HUB_KUSTOMIZATION} cluster config apply" "1m" "5"
    echo "Waiting for metallb-system deployments to be ready"
    kubectl -n metallb-system wait --for=condition=ready pod --selector=app=metallb --timeout=300s
    echo "Waiting for cert-manager deployments to be ready"
    kubectl -n cert-manager wait --timeout=300s --for=condition=Available deployments --all
    echo "Waiting for istio deployments to be ready"
    kubectl -n istio-operator wait --timeout=300s --for=condition=Available deployments --all
}

deployQuickStartWorkload() {
   clusterName=${1}
    kubectl config use-context kind-${clusterName}
    echo "Initialize quickstart setup on ${clusterName}"
    wait_for "kustomize --load-restrictor LoadRestrictionsNone build ${QUICK_START_SPOKE_KUSTOMIZATION} --enable-helm --helm-command helm | kubectl apply -f -" "${QUICK_START_SPOKE_KUSTOMIZATION} cluster config apply" "1m" "5"
    echo "Waiting for metallb-system deployments to be ready"
    kubectl -n metallb-system wait --for=condition=ready pod --selector=app=metallb --timeout=300s
}

configureControlCluster() {
    clusterName=${1}
    # Ensure the current context points to the control plane cluster
    kubectl config use-context kind-${clusterName}
    kubectl label managedcluster kind-mgc-control-plane ingress-cluster=true
}


# Prompt user for any required env vars that have not been set
if [[ -z "${MGC_AWS_ACCESS_KEY_ID}" ]]; then
  echo "Enter an AWS secret access key id for an Account where you have access to Route53:"
  read MGC_AWS_ACCESS_KEY_ID </dev/tty
  echo "export MGC_AWS_ACCESS_KEY_ID for future executions of the script to skip this step"
fi

if [[ -z "${MGC_AWS_SECRET_ACCESS_KEY}" ]]; then
  echo "Enter your AWS secret access key id for an Account where you have access to Route53:"
  read MGC_AWS_SECRET_ACCESS_KEY </dev/tty
  echo "export MGC_AWS_SECRET_ACCESS_KEY for future executions of the script to skip this step"
fi

if [[ -z "${MGC_AWS_REGION}" ]]; then
  echo "Enter an AWS region (e.g. eu-west-1) for an Account where you have access to Route53:"
  read MGC_AWS_REGION </dev/tty
  echo "export MGC_AWS_REGION for future executions of the script to skip this step"
fi

if [[ -z "${MGC_AWS_DNS_PUBLIC_ZONE_ID}" ]]; then
  echo "Enter the Public Zone ID of your Route53 zone:"
  read MGC_AWS_DNS_PUBLIC_ZONE_ID </dev/tty
  echo "export MGC_AWS_DNS_PUBLIC_ZONE_ID for future executions of the script to skip this step"
fi

if [[ -z "${MGC_ZONE_ROOT_DOMAIN}" ]]; then
  echo "Enter the root domain of your Route53 hosted zone (e.g. www.example.com):"
  read MGC_ZONE_ROOT_DOMAIN </dev/tty
  echo "export MGC_ZONE_ROOT_DOMAIN for future executions of the script to skip this step"
fi

# Default config
if [[ -z "${LOG_LEVEL}" ]]; then
  LOG_LEVEL=1
fi
if [[ -z "${OCM_SINGLE}" ]]; then
  OCM_SINGLE=true
fi
if [[ -z "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  MGC_WORKLOAD_CLUSTERS_COUNT=1
fi

cleanup

port80=9090
port443=8445
proxyPort=9200
metalLBSubnetStart=200

# Create network for the clusters
docker network create -d bridge --subnet 172.31.0.0/16 mgc --gateway 172.31.0.1 \
  -o "com.docker.network.bridge.enable_ip_masquerade"="true" \
  -o "com.docker.network.driver.mtu"="1500"

# Create Kind control plane cluster
kindCreateCluster ${KIND_CLUSTER_CONTROL_PLANE} ${port80} ${port443}

# Create Kind workload cluster(s)
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    kindCreateCluster ${KIND_CLUSTER_WORKLOAD}-${i} $((${port80} + ${i})) $((${port443} + ${i})) $((${i} + 1))
  done
fi

# Apply Cluster Configurations to Control cluster
# Deploy OCM hub
deployOCMHub ${KIND_CLUSTER_CONTROL_PLANE}
# Deploy Quick start kustomize
deployQuickStartControl ${KIND_CLUSTER_CONTROL_PLANE}
# Initialize local dev setup for the controller on the control-plane cluster
configureController ${KIND_CLUSTER_CONTROL_PLANE}
# Deploy MetalLb
configureMetalLB ${KIND_CLUSTER_CONTROL_PLANE} ${metalLBSubnetStart}
configureControlCluster ${KIND_CLUSTER_CONTROL_PLANE}


# Apply Cluster Configurations to Workload clusters
if [[ -n "${MGC_WORKLOAD_CLUSTERS_COUNT}" ]]; then
  for ((i = 1; i <= ${MGC_WORKLOAD_CLUSTERS_COUNT}; i++)); do
    deployQuickStartWorkload ${KIND_CLUSTER_WORKLOAD}-${i}
    deployOLM ${KIND_CLUSTER_WORKLOAD}-${i}
    deployOCMSpoke ${KIND_CLUSTER_WORKLOAD}-${i}
  done
fi


kubectl config use-context kind-${KIND_CLUSTER_CONTROL_PLANE}


echo ""
echo "What's next...

      Now that you have 2 kind clusters configured and with multicluster-gateway-controller installed you are ready to begin creating gateways
      Visit https://docs.kuadrant.io/multicluster-gateway-controller/docs/how-to/ocm-control-plane-walkthrough/#create-a-gateway for next steps"