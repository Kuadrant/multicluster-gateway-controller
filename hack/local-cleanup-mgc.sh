LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/.binEnv
source "${LOCAL_SETUP_DIR}"/.setupEnv
source "${LOCAL_SETUP_DIR}"/.cleanupUtils

set -o pipefail

echo "Ensure controller is running if you want DNS record to be properly cleaned up. Press [ENTER] to continue"
read

cleanupMGC