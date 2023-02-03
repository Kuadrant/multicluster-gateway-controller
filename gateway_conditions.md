# GatewayClass & Gateway Status Conditions

## GatewayClass conditions

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

If the Gateway is not valid or an unsupported class is specified, the condition changes to:

```yaml
    - lastTransitionTime: "1970-01-01T00:00:00Z"
      status: "False"
      type: Accepted
      reason: InvalidParameters
      message: Invalid Parameters - <specific error message>
      observedGeneration: <corresponding metadata.generation>
```

### Full list of possible GatewayClass conditions

See https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io%2fv1beta1.GatewayClassConditionType

## Gateway conditions

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
If the Gateway is syntactically and semantically valid, the condition changes to:

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

### Full list of possible Gateway conditions

See https://gateway-api.sigs.k8s.io/references/spec/#gateway.networking.k8s.io%2fv1beta1.GatewayConditionType