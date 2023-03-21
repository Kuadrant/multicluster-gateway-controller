# GatewayClass & Gateway Status

## GatewayClass `status.conditions`

There is 1 condition type for GatewayClasses -  `Accepted`.

### 'Accepted' GatewayClass condition type

When a GatewayClass is created, it starts as below. This is the default defined in the CRD:

```yaml
    - lastTransitionTime: "1970-01-01T00:00:00Z"
      status: Unknown
      type: Accepted
      reason: Waiting
      message: Waiting for controller
```

When reconciled, the condition may change to 1 of 2 states.
If the GatewayClass is valid *and* it is a supported class for which mctc will provision Gateways, the condition changes to:

```yaml
    - lastTransitionTime: "1970-01-01T00:00:00Z"
      status: "True"
      type: Accepted
      reason: Accepted
      message: Handled by kuadrant.io/mctc-gw-controller
      observedGeneration: <corresponding metadata.generation>
```

If the GatewayClass is not valid or an unsupported class is specified, the condition changes to:

```yaml
    - lastTransitionTime: "1970-01-01T00:00:00Z"
      status: "False"
      type: Accepted
      reason: InvalidParameters
      message: Invalid Parameters - <specific error message>
      observedGeneration: <corresponding metadata.generation>
```

## Gateway `status.conditions`

For simplicity of implementing an abstraction on multiple Gateway resources in multiple clusters, only the `Accepted` and `Programmed` condition types will be used.
The `Ready` condition type, which is optional, will *not* be used.
This may change in future based on requirements.

### 'Accepted' Gateway condition type

When a Gateway is created, it starts as below. This is the default defined in the CRD:

```yaml
    - lastTransitionTime: "2019-10-22T16:29:24Z"
      status: "Unknown"
      type: Accepted
      reason: Pending
      message: "Waiting for controller"
```

When reconciled, the condition may change to 1 of 2 states.
If the Gateway is syntactically and semantically valid, and references a supported gatewayClassName, the condition changes to:

```yaml
    - lastTransitionTime: "2023-02-03T13:45:58Z"
      status: "True"
      type: Accepted
      reason: Accepted
      message: Handled by kuadrant.io/mctc-gw-controller
      observedGeneration: <corresponding metadata.generation>
```

If the Gateway is not valid, the condition changes to:

```yaml
    - lastTransitionTime: "2023-02-03T13:45:58Z"
      status: "Unknown"
      type: Accepted
      reason: Pending
      message: Invalid gateway configuration - <specific error message>
      observedGeneration: <corresponding metadata.generation>
```

### 'Programmed' Gateway condition type

When a Gateway is created, it starts as below. This is the default defined in the CRD:

```yaml
    - lastTransitionTime: "2019-10-22T16:29:24Z"
      status: "Unknown"
      type: Programmed
      reason: Pending
      message: "Waiting for controller"
```

The condition will transition to the below when the Gateway configuration has been reconciled into the data plane Gateway resources:

```yaml
    - lastTransitionTime: "2019-10-22T16:29:31Z"
      status: "True"
      type: Programmed
      reason: Programmed
      message: "Gateways configured in data plane clusters - [<comma separated list of cluster names>]"
      observedGeneration: <corresponding metadata.generation>
```

If there are no data plane clusters (i.e. none configured yet or none selected) the condition state will look like this:

```yaml
    - lastTransitionTime: "2019-10-22T16:29:31Z"
      status: "False"
      type: Programmed
      reason: Pending
      message: "No clusters match selection"
      observedGeneration: <corresponding metadata.generation>
```

If there is a problem with configuring the data plane Gateway resources, the condition state will look like this:

```yaml
    - lastTransitionTime: "2019-10-22T16:29:31Z"
      status: "False"
      type: Programmed
      reason: Pending
      message: "Problem configuring data plane - <specific error message>"
      observedGeneration: <corresponding metadata.generation>
```

## Gateway `status.addresses`

The `addresses` field behaves similar to the same field in the [Gateway API Specification](https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io/v1beta1.GatewayStatus).
It lists the IP addresses that have actually been bound to *all* Gateway instances across *all* data plane clusters the Gateway has been instansiated on.
For example, if there are 2 Gateway instances in the data plane with `status.addresses` values of below:

```yaml
# Data Plane Gateway 1
status:
  addresses:
  - type: IPAddress
    value: 172.16.0.1

# Data Plane Gateway 2
status:
  addresses:
  - type: IPAddress
    value: 172.16.0.2
```

The `status.addresses` field in the control plane Gateway will look something like this:

```yaml
status:
  addresses:
  - type: IPAddress
    value: 172.16.0.1
  - type: IPAddress
    value: 172.16.0.2
```

## Individual Data Plane Gateway statuses

The full status of each Gateway in a data plane cluster may have some value beyond what's included in the rolled up control plane Gateway status.
For this reason, the individual Gateway statuses can be retrieved from annotations as json.
For example, a Gateway in a data plane cluster with an id of `eu-apps-1` would have the full status available like this in the control plane Gateway:

```yaml
metadata:
  annotations:
    mctc-status-syncer-status-eu-apps-1: {"addresses":[{"type":"IPAddress","value":"172.16.0.1"}],"conditions":[{{"lastTransitionTime":"2023-02-16T11:47:09Z","message":"Gateway valid, assigned to service(s) istio.istio-system.svc.cluster.local:80","observedGeneration":4,"reason":"ListenersValid","status":"True","type":"Ready"}],"listeners":[{"attachedRoutes":1,"conditions":[{"lastTransitionTime":"2023-02-16T11:47:09Z","message":"No errors found","observedGeneration":4,"reason":"Ready","status":"True","type":"Ready"}],"name":"test","supportedKinds":[{"group":"gateway.networking.k8s.io","kind":"HTTPRoute"}]}]}
```

### WIP Multicluster-gateway-controller specific condition types

WIP

The individual status of each data plane Gateway may also include status conditions that are specific to multicluster-gateway-controller functionality.
For example, take an Istio Gateway with the below status conditions:

```yaml
  - lastTransitionTime: "2023-02-16T11:53:13Z"
    message: Deployed gateway to the cluster
    observedGeneration: 4
    reason: ResourcesAvailable
    status: "True"
    type: Scheduled
  - lastTransitionTime: "2023-02-16T11:47:09Z"
    message: Gateway valid, assigned to service(s) istio.istio-system.svc.cluster.local:80
    observedGeneration: 4
    reason: ListenersValid
    status: "True"
    type: Ready
```

If this Gateway has a multicluster-gateway-controller 'Policy' attached to it at the control plane layer, additional status like below will be added to the Gateway in the data plane.

```
  - lastTransitionTime: "2023-02-16T11:47:09Z"
    message: Gateway valid, assigned to service(s) istio.istio-system.svc.cluster.local:80
    observedGeneration: 4
    reason: ListenersValid
    status: "True"
    type: Ready
```