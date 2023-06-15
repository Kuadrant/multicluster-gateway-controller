# Kuadrant operator addon

## Introduction
The following walkthrough will show you how to install/setup the Kuadrant operator via OCM (Open cluster management addons)

**_NOTE:_** :exclamation: A good walkthrough to have done before this is [Open Cluster Management and Multi-Cluster gateways](ocm-control-plane-walkthrough.md)


## Prerequisites
* Kind 

## Open terminal sessions
For this walkthrough, we're going to use multiple terminal sessions/windows, all using `multicluster-gateway-controller` as the `pwd`.

Open 2 windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub/control plane cluster, Where we'll run our controller locally)
* `T2` (Hub/control plane cluster)
* `T3` (Spoke/workload cluster 1)

## Setup up local environment
1. Clone this repo locally
1. In `T1` run the following command to bring up the kind clusters. The number of spoke cluster you want is dictated by the env var `MGC_WORKLOAD_CLUSTERS_COUNT`

    ```bash
    make local-setup MGC_WORKLOAD_CLUSTERS_COUNT=1
    ```
    > :sos: Linux users may encounter the following error:
    > `ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
    > make: *** [Makefile:75: local-setup] Error 1ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
    > make: *** [Makefile:75: local-setup] Error 1` 
    > This is a known issue with Kind. [Follow the steps here](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files) to resolve it.

### Running the addon manager controller and deploying Kuadrant resources


> **_NOTE:_** :exclamation: Your terminal should have the context of the Hub cluster or the control plane cluster. This is by default the context after you run the make local setup. To get the context run the following command
     `kind export kubeconfig --name=mgc-control-plane --kubeconfig=$(pwd)/local/kube/control-plane.yaml && export KUBECONFIG=$(pwd)/local/kube/control-plane.yaml`

1. In `T1` run the following to bring up the controller.
    ```bash
    make run-ocm
    ```
1. Update the managed cluster addon `namespace` to the spoke cluster name you want to deploy Kuadrant to e.g `kind-mgc-workload-1`. Then in `T2` deploy it to the hub cluster
    ```bash
    kubectl apply -f config/kuadrant/deploy/hub
    ```  
1. In the `T3` change the context to the workload cluster via 
    ```bash
    kind export kubeconfig --name=mgc-workload-1 --kubeconfig=$(pwd)/local/kube/workload1.yaml && export KUBECONFIG=$(pwd)/local/kube/workload1.yaml`
    ```    
1. In `T3` Running the following:
    ```bash
    kubectl get pods -n kuadrant-system
    ```
    you should see the namespace `kuadrant-system` be created and the following pods come up:
* authorino-*value*
* authorino-operator-*value*
* kuadrant-operator-controller-manager-*value*
* limitador-*value*
* limitador-operator-controller-manager-*value*

# Follow on Walkthroughs
Some good follow on walkthroughs that build on this walkthrough

* [Deploying/Configuring Redis, Limitador and Rate limit policies.](https://github.com/Kuadrant/multicluster-gateway-controller/blob/main/docs/how-to/ratelimiting-shared-redis.md)






