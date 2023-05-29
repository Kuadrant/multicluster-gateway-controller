# Multiple DNS Provider Support

Authors: Michael Nairn @mikenairn

Epic: https://github.com/Kuadrant/multicluster-gateway-controller/issues/189

Date: 25th May 2023


## Job Stories

- As a developer, I want to use MGC with a domain hosted in one of the major cloud DNS providers (Google Cloud DNS, Azure DNS or AWS Route53) 
- As a developer, I want to use multiple domains with a single instance of MGC, each hosted on different cloud providers

## Goals

- Add ManagedZone and DNSRecord support for [Google Cloud DNS](https://cloud.google.com/dns/docs/)
- Add ManagedZone and DNSRecord support for [Azure DNS](https://azure.microsoft.com/en-us/services/dns)
- Add DNSRecord support for [CoreDNS](https://coredns.io/) (Default for development environment)
- Update ManagedZone and DNSRecord support for [AWS Route53](https://aws.amazon.com/route53/)
- Add support for multiple providers with a single instance of MGC

## Non Goals

- Support for every DNS provider
- Support for health checks

## Current Approach

Currently, MGC only supports AWS Route53 as a dns provider. A single instance of a DNSProvider resource is created per MGC instance which is configured with AWS config loaded from the environment. 
This provider is loaded into all controllers requiring dns access (ManagedZone and DNSRecord reconciliations), allowing a single instance of MGC to operate against a single account on a single dns provider.

## Proposed Solution

MGC has three features it requires of any DNS provider in order to offer full support, DNSRecord management, Zone management and DNS Health checks.  We do not however want to limit to providers that only offer this functionality, so to add support for a provider the minimum that provider should offer is API access to managed DNS records.
MGC will continue to provide Zone management and DNS Health checks support on a per-provider basis.

Support will be added for AWS(Route53), Google(Google Cloud DNS), Azure and investigation into possible adding CoreDNS (intended for local dev purposes), with the following proposed initial support:

| Provider  | DNS Records | DNS Zones | DNS Health |
| ------------ | ------------- |  ------------- |---|
| AWS Route53 | X | X | X |
| Google Cloud DNS | X | X | - |
| AzureDNS| X | X | - |
| CoreDNS| X | - | - |


Add DNSProvider as an API for MGC which contains all the required config for that particular provider including the credentials. This can be thought of in a similar way to a cert manager Issuer.
Update ManagedZone to add a reference to a DNSProvider. This will be a required field on the ManagedZone and a DNSProvider must exist before a ManagedZone can be created.
Update all controllers load the DNSProvider directly from the ManagedZone during reconciliation loops and remove the single controller wide instance. 
Add new provider implementations for [google](assets/multiple-dns-provider-support/google/google.md), [azure](assets/multiple-dns-provider-support/azure/azure.md) and coredns.
    * All providers constructors should accept a single struct containing all required config for that particular provider.
    * Providers must be configured from credentials passed in the config and not rely on environment variables.

## Other Solutions investigated

Investigation was carried out into the suitability of [External DNS] (https://github.com/kubernetes-sigs/external-dns) as the sole means of managing dns resources.
Unfortunately, while external dns does offer support for basic dns record management with a wide range of providers, there were too many features missing making it unsuitable at this time for integration.

### External DNS as a separate controller

Run external dns, as intended, as a separate controller alongside mgc, and pass all responsibility for reconciling DNSRecord resources to it. All DNSRecord reconciliation is removed from MGC.

Issues:

* A single instance of external dns will only work with a single provider and a single set of credentials. As it is, in order to support more than a single provider, more than one external dns instance would need to be created, one for each provider/account pair.
* Geo and Weighted routing policies are not implemented for any provider other than AWS Route53.
* Only supports basic dns record management (A,CNAME, NS records etc ..), with no support for managed zones or health checks.

### External DNS as a module dependency

Add external dns as a module dependency in order to make use of their DNS Providers, but continue to reconcile DNSRecords in MGC.

Issues:

* External DNS Providers all create clients using the current environment. Would require extensive refactoring in order to modify each provider to optionally be constructed using static credentials.
* Clients were all internal making it impossible, without modification, to use the upstream code to extend the provider behaviour to support additional functionality such as managed zone creation.

## Checklist

- [ ] An epic has been created and linked to
- [ ] Reviewers have been added. It is important that the right reviewers are selected.
