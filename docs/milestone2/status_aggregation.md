# Proposal: Aggregation of Status Conditions

## Background

[Status conditions](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties) are used to represent the current state of a resource and provide information about any problems or issues that might be affecting it. They are defined as an array of [Condition](https://pkg.go.dev/k8s.io/apimachinery@v0.26.3/pkg/apis/meta/v1#Condition) objects within the status section of a resource's YAML definition.

## Problem Statement

When multiple instances of a resource (e.g. a Gateway) are running across multiple clusters, it can be difficult to know the current state of each instance without checking each one individually. This can be time-consuming and error-prone, especially when there are a large number of clusters or resources.

## Proposal

To solve this problem, I'm proposing we leverage the status conditions in the control plane instance of that resource, aggregating the statuses to convey the necessary information.

For example, if the `ListenersValid` status condition type of a `Gateway` is `True` for all instances of the `Gateway` resource across all clusters, then the `Gateway` in the control plane will have the `ListenersValid` status condition type also set to `True`.

```yaml
status:
  conditions:
  - type: ListenersValid
    status: True
    message: All listeners are valid
```

If the `ListenersValid` status condition type of some instances is *not* `True`, the `ListenersValid` status condition type of the `Gateway` in the control plane will be `False`.

```yaml
status:
  conditions:
  - type: ListenersValid
    status: False
```

In addition, if the `ListenersValid` status condition type is `False`, the `Gateway` in the control plane should include a status message for each `Gateway` instance where `ListenersValid` is `False`. This message would indicate the reason why the condition is not true for each `Gateway`.

```yaml
status:
  conditions:
  - type: ListenersValid
    status: False
    message: "gateway-1 Listener certificate is expired; gateway-3 No listener configured for port 80"
```

In this example, the `ListenersValid` status condition type is `False` because two of the three Gateway instances (gateway-1 and gateway-3) have issues with their listeners. For gateway-1, the reason for the `False` condition is that the listener certificate is expired, and for gateway-3, the reason is that no listener is configured for port 80. These reasons are included as status messages in the `Gateway` resource in the control plane.

As there may be different reasons for the condition being `False` across different clusters, it doesn't make sense to aggregate the `reason` field. The `reason` field is intended to be a programatic identifier, while the `message` field allows for a human readable message i.e. a semi-colon separated list of messages.

The `lastTransitionTime` and `observedGeneration` fields will behave as normal for the resource in the control plane.