# Placements

The Placement API allows a resource to be synced to 1 or more data plane clusters.
Here is an example Placement CR:

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: Placement
metadata:
  name: example-placement
spec:
  targetRef:
    group: gateway.networking.k8s.io
    version: v1beta1
    resource: gateways
    name: example-gateway
  predicates:
  - requiredClusterSelector:
      labelSelector:
        matchLabels:
          region: us-east-1
```

The Placment spec is telling the placement controller 2 things.

First, the resource we want to be placed in 1 or more clusters.
In this case, it's a `Gateway` resource called `example-gateway`.
The namespace is assumed to be the same as the Placement namespace.

Second, what clusters we want to place the resource on.
In this case, the `Gateway` will be placed on all clusters that have a label of `region=us-east-1`.
A cluster is represented by a `Secret`. So any cluster secrets with that label will be matched.

The Placement controller logic will maintain any syncer annotations on the targetRef resource to ensure it's synced to the right clusters.
Again, in this example case, the Gateway will have the below annotations added if 2 cluster match that label selector:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  annotations:
    mctc-sync-agent/cluster1: "true"
    mctc-sync-agent/cluster2: "true"
  name: example-gateway
```

If you need to make changes to a resource as its synced to the data plane, you can use these `sync` annotations to determine which additional `patch` annotations to add.
For example, if you want to change the `spec.gatewayClassName` field to a `istio` when it's synced to the data plane, you would:

* Look for any annotations starting with `mctc-sync-agent/`
* Take the cluster name from the end of those annotation names
* For each cluster name, add a new annotation with a name of `mctc-syncer-patch/<cluser_name>` e.g. `mctc-syncer-patch/cluster1`
* The value of the annotation would be an array of json patches e.g. `[{"op": "replace", "path": "/spec/gatewayClassName", "value": "istio"}]`
