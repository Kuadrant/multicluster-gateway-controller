# multi-cluster-traffic-controller

## Description:
When deploying the multi cluster traffic controller using the make targets the following will be created: 
* Kind cluster(s)
* Gateway API CRDs in the control plane cluster
* Ingress controller
* Cert manager
* External DNS
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

### 1. Running the operator in the cluster:


1. Setup your local environment 
    ```sh
    make local_setup MCTC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```

1. Update the manager to contain the necessary environment variable and update the image policy:
    ```
    containers:
        - command:
            - /manager
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

3. Build the image
    ```sh
    make docker-build
    ```
4. Load the image it into the control plane cluster

    ```sh
    kind load docker-image controller:latest --name mctc-control-plane  --nodes mctc-control-plane-control-plane
    ```

5. Deploy the controller to the control plane cluster
    ```sh
    make deploy
    ```

## 2. Running locally:
1. Create two env files:
    * One called `aws-credentials.env` containing **AWS_ACCESS_KEY_ID** and **AWS_SECRET_ACCESS_KEY**
    * One called `controller-config` containing **AWS_DNS_PUBLIC_ZONE_ID** and **ZONE_ROOT_DOMAIN**


2.  Setup your local environment 

    ```sh
    make local_setup MCTC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```

3. Deploy the controller to the control plane cluster
    ```sh
    make deploy
    ```
4. Deploy the operator
    ```sh
    (export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build install run
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

