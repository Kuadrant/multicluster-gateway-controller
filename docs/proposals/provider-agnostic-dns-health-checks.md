# Provider agnostic DNS Health checks

## Introduction

The MGC has the ability to extend the DNS configuration of the gateway with the DNSPolicy resource. This resource allows 
users to configure health checks. As a result of configuring health checks, the controller creates the health checks in 
Route53, attaching them to the related DNS records. This has the benefit of automatically disabling an endpoint if it 
becomes unhealthy, and enabling it again when it becomes healthy again.

This feature has a few shortfalls:
1. It’s tightly coupled with Route53. If other DNS providers are supported they must either provide a similar feature, 
or health checks will not be supported
2. Lacks the ability to reach endpoints in private networks
3. requires using the gateway controller to implement, maintain and test multiple providers

This document describes a proposal to extend the current health check implementation to overcome these shortfalls.

### Goals

* Ability to configure health checks in the DNSPolicy associated to a Gateway
* DNS records are disabled when the associated health check fails
* Current status of the defined health checks is visible to the end user

### Nongoals
* Ability for the health checks to reach endpoints in private networks
* Transparently keep support for other health check providers like Route53

## Proposal

#### `DNSPolicy` resource

The presence of the `healthCheck` means that for every DNS endpoint (that is either an A record, or a CNAME to an external host), 
a health check is created based on the health check configuration in the DNSPolicy.

A `failureThreshold` field will be added to the health spec, allowing users to configure a number of consecutive health 
check failures that must be observed before the endpoint is considered unhealthy.

Example DNS Policy with a defined health check.
```yaml
apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
metadata:
  name: prod-web
  namespace: multi-cluster-gateways
spec:
  healthCheck:
    endpoint: /health
    failureThreshold: 5
    port: 443
    protocol: https
  targetRef:
    group: gateway.networking.k8s.io
    kind: Gateway
    name: prod-web
    namespace: multi-cluster-gateways
```
#### `DNSHealthCheckProbe` resource

The DNSHealthCheckProbe resource configures a health probe in the controller to perform the health checks against an 
identified final A or CNAME endpoint. When created by the controller as a result of a DNS Policy, this will have an 
owner ref of the DNS Policy that caused it to be created.

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: DNSHealthCheckProbe
metadata:
  name: example-probe
spec:
  port: "..."
  host: “...”
  address: "..."
  path: "..."
  protocol: "..."
  interval: "..."
status:
  healthy: true
  consecutiveFailures: 0
  reason: ""
  lastCheck: "..."
```

#### Spec Fields Definition
- **Port** The port to use
- **Address** The address to connect to (e.g. IP address or hostname of a clusters loadbalancer)
- **Host** The host to request in the Host header
- **Path** The path to request
- **Protocol** The protocol to use for this request
- **Interval** How frequently this check would ideally be executed.

The reconciliation of this resource results in the configuration of a health probe, which targets the endpoint and 
updates the status. The status is propagated to the providerSpecific status of the equivalent endpoint in the DNSRecord

### Changes to current controllers

In order to support this new feature, the following changes in the behaviour of the controllers are proposed.

#### DNSPolicy controller

Currently, the reconciliation loop of this controller creates health checks in the configured DNS provider 
(Route53 currently) based on the spec of the DNSPolicy, separately from the reconciliation of the DNSRecords. 
The proposed change is to reconcile health check probe CRs based on the combination of DNS Records and DNS Policies.

Instead of Route53 health checks, the controller will create `DNSHealthCheckProbe` resources.

#### DNSRecord controller

When reconciling a DNS Record, the DNS Record reconciler will retrieve the relevant DNSHealthCheckProbe CRs, and consult
the status of them when determining what value to assign to a particular endpoint's weight. 

There are a few scenarios to cover when considering the removal of DNS Record endpoints due to unhealthiness.

1. All endpoints are reporting unhealthy responses, then the weight will be assigned as if they are all healthy
1. One or more, but not all endpoints are reporting an unhealthy response, then the weight will be assigned 0 for the 
unhealthy endpoints, otherwise the usual weight assigning process will be executed.

## Executing the probes
There will be a 