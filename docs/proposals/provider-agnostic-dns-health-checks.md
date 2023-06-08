# Provider agnostic DNS Health checks

## Introduction

The MGC has the ability of extending the DNS configuration of the gateway with the DNSPolicy resource. This resource allows users to configure health checks. As a result of configuring health checks, the controller creates the health checks in Route53, attaching them to the related DNS records. This has the benefit of automatically disabling an endpoint if it becomes unhealthy, and enabling it again when it becomes healthy again.
This feature has a few shortfalls:

1. It’s tightly coupled with Route53. If other DNS providers are supported they must either provide a similar feature, or health checks will not be supported
2. Lacks the ability to reach endpoints in private networks

This document describes a proposal to extend the current health check implementation to overcome these shortfalls.

### Goals

* Ability to configure health checks in the DNSPolicy associated to a Gateway
* Ability for the health checks to reach endpoints in private networks
* DNS records are disabled when the associated health check fails
* Current status of the defined health checks is visible to the end user
* ~~Transparently keep support for other health check providers like Route53~~

## Proposal

#### `DNSPolicy` resource

The presence of the `healthCheck` field in the DNSPolicy will affect the reconciliation
of the endpoints when creating/updating DNSRecords:

* For every endpoint that is generated, a health check is created based on the configuration in the DNSPolicy
* Relevant `providerSpecific` fields are added to the endpoint:
  * A reference to the equivalent `DNSHealthCheckProbe` in the downstream cluster
  will be set: `health-probe/name`, `health-probe/namespace`
  * The health status of the `DNSHealthCheckProbe` will be propagated

A `failureThreshold` field will be added to the health spec, allowing users
to configure how many consecutive health check failures are observed before
taking the unhealthy endpoint down

#### `DNSHealthCheckProbe` resource

The DNSHealthCheckProbe resource configures a health probe in the controller to perform the health checks against a local endpoint.

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: DNSHealthCheckProbe
metadata:
  name: example-probe
spec:
  port: "..."
  host: “...”
  ipAddress: "..."
  path: "..."
  protocol: "..."
  interval: "..."
status:
  healthy: true
  consecutiveFailures: 0
  reason: ""
  lastCheck: "..."
```

The reconciliation of this resource results in the configuration of a health probe,
which targets the endpoint and updates the status. The status is propagated to the providerSpecific status of the equivalent endpoint in the DNSRecord

### Changes to current controllers

In order to support this new feature, the following changes in the behaviour of the controllers are proposed.

#### DNSPolicy controller

Currently the reconciliation loop of this controller is in charge of creating health checks in the configured DNS provider (Route53 currently) based on the spec of the DNSPolicy, separately from the reconciliation of the DNSRecords. The proposed change is to reconcile health checks as the DNSRecords are created, using the health check
configuration to alter the behaviour of the DNSRecord reconciliation.

Instead of Route53 health checks, the controller will create `DNSHealthCheckProbe` resources

#### DNSRecord controller

Currently the reconciliation loop of this controller updates the DNS records using the aws/health-check-id provider specific field, that is updated as a result of the DNSPolicy reconciliation. The proposed change is to extend the functionality of acting on provider specific fields, to disable/enable the DNS record if the health-probe/healthy field is Unhealthy. This can be achieved by either deleting/re-creating the DNS record, or setting the weight to 0 in order to disable it