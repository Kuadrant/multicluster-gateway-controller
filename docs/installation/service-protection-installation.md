# Installing Kuadrant Service Protection into an existing OCM Managed Cluster

## Introduction
This walkthrough will show you how to install and setup the Kuadrant Operator into an [OCM](https://open-cluster-management.io/) [Managed Cluster](https://open-cluster-management.io/concepts/managedcluster/).

## Prerequisites
* Access to an Open Cluster Management (>= v0.11.0) Managed Cluster, which has already been bootstrapped and registered with a hub cluster
  * We have [a guide](./control-plane-installation.md) which covers this in detail
  * Also see:
    * [https://open-cluster-management.io/getting-started/quick-start/]
    * [https://open-cluster-management.io/concepts/managedcluster/]
- [Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) (>= v1.14.0)
* OLM will need to be installed into the ManagedCluster where you want to run the Kuadrant Service Protection components
  * See: 
    * https://sdk.operatorframework.io/docs/installation/
    * https://olm.operatorframework.io/docs/getting-started/
* Kuadrant uses Istio as a Gateway API provider - this will need to be installed into the data plane clusters
  * We recommend installing Istio 1.20.0, including Gateway API v1
  * ```
    kubectl apply -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml"
    ```
  * See also: [https://preliminary.istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/]

## Install the Kuadrant OCM Add-On
To install the Kuadrant Service Protection components into a spoke `ManagedCluster`, target your OCM Hub cluster with `kubectl` and run:

```
kubectl apply -k "github.com/kuadrant/multicluster-gateway-controller.git/config/service-protection-install-guide?ref=release-0.2" -n namespace-of-your-managed-spoke-cluster-on-the-hub
```

The above command will install the `ManagedClusterAddOn` resource needed to install the Kuadrant addon into the namespace representing a spoke cluster, and install the Kuadrant data-plane components into the `open-cluster-management-agent-addon` namespace. 

The Kuadrant addon will install:

* the Kuadrant Operator
* Limitador (and its associated operator)
* Authorino  (and its associated operator)

For more details, see the Kuadrant components installed by the (kuadrant-operator)[https://github.com/Kuadrant/kuadrant-operator#kuadrant-components]

### OLM and OpenShift CatalogSource

The Kuadrant OCM (Open Cluster Management) Add-On depends on the Operator Lifecycle Manager (OLM)'s `CatalogSource`. By default, this is set to `olm/operatorhubio-catalog`.

In OpenShift environments, OLM comes pre-installed. However, it is configured to use the `openshift-marketplace/community-operators` CatalogSource by default, not the `olm/operatorhubio-catalog`.

To align the Kuadrant add-on with the OpenShift default CatalogSource, you can patch the add-on's CatalogSource configuration. Run the following command (note it needs to be run for each managed cluster where the add-on is installed): 

```bash
kubectl annotate managedclusteraddon kuadrant-addon "addon.open-cluster-management.io/values"='{"CatalogSource":"community-operators", "CatalogSourceNS":"openshift-marketplace"}' -n managed-cluster-ns
```

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





