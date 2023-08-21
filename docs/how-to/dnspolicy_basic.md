# Defining a basic DNSPolicy

## What is a DNSPolicy

DNSPolicy is a Custom Resource Definition supported by the MGC that follows the
[policy attachment model](https://gateway-api.sigs.k8s.io/references/policy-attachment/), which allows users to enable and configure DNS against the Gateway.

This document describes how to enable DNS by creating a basic DNSPolicy

## Pre-requisites

* A [ManagedZone](managedZone.md) has been created
* A Gateway has been created with an attached HTTPRoute

> See [the OCM walkthrough](ocm-control-plane-walkthrough.md) for step by step
instructions on deploying these with a simple application.

## Steps

The DNSPolicy will target the existing Multi Cluster Gateway, resulting in the
creation of DNS Records targeting the Gateway listeners, as well as optional
DNS health checks and load balancing.

In order to enable basic DNS, create a minimal DNSPolicy resource

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
metadata:
  name: basic-dnspolicy
  namespace: <Gateway namespace>
spec:
  targetRef:
    name: <Gateway name>
    group: gateway.networking.k8s.io
    kind: Gateway     
```

Once created, the Multicluster Gateway Controller will reconcile the DNS records. You
can see the status by querying the DNSRecord resources.

```sh
kubectl get dnsrecords -A
```

The DNS records will be propagated in a few minutes, and the application will
be available through the defined hosts.

## Advanced DNS configuration

The DNSPolicy supports other optional configuration options like load balancing
and DNS health checks. For more detailed information about these options, see [DNSPolicy reference](../dns-policy.md)