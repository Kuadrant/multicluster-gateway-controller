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

set -e pipefail

LOCAL_SETUP_DIR="$(dirname "${BASH_SOURCE[0]}")"
source "${LOCAL_SETUP_DIR}"/../.binEnv
source "${LOCAL_SETUP_DIR}"/../.setupEnv
source "${LOCAL_SETUP_DIR}"/../.deployUtils

export TMP_DIR=./tmp

# Initialise Skupper
echo "Initialising Skupper"
kubectl config use-context kind-mgc-control-plane
kubectl config set-context --current --namespace default
skupper init --enable-console --enable-flow-collector

kubectl config use-context kind-mgc-workload-1
kubectl config set-context --current --namespace default
skupper init

# Link Sites
echo "Linking Skupper sites"
kubectl config use-context kind-mgc-control-plane
skupper token create ~/skupper-mgc-control-plane.token

kubectl config use-context kind-mgc-workload-1
skupper link create ~/skupper-mgc-control-plane.token

# Output status
echo "Skupper setup finished"
kubectl config use-context kind-mgc-control-plane
skupper status

kubectl config use-context kind-mgc-workload-1
skupper status

# Switch back to control plane context
kubectl config use-context kind-mgc-control-plane
