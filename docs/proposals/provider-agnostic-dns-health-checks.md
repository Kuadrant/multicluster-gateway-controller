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
* Ability for the health checks to reach endpoints in separate private networks
* Transparently keep support for other health check providers like Route53
* Having health checks for wildcard listeners

## Use-cases
* As a gateway administrator, I would like to define a health check that each service sitting behind a particular 
listener across the production clusters has to implement to ensure we can automatically respond, failover and 
mitigate a failing instance of the service

## Proposal

Currently, this functionality will be added to the existing MGC, and executed within that component. This will be created
with the knowledge that it may need to be made into an external component in the future.

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
    additionalHeaders: <SecretRef>
    expectedResponses:
      - 200
      - 301
      - 302
      - 407
    AllowInsecureCertificates: true
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
  additionalHeaders: <SecretRef>
  expectedResponses:
  - 200
    201
    301
  AllowInsecureCertificate: true
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
- **AdditionalHeaders** Optional secret ref which contains k/v: headers and their values that can be specified to ensure the health check is successful.
- **ExpectedResponses** Optional HTTP response codes that should be considered healthy (defaults are 200 and 201).
- **AllowInsecureCertificate** Optional flag to allow using invalid (e.g. self-signed) certificates, default is false.


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

## DNS Record Structure Diagram:

https://lucid.app/lucidchart/2f95c9c9-8ddf-4609-af37-48145c02ef7f/edit?viewport_loc=-188%2C-61%2C2400%2C1183%2C0_0&invitationId=inv_d5f35eb7-16a9-40ec-b568-38556de9b568
How

## Removing unhealthy Endpoints
When a DNS health check probe is failing, it will update the DNS Record CR with a custom field on that endpoint to mark it as failing.

There are then 3 scenarios which we need to consider:
1 - All endpoints are healthy
2 - All endpoints are unhealthy
3 - Some endpoints are healthy and some are unhealthy.

In the cases 1 and 2, the result should be the same: All records are published to the DNS Provider.

When scenario 3 is encountered the following process should be followed:

    For each gateway IP or CNAME: this should be omitted if unhealthy.
    For each managed gateway CNAME: This should be omitted if all child records are unhealthy.
    For each GEO CNAME: This should be omitted if all the managed gateway CNAMEs have been omitted.
    Load balancer CNAME: This should never be omitted.

If we consider the DNS record to be a hierarchy of parents and children, then whenever any parent has no healthy children that parent is also considered unhealthy. No unhealthy elements are to be included in the DNS Record.

## Executing the probes
There will be a DNSHealthCheckProbe CR controller added to the controller. This controller will create an instance of a 
`HealthMonitor`, the HealthMonitor ensures that each DNSHealthCheckProbe CR has a matching probeQueuer object running.
It will also handle both the updating of the probeQueuer on CR update and the removal of probeQueuers, when a 
DNSHealthcheckProbe is removed.

The `ProbeQueuer` will add a health check request to a queue based on a configured interval, this queue is consumed by a
`ProbeWorker`, probeQueuers work on their own goroutine.

The ProbeWorker is responsible for actually executing the probe, and updating the DNSHealthCheckProbe CR status. The 
probeWorker executes on its own goroutine.
