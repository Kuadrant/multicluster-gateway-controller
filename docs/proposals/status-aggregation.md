# Proposal: Aggregation of Status Conditions

## Background

[Status conditions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) are used to represent the current state of a resource and provide information about any problems or issues that might be affecting it. They are defined as an array of [Condition](https://pkg.go.dev/k8s.io/apimachinery@v0.26.3/pkg/apis/meta/v1#Condition) objects within the status section of a resource's YAML definition.

## Problem Statement

When multiple instances of a resource (e.g. a Gateway) are running across multiple clusters, it can be difficult to know the current state of each instance without checking each one individually. This can be time-consuming and error-prone, especially when there are a large number of clusters or resources.

## Proposal

To solve this problem, I'm proposing we leverage the status block in the control plane instance of that resource, aggregating the statuses to convey the necessary information.

### Status Conditions

For example, if the `Ready` status condition type of a `Gateway` is `True` for all instances of the `Gateway` resource across all clusters, then the `Gateway` in the control plane will have the `Ready` status condition type also set to `True`.

```yaml
status:
  conditions:
  - type: Ready
    status: True
    message: All listeners are valid
```

If the `Ready` status condition type of some instances is *not* `True`, the `Ready` status condition type of the `Gateway` in the control plane will be `False`.

```yaml
status:
  conditions:
  - type: Ready
    status: False
```

In addition, if the `Ready` status condition type is `False`, the `Gateway` in the control plane should include a status message for each `Gateway` instance where `Ready` is `False`. This message would indicate the reason why the condition is not true for each `Gateway`.

```yaml
status:
  conditions:
  - type: Ready
    status: False
    message: "gateway-1 Listener certificate is expired; gateway-3 No listener configured for port 80"
```

In this example, the `Ready` status condition type is `False` because two of the three Gateway instances (gateway-1 and gateway-3) have issues with their listeners. For gateway-1, the reason for the `False` condition is that the listener certificate is expired, and for gateway-3, the reason is that no listener is configured for port 80. These reasons are included as status messages in the `Gateway` resource in the control plane.

As there may be different reasons for the condition being `False` across different clusters, it doesn't make sense to aggregate the `reason` field. The `reason` field is intended to be a programatic identifier, while the `message` field allows for a human readable message i.e. a semi-colon separated list of messages.

The `lastTransitionTime` and `observedGeneration` fields will behave as normal for the resource in the control plane.

### Addresses and Listeners status

The Gateway status can include information about addresses, like load balancer IP Addresses assigned to the Gateway,
and listeners, such as the number of attached routes for each listener.
This information is useful at the control plane level.
For example, a DNS Record should only exist as long as there is at least 1 attached route for a listener.
It can also be more complicated than that when it comes to multi cluster gateways.
A DNS Record should only include the IP Addresses of the Gateway instances where the listener has at least 1 attached route.
This is important when initial setup of DNS Records happen as applications start.
It doesn't make sense to route traffic to a Gateway where a listener isn't ready/attached yet.
It also comes into play when a Gateway is displaced either due to changing placement decision or removal.

In summary, the IP Addresses and number of attached routes per listener per Gateway instance is needed in the control plane to manage DNS effectively.
This proposal adds that information the hub Gateway status block.
This will ensure a decoupling of the DNS logic from the underlying resource/status syncing implementation (i.e. ManifestWork status feedback rules)

First, here are 2 instances of a multi cluster Gateway in 2 separate spoke clusters.
The yaml is shortened to highlight the status block.

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: gateway
status:
  addresses:
  - type: IPAddress
    value: 172.32.200.0
  - type: IPAddress
    value: 172.32.201.0
  listeners:
  - attachedRoutes: 0
    conditions:
    name: api
  - attachedRoutes: 1
    conditions:
    name: web
---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: gateway
status:
  addresses:
  - type: IPAddress
    value: 172.32.202.0
  - type: IPAddress
    value: 172.32.203.0
  listeners:
  - attachedRoutes: 1
    name: api
  - attachedRoutes: 1
    name: web
```

And here is the proposed status aggregation in the hub Gateway:

```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: gateway
status:
  addresses:
    - type: kuadrant.io/MultiClusterIPAddress
      value: cluster_1/172.32.200.0
    - type: kuadrant.io/MultiClusterIPAddress
      value: cluster_1/172.32.201.0
    - type: kuadrant.io/MultiClusterIPAddress
      value: cluster_2/172.32.202.0
    - type: kuadrant.io/MultiClusterIPAddress
      value: cluster_2/172.32.203.0
  listeners:
    - attachedRoutes: 0
      name: cluster_1.api
    - attachedRoutes: 1
      name: cluster_1.web
    - attachedRoutes: 1
      name: cluster_2.api
    - attachedRoutes: 1
      name: cluster_2.web
```

The MultiCluster Gateway Controller will use a custom implementation of the `addresses` and `listenerers` fields.
The address `type` is of type [AddressType](https://github.com/kubernetes-sigs/gateway-api/blob/f883de997b88dd6ee138930198542da8a9b2f634/apis/v1beta1/shared_types.go#L552), where the type is a domain-prefixed string identifier.
The value can be split on the forward slash, `/`, to give the cluster name and the underlying Gateway IPAddress value of type IPAddress.
Both the IPAddress and Hostname types will be supported.
The type strings for either will be `kuadrant.io/MultiClusterIPAddress` and `kuadrant.io/MultiClusterHostname`

The listener `name` is of type [SectionName](https://github.com/kubernetes-sigs/gateway-api/blob/f883de997b88dd6ee138930198542da8a9b2f634/apis/v1beta1/shared_types.go#L484), with validation on allowed characters and max length of 253.
The name can be split on the period, `.`, to give the cluster name and the underlying listener name.
As there are limits on the character length for the `name` field, this puts a lower limit restriction on the cluster names and listener names used to ensure proper operation of this status aggregation.
If the validation fails, a status condition showing a validation error should be included in the hub Gateway status block.
