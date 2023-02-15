# multi-cluster-traffic-controller

## Description:
When deploying the multi cluster traffic controller using the make targets the following will be created: 
* Kind cluster(s)
* Gateway API CRDs in the control plane cluster
* Ingress controller
* Cert manager
* Webhook and webhook config
* ARGO CD instance
* K8s Dashboard
* Lets encrypt certs
	


## Prerequisites:
* AWS
* Kind 
    * `make kind`
* yq 
    * `make yq`
* openssl>=3
    * On macos a later version is available with `brew install openssl`. You'll need to update your PATH as macos provides an older version via libressl as well
    * On fedora use `dnf install openssl`

### 1. Running the operator in the cluster:


1. Setup your local environment 
    ```sh
    make local-setup MCTC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```

1. Update the manager to contain the necessary environment variable and update the image policy (N.B. Do not copy/paste this, the hyphens from github are bad for the YAML parser):
    ```
    containers:
        - command:
            - /controller
            args:
            - --leader-elect
            image: controller:latest
            imagePullPolicy: Never
            env:
            - name: AWS_ACCESS_KEY_ID
            value: <AWS_ACCESS_KEY_ID>
            - name: AWS_SECRET_ACCESS_KEY
            value: <AWS_SECRET_ACCESS_KEY>
            - name: AWS_DNS_PUBLIC_ZONE_ID
            value: <AWS_DNS_PUBLIC_ZONE_ID>
            - name: ZONE_ROOT_DOMAIN
            value: <ZONE_ROOT_DOMAIN>
    ```

1. Build the controller image and load it into the control plane
    ```sh
    export KUBECONFIG=./hack/kubeconfigs/mctc-control-plane.kubeconfig
    make kind-load-controller
    ```

1. Deploy the controller to the control plane cluster
    ```sh
    make deploy-controller
    ```

1. (Optional) View the logs of the deployed controller
    ```sh
    kubectl logs -f $(kubectl get pods -n multi-cluster-traffic-controller-system | grep "mctc-" | awk '{print $1}') -n multi-cluster-traffic-controller-system
    ```

## 2. Running locally:
1. Create two env files:
    * One called `aws-credentials.env` containing **AWS_ACCESS_KEY_ID** and **AWS_SECRET_ACCESS_KEY**
    * One called `controller-config` containing **AWS_DNS_PUBLIC_ZONE_ID** and **ZONE_ROOT_DOMAIN**


1.  Setup your local environment 

    ```sh
    make local_setup MCTC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```

1. Deploy the operator
    ```sh
    (export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build install run
    ```

## 3. Deploy the agent
1. Update the secret in `config/agent/secret.yaml` with the correct credentials for the control plane. (**N.B.** The server should remain `https://mctc-control-plane-control-plane:6443`)

1. Build the agent image and load it into the workload cluster
    ```sh
    export KUBECONFIG=./hack/kubeconfigs/mctc-workload-1.kubeconfig
    make kind-load-agent
    ```

1. Deploy the agent to the workload cluster
    ```sh
    make deploy-agent
    ```
    
## 4. Run the ingress agent locally
1. Target the workload cluster you wish to run on:
```sh
export KUBECONFIG=./hack/kubeconfigs/mctc-workload-1.kubeconfig
```
1. Create the control plane secret
Update the `samples/syncer/secret.yaml` from the `hack/kubeconfigs/mctc-control-plane.kubeconfig` and apply it:
```sh
kubectl create -f samples/syncer/secret.yaml
```
1. Run the agent locally:
```sh
make run-agent
```

## License

Copyright 2022 The MultiCluster Traffic Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

