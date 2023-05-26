LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/.setupEnv
source "${LOCAL_SETUP_DIR}"/.cleanupUtils

KIND_CLUSTER_PREFIX="mgc-"

set -e pipefail

cleanup
