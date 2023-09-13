# Installing Kuadrant Service Protection into an existing OCM Managed Cluster

## Introduction
This walkthrough will show you how to install and setup the Kuadrant Operator into an [OCM](https://open-cluster-management.io/) [Managed Cluster](https://open-cluster-management.io/concepts/managedcluster/).

## Prerequisites
* Access to an Open Cluster Management (>= v0.11.0) Managed Cluster, which has already been bootstrapped and registered with a hub cluster
  * We have [a guide](./control-plane-installation.md) which covers this in detail
  * Also see:
    * https://open-cluster-management.io/getting-started/quick-start/
    * https://open-cluster-management.io/concepts/managedcluster/
* OLM will need to be installed into the ManagedCluster where you want to run the Kuadrant Service Protection components
  * See https://olm.operatorframework.io/docs/getting-started/
* Kuadrant uses Istio as a Gateway API provider - this will need to be installed into the data plane clusters
  * We recommend installing Istio 1.17.0, including Gateway API v0.6.2
  * ```bash
    kubectl apply -k "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v0.6.2"
    ```
  * See also: https://istio.io/v1.17/blog/2022/getting-started-gtwapi/


Alternatively, if you'd like to quickly get started locally, without having to worry to much about the pre-requisites, take a look our [Quickstart Guide](https://docs.kuadrant.io/getting-started/). It will get you setup with Kind, OLM, OCM & Kuadrant in a few short steps.


## Install the Kuadrant OCM Add-On


**Note:** if you've run our [Getting Started Guide](https://docs.kuadrant.io/getting-started/), you'll be set to run this command as-is.

To install the Kuadrant Service Protection components into a `ManagedCluster`, target your OCM hub cluster with `kubectl` and run:

`kubectl apply -k "github.com/kuadrant/multicluster-gateway-controller.git/config/service-protection-install-guide?ref=main" -n <your-managed-cluster>`

The above command will install the `ManagedClusterAddOn` resource needed to install the Kuadrant addon into the specified namespace, and install the Kuadrant data-plane components into the `open-cluster-management-agent-addon` namespace. 

The Kuadrant addon will install:

* the Kuadrant Operator
* Limitador (and its associated operator)
* Authorino  (and its associated operator)

For more details, see the Kuadrant components installed by the (kuadrant-operator)[https://github.com/Kuadrant/kuadrant-operator#kuadrant-components]

### Existing Istio installations and changing the default Istio Operator name
In the case where you have an existing Istio installation on a cluster, you may encounter an issue where the Kuadrant Operator expects Istio's Operator to be named `istiocontrolplane`.

The `istioctl` command saves the IstioOperator CR that was used to install Istio in a copy of the CR named `installed-state`.

To let the Kuadrant operator use this existing installation, set the following:

`kubectl annotate managedclusteraddon kuadrant-addon "addon.open-cluster-management.io/values"='{"IstioOperator":"installed-state"}' -n <managed spoke cluster>`

This will propogate down and update the Kuadrant Operator, used by the Kuadrant OCM Addon.

## Verify the Kuadrant addon installation

To verify the Kuadrant OCM addon has installed currently, run:

```bash
kubectl wait --timeout=5m -n kuadrant-system kuadrant/kuadrant-sample --for=condition=Ready
```

You should see the namespace `kuadrant-system`, and the following pods come up:
* authorino-*value*
* authorino-operator-*value*
* kuadrant-operator-controller-manager-*value*
* limitador-*value*
* limitador-operator-controller-manager-*value*

# Further Reading
With the Kuadrant data plane components installed, here is some further reading material to help you utilise Authorino and Limitador:

[Getting started with Authorino](https://docs.kuadrant.io/authorino/)
[Getting started With Limitador](https://docs.kuadrant.io/limitador-operator/)





