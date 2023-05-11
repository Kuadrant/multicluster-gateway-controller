# multicluster-gateway-controller

## Description:
When deploying the multicluster gateway controller using the make targets, the following will be created: 
* Kind cluster(s)
* Gateway API CRDs in the control plane cluster
* Ingress controller
* Cert manager
* ArgoCD instance
* K8s Dashboard
* LetsEncrypt certs
	


## Prerequisites:
* AWS
* Kind 
    * `make kind`
* yq 
    * `make yq`
* openssl>=3
    * On macos a later version is available with `brew install openssl`. You'll need to update your PATH as macos provides an older version via libressl as well
    * On fedora use `dnf install openssl`
* go >= 1.20

### 1. Running the controller in the cluster:
1. Create env files:
    * One called `aws-credentials.env` containing **AWS_ACCESS_KEY_ID**, **AWS_SECRET_ACCESS_KEY** and **AWS_REGION**
    * One called `controller-config.env` containing **AWS_DNS_PUBLIC_ZONE_ID** and **ZONE_ROOT_DOMAIN**

1. Setup your local environment 
    ```sh
    make local-setup MCTC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```  
1. Build the controller image and load it into the control plane
    ```sh
   kubectl config use-context kind-mctc-control-plane 
   make kind-load-controller
    ```

1. Deploy the controller to the control plane cluster
    ```sh
    make deploy-controller
    ```

1. (Optional) View the logs of the deployed controller
    ```sh
    kubectl logs -f $(kubectl get pods -n multi-cluster-gateways | grep "mctc-" | awk '{print $1}') -n multi-cluster-gateways
    ```

## 2. Running the controller locally:
1. Create env files:
    * One called `aws-credentials.env` containing **AWS_ACCESS_KEY_ID**, **AWS_SECRET_ACCESS_KEY** and **AWS_REGION**
    * One called `controller-config.env` containing **AWS_DNS_PUBLIC_ZONE_ID** and **ZONE_ROOT_DOMAIN**

1.  Setup your local environment 

    ```sh
    make local_setup MCTC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```

1. Run the controller locally:
    ```sh
    kubectl config use-context kind-mctc-control-plane 
    (export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build-controller install run-controller)
    ```

## 3. Running the agent in the cluster:
1. Build the agent image and load it into the workload cluster
    ```sh
    kubectl config use-context kind-mctc-workload-1 
    make kind-load-agent
    ```

1. Deploy the agent to the workload cluster
    ```sh
    make deploy-agent
    ```
    
## 4. Running the agent locally
1. Target the workload cluster you wish to run on:
```sh
export KUBECONFIG=./tmp/kubeconfigs/mctc-workload-1.kubeconfig
```
1. Run the agent locally:
```sh
make build-agent run-agent
```

## License

Copyright 2022 Red Hat.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

