## Define and Place Gateways

In this guide, we will go through defining a Gateway in the OCM hub cluster that can then be distributed to and instantiated on a set of managed spoke clusters.

### Pre Requisites

- Go through the [getting started guide](https://docs.kuadrant.io/getting-started/). 

You should start this guide with OCM installed, 1 or more spoke clusters registered with the hub and Kuadrant installed into the hub.

Going through the installation will also ensure that a supported `GatewayClass` is registered in the hub cluster that the Kuadrant multi-cluster gateway controller will handle. 


### Defining a Gateway

Once you have Kudarant installed in to the OCM hub cluster, you can begin defining and placing Gateways across your OCM managed infrastructure.

To define a Gateway and have it managed by the multi-cluster gateway controller, we need to do the following things

- Create a Gateway API Gateway resource in the Hub cluster
- Ensure that gateway resource specifies the correct gateway class so that it will be picked up and managed by the multi-cluster gateway controller

So really there is very little different from setting up a gateway in a none OCM hub. The key difference here is this gateway definition, represents a "template" gateway that will then be distributed and provisioned on chosen spoke clusters. The actual provider for this Gateway instance defaults to Istio. This is because kuadrant also offers APIs that integrate at the gateway provider level and the gateway provider we currently support is Istio.

The Gateway API CRDS will have been installed into your hub as part of installation of Kuadrant into the hub. Below is an example gateway. [More Examples](https://github.com/kubernetes-sigs/gateway-api/tree/main/examples/standard). Assuming you have the correct RBAC permissions and a namespace, the key thing is to define the correct `GatewayClass` name to use and a listener host.

```
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: prod-web
  namespace: multi-cluster-gateways
spec:
  gatewayClassName: kuadrant-multi-cluster-gateway-instance-per-cluster #this needs to be set in your gateway definiton
  listeners:
  - allowedRoutes:
      namespaces:
        from: All
    name: specific
    hostname: 'some.domain.example.com'
    port: 443
    protocol: HTTP

```

### Placing a Gateway

To place a gateway, we will need to create a Placement resource. 

Below is an example placement resource. To learn more about placement check out the OCM docs [placement](https://open-cluster-management.io/concepts/placement/)


```
 apiVersion: cluster.open-cluster-management.io/v1beta1
  kind: Placement
  metadata:
    name: http-gateway-placement
    namespace: multi-cluster-gateways
  spec:
    clusterSets:
    - gateway-clusters # defines which ManagedClusterSet to use. https://open-cluster-management.io/concepts/managedclusterset/ 
    numberOfClusters: 2 #defines how many clusters to select from the chosen clusterSets

```


Finally in order to actually have the Gateway instances deployed to your spoke clusters that can start receiving traffic, you need to label the hub gateway with a placement label. In the above example we would add the following label to the gateway.


```
cluster.open-cluster-management.io/placement: http-gateway #this value should match the name of your placement.
```

### What if you want to use a different gateway provider?

While we recommend using Istio as the gateway provider as that is how you will get access to the full suite of policy APIs, it is possible to use another provider if you choose to however this will result in a reduced set of applicable policy objects.

If you are only using the DNSPolicy and TLSPolicy resources, you can use these APIs with any Gateway provider. To change the underlying provider, you need to set the gatewayclass param `downstreamClass`. To do this create the following configmap:

``` 
apiVersion: v1
data:
  params: |
    {
      "downstreamClass": "eg" #this is the class for envoy gateway used as an example
    }
kind: ConfigMap
metadata:
  name: gateway-params
  namespace: multi-cluster-gateways
```

Once this has been created, any gateway created from that gateway class will result in a downstream gateway being provisioned with the configured downstreamClass.