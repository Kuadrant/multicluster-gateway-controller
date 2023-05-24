#!/bin/bash

### Usage
### export KUBECONFIG=$(pwd)/tmp/kubeconfigs/mgc-control-plane.kubeconfig
### register-cluster.sh <PATH TO CONTROL CLUSTER KUBECONFIG> <PATH TO TENANT CLUSTER KUBECONFIG> <TENANT NAMESPACE> <NAME OF THE TENANT CLUSTER> > agent-manifests.yaml
###
### Outputs the manifests to deploy the syncer in the tenant cluster

LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/.setupEnv
source "${LOCAL_SETUP_DIR}"/.clusterUtils
source "${LOCAL_SETUP_DIR}"/.argocdUtils

controlClusterKubeconfig=$1
clusterKubeconfig=$2
tenantNs=$3
clusterName=$4

# Validate that Gateway API is installed in the tenant cluster
if ! kubectl --kubeconfig $clusterKubeconfig get crd/gateways.gateway.networking.k8s.io > /dev/null ; then
  echo "Gateway API not found in tenant cluster" > /dev/stderr
  exit 1
fi

# Create ArgoCD secret for cluster in control plane
makeSecretForKubeconfig $clusterKubeconfig $clusterName |
setNamespacedName argocd $clusterName |
setLabel argocd.argoproj.io/secret-type cluster |
kubectl apply -f - > /dev/null

# Create SA in tenant ns
cd ${LOCAL_SETUP_DIR}/../config/syncer-control
${KUSTOMIZE_BIN} edit set namespace $tenantNs
${KUSTOMIZE_BIN} build . | kubectl apply -f - > /dev/null
cd ../../

# Get the token for the SA
export saToken=$(kubectl get secret/syncer-token -n $tenantNs -o go-template="{{.data.token | base64decode}}")


# Create secret to access control cluster in workload cluster
makeSecretForKubeconfig $controlClusterKubeconfig kind-mgc-control-plane $clusterName |
setNamespacedName mgc-system control-plane-cluster-internal |
setLabel argocd.argoproj.io/secret-type cluster |
setConfig '.bearerToken=strenv(saToken)' > ${LOCAL_SETUP_DIR}/../config/syncer/secret.yaml

# Set the --control-plane-namespace flag in the agent deployment to the tenant
# namespace
${YQ_BIN} -i -P -o=json '.[1].value = "'$tenantNs'"' ${LOCAL_SETUP_DIR}/../config/syncer/syncer_parameter_patch.json

# Output deployment manifests
${KUSTOMIZE_BIN} build ${LOCAL_SETUP_DIR}/../config/syncer