# multicluster-gateway-controller

## Description:

The multi-cluster gateway controller, leverages the gateway API standard and Open Cluster Management to provide multi-cluster connectivity and global load balancing

Key Features:

- Central Gateway Definition that can then be distributed to multiple clusters
- Automatic TLS and cert distribution for HTTPS based listeners
- DNSPolicy to decide how North-South based traffic should be balanced and reach the gateways
- Health checks to detect and take remedial action against unhealthy endpoints
- Cloud DNS provider integrations (AWS route 53) with new ones being added (google DNS)


When deploying the multicluster gateway controller using the make targets, the following will be created: 
* Kind cluster(s)
* Gateway API CRDs in the control plane cluster
* Ingress controller
* Cert manager
* ArgoCD instance
* K8s Dashboard
* LetsEncrypt certs
	


## Prerequisites:
* AWS or GCP
* Various dependencies installed into $(pwd)/bin e.g. kind, yq etc.
  * Run `make dependencies`
* openssl>=3
    * On macOS a later version is available with `brew install openssl`. You'll need to update your PATH as macOS provides an older version via libressl as well
    * On Fedora use `dnf install openssl`
* go >= 1.21

### 1. Running the controller in the cluster:
1. Set up your DNS Provider by following these [steps](https://github.com/Kuadrant/dns-operator/blob/main/docs/provider.md)

1. Setup your local environment 
    ```sh
    make local-setup MGC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```  
1. Build the controller image, load it into the control plane and deploy
    ```sh
   kubectl config use-context kind-mgc-control-plane 
   make local-deploy
    ```

1. (Optional) View the logs of the deployed controller
    ```sh
    kubectl logs -f deployment/mgc-controller-manager -n multicluster-gateway-controller-system
    ```

## 2. Running the controller locally:
1. Set up your DNS Provider by following these [steps](https://github.com/Kuadrant/dns-operator/blob/main/docs/provider.md)

1.  Setup your local environment 

    ```sh
    make local-setup MGC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER>
    ```

1. Run the controller locally:
    ```sh
    kubectl config use-context kind-mgc-control-plane 
    make build run
    ```

## 5. Clean up local environment
In any terminal window target control plane cluster by:
```bash
kubectl config use-context kind-mgc-control-plane 
```
If you want to wipe everything clean consider using:
```bash
make local-cleanup # Remove kind clusters created locally and cleanup any generated local files.
```
If the intention is to cleanup kind cluster and prepare them for re-installation consider using:
```bash
make local-cleanup-mgc MGC_WORKLOAD_CLUSTERS_COUNT=<NUMBER_WORKLOAD_CLUSTER> # prepares clusters for make local-setup-mgc
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

