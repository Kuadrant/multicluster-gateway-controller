export KIND_BIN=kind
export KIND_CLUSTER_PREFIX="mgc-"

source /dev/stdin <<< "$(curl -s https://raw.githubusercontent.com/Kuadrant/multicluster-gateway-controller/main/hack/.cleanupUtils)"

cleanup