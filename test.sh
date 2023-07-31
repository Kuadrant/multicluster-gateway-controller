#!/bin/bash

TOOLS_IMAGE=quay.io/kuadrant/mgc-tools:latest

docker build . -t ${TOOLS_IMAGE} -f ./Dockerfile.tools

#Assuming you have ran local-setup with `make local-setup OCM_SINGLE=true MGC_WORKLOAD_CLUSTERS_COUNT=2`
mkdir -p /tmp/mgc
# Export the kubeconfigs of each cluster with `--internal` so they can be accessed by each other
kind export kubeconfig -n mgc-workload-2 --kubeconfig /tmp/mgc/kubeconfig --internal
kind export kubeconfig -n mgc-workload-1 --kubeconfig /tmp/mgc/kubeconfig --internal
kind export kubeconfig -n mgc-control-plane --kubeconfig /tmp/mgc/kubeconfig --internal

docker run --rm "${TOOLS_IMAGE}" -c 'kustomize version'
docker run --rm "${TOOLS_IMAGE}" -c 'operator-sdk version'
docker run --rm "${TOOLS_IMAGE}" -c 'kind --version'
docker run --rm "${TOOLS_IMAGE}" -c 'helm version'
docker run --rm "${TOOLS_IMAGE}" -c 'yq --version'
docker run --rm "${TOOLS_IMAGE}" -c 'istioctl version'

# Requires access to kube server of cluster running on host so mount the local kubeconfig directory, set the same network, and set KUBECONFIG
docker run -u $UID -v "/tmp/mgc:/tmp/mgc:z" --network mgc -e KUBECONFIG='/tmp/mgc/kubeconfig' --rm "${TOOLS_IMAGE}" -c 'clusteradm version'

docker run -u $UID -v "/tmp/mgc:/tmp/mgc:z" --network mgc -e KUBECONFIG='/tmp/mgc/kubeconfig' --rm "quay.io/kuadrant/mgc-tools:latest" -c 'kubectl --context kind-mgc-control-plane get deployments -A'
docker run -u $UID -v "/tmp/mgc:/tmp/mgc:z" --network mgc -e KUBECONFIG='/tmp/mgc/kubeconfig' --rm "quay.io/kuadrant/mgc-tools:latest" -c 'kubectl --context kind-mgc-workload-1 get deployments -A'
docker run -u $UID -v "/tmp/mgc:/tmp/mgc:z" --network mgc -e KUBECONFIG='/tmp/mgc/kubeconfig' --rm "quay.io/kuadrant/mgc-tools:latest" -c 'kubectl --context kind-mgc-workload-2 get deployments -A'

docker run --rm "${TOOLS_IMAGE}" -c 'kustomize build github.com/Kuadrant/multicluster-gateway-controller/config/cert-manager?ref=main --enable-helm --helm-command helm'

docker run -u $UID -v "/tmp/mgc:/tmp/mgc:z" --network mgc -e KUBECONFIG='/tmp/mgc/kubeconfig' --rm "quay.io/kuadrant/mgc-tools:latest" -c 'operator-sdk olm status'
