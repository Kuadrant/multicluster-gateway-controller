## Define and Place Gateways

In this guide, we will go through defining a Gateway in the OCM hub cluster that can then be distributed to and instantiated on a set of managed spoke clusters.

### Prerequisites
* Complete the [Getting Started Guide](https://docs.kuadrant.io/getting-started/) to bring up a suitable environment. 

If you are looking to change provider from the default Istio:
* Please have the Gateway provider of your choice installed and configured (in this example we use Envoy gateway. See the following [docs](https://gateway.envoyproxy.io/v0.5.0/user/quickstart.html))

## Initial setup 

export `MGC_SUB_DOMAIN` in each terminal if you haven't already added it to your `.zshrc` or `.bash_profile`.

Going through the quick start above, will ensure that a supported `GatewayClass` is registered in the hub cluster that the Kuadrant multi-cluster gateway controller will handle. 

**NOTE** :exclamation: The quick start script will create a placement resource as part of the setup. You can use this as further inspiration for other placement resources you would like to create.


### Defining a Gateway

Once you have the Kuadrant multi-cluster gateway controller installed into the OCM hub cluster, you can begin defining and placing Gateways across your OCM managed infrastructure.

To define a Gateway and have it managed by the multi-cluster gateway controller, we need to do the following things

- Create a Gateway API Gateway resource in the Hub cluster, ensuring the gateway resource specifies the correct gateway class allowing it to be picked up and managed by the multi-cluster gateway controller

```bash
kubectl --context kind-mgc-control-plane apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: prod-web
  namespace: multi-cluster-gateways
spec:
  gatewayClassName: kuadrant-multi-cluster-gateway-instance-per-cluster
  listeners:
  - allowedRoutes:
      namespaces:
        from: All
    name: api
    hostname: $MGC_SUB_DOMAIN
    port: 443
    protocol: HTTP
EOF
```

### Placing a Gateway

 To place a gateway, we will need to create a Placement resource. 
```bash
kubectl --context kind-mgc-control-plane apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: http-gateway-placement
  namespace: multi-cluster-gateways
spec:
  clusterSets:
  - gateway-clusters # defines which ManagedClusterSet to use.  
  numberOfClusters: 2 # defines how many clusters to select from the chosen clusterSets
EOF
```
For more information on ManagedClusterSets and placements please see the OCM official docs:

* [ManagedClusterSets](https://open-cluster-management.io/concepts/managedclusterset/)

* [Placements](https://open-cluster-management.io/concepts/placement/)



Finally in order to have the Gateway instances deployed to your spoke clusters that can start receiving traffic, you need to 
* Add the second cluster to the clusterset
* Label the hub gateway with a placement label.

1. Add the second cluster to the clusterset, by running the following:

    ```bash
    kubectl --context kind-mgc-control-plane label managedcluster kind-mgc-workload-1 ingress-cluster=true
    ```
1. To place the gateway, we need to add a placement label to gateway resource to instruct the gateway controller where we want this gateway instantiated.

    ```bash
    kubectl --context kind-mgc-control-plane label gateway prod-web "cluster.open-cluster-management.io/placement"="http-gateway-placement" -n multi-cluster-gateways
    ```

2. To find a configured gateway and instantiated gateway on the hub cluster. Run the following  

    ```bash
    kubectl --context kind-mgc-control-plane get gateway -A
    ```

    You'll see the following:

    ```
    kuadrant-multi-cluster-gateways   prod-web   istio                                         172.31.200.0                29s
    multi-cluster-gateways            prod-web   kuadrant-multi-cluster-gateway-instance-per-cluster                  True         2m42s
    ```
3.  Execute the following to see the gateway on the workload-1 cluster:

    ```bash
    kubectl --context kind-mgc-workload-1 get gateways -A
    ```
    You'll see the following
    ```
    NAMESPACE                         NAME       CLASS   ADDRESS        PROGRAMMED   AGE
    kuadrant-multi-cluster-gateways   prod-web   istio   172.31.201.0                90s
    ```
### Using a different gateway provider?

While we recommend using Istio as the gateway provider as that is how you will get access to the full suite of policy APIs, it is possible to use another provider if you choose to however this will result in a reduced set of applicable policy objects.

If you are only using the DNSPolicy and TLSPolicy resources, you can use these APIs with any Gateway provider. To change the underlying provider, you need to set the gatewayclass param `downstreamClass`. 

1.  Create the following configmap. Note: In this example, 'eg' stands for the Envoy gateway, which is mentioned in the prerequisites above:

    ```bash
    kubectl --context kind-mgc-control-plane apply -f - <<EOF
    apiVersion: v1
    data:
      params: |
        {
          "downstreamClass": "eg"
        }
    kind: ConfigMap
    metadata:
      name: gateway-params
      namespace: multi-cluster-gateways
    EOF
    ```
2. Update the gatewayclass to include the above Configmap

    ```bash
    kubectl --context kind-mgc-control-plane patch gatewayclass kuadrant-multi-cluster-gateway-instance-per-cluster -n multi-cluster-gateways --type merge --patch '{"spec":{"parametersRef":{"group":"","kind":"ConfigMap","name":"gateway-params","namespace":"multi-cluster-gateways"}}}'
    ```

Once this has been created, any gateways created from that gateway class will result in a downstream gateway being provisioned with the configured downstreamClass.
Run the following in both your hub  and spoke cluster to see the gateways:

  ```bash
  kubectl --context kind-mgc-control-plane get gateway -A
  ```
  ```bash
  kubectl --context kind-mgc-workload-1 get gateway -A
  ```