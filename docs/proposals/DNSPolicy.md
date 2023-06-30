# DNS Policy


## Problem

Gateway admins, need a way to define the DNS policy for a multi-cluster gateway in order to control how much and which traffic reaches these gateways. 
Ideally we would allow them to express a strategy that they want to use without needing to get into the details of each provider and needing to create and maintain dns record structure and individual records for all the different gateways that may be within their infrastructure.

**Use Cases**

As a gateway admin, I want to be able to reduce latency for my users by routing traffic based on the GEO location of the client. I want this strategy to automatically expand and adjust as my gateway topology grows and changes.

As a gateway admin, I have a discount with a particular cloud provider and want to send more of my traffic to the gateways hosted in that providers infrastructure and as I add more gateways I want that balance to remain constant and evolve to include my new gateways.



## Goals

- Allow definition of a DNS load balancing strategy to decide how traffic should be weighted across multiple gateway instances from the central control plane.


## None Goals

- Allow different DNS policies for different listeners. Although this may be something we look to support in the future, currently policy attachment does not allow for this type of targeting. This means a DNSPolicy is applied for the whole gateway currently. 
- Define how health checks should work, this will be part of a separate proposal


## Terms

- **managed listener**: This is a listener with a host backed by a DNS zone managed by the multi-cluster gateway controller
- **hub cluster**: control plane cluster that managed 1 or more spokes
- **spoke cluster**: a cluster managed by the hub control plane cluster. This is where gateway are instantiated

## Proposal

Provide a control plane DNSPolicy API that uses the idea of direct [policy attachment](https://gateway-api.sigs.k8s.io/references/policy-attachment/#direct-policy-attachment) from gateway API that allows a load balancing strategy to be applied to the DNS records structure for any managed listeners being served by the data plane instances of this gateway. 
The DNSPolicy also covers health checks that inform the DNS response but that is not covered in this document.

Below is a draft API for what we anticipate the DNSPolicy to look like

```yaml 

apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
spec:
  targetRef: # defaults to gateway gvk and currrent namespace
    name: gateway-name
  health:
   ...
  loadBalancing:
    weighted: # always requird
     default: 10  #always required
     custom: #optional
     - value: AWS  #optional with both GEO and weighted. With GEO the custom weight is applied to gateways within a Geographic region
       weight: 10
     - value: GCP
       weight: 20
    GEO: #optional
      default: IE # required with GEO. Choses a default DNS response when no particular response is defined for a request from an unknown GEO.
```  

### Available Load Balancing Strategies  

GEO and Weighted load balancing are well understood strategies and this API effectively allow a complex requirement to be expressed relatively simply and executed by the gateway controller in the chosen DNS provider. Our default policy will execute a "Round Robin" weighted strategy which reflects the current default behaviour.

With the above API we can provide weighted and GEO and weighted within a GEO. A weighted strategy with a minimum of a default weight is always required and the simplest type of policy. The multi-cluster gateway controller will set up a default policy when a gateway is discovered (shown below). This policy can be replaced or modified by the user.  A weighted strategy can be complimented with a GEO strategy IE they can be used together in order to provide a GEO and weighted (within a GEO) load balancing. By defining a GEO section, you are indicating that you want to use a GEO based strategy (how this works is covered below).
  

```yaml 

apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
name: default-policy
spec:
  targetRef: # defaults to gateway gvk and currrent namespace
    name: gateway-name
  loadBalancing:
    weighted: # required
     default: 10  #required, all records created get this weight
  health:
   ...   
```  

In order to provide GEO based DNS and allow customisation of the weighting, we need some additional information to be provided by the gateway / cluster admin about where this gateway has been placed. For example if they want to use GEO based DNS as a strategy, we need to know what GEO identifier(s) to use for each record we create and a default GEO to use as a catch all. Also if the desired load balancing approach is to provide custom weighting and no longer simply use Round Robin, we will need a way to identify which records to apply that custom weighting to based on the clusters the gateway is placed on.

To solve this we will allow two new attributes to be added to the `ManagedCluster` resource as labels:

```
   kuadrant.io/lb-attribute-GEO-<GEO-code-type>: "IE"
   kuadrant.io/lb-attribute-custom-weight: "GCP"
```

These two labels allow setting values in the DNSPolicy that will be reflected into DNS records for gateways placed on that cluster depending on the strategies used. (see the first DNSPolicy definition above to see how these values are used) or take a look at the examples at the bottom.


example :
```yaml
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
 labels:
   kuadrant.io/lb-attribute-GEO-country-code: "IE"
   kuadrant.io/lb-attribute-custom-weight: "GCP"
spec:    
```  

The attributes provide the key and value we need in order to understand how to define records for a given LB address based on the DNSPolicy targeting the gateway.


### DNS Record Structure

This is an advanced topic and so is broken out into its own proposal doc [DNS Record Structure](./DNSRecordStructure.md)


### Custom Weighting 

Custom weighting will use the associated `custom-weight` attribute set on the `ManagedCluster` to decide which records should get a specific weight. The value of this attribute is up to the end user.

example:

```yaml
apiVersion: cluster.open-cluster-management.io/v1
kind: ManagedCluster
metadata:
 labels:
   kuadrant.io/lb-attribute-custom-weight: "GCP"
```

The above is then used in the DNSPolicy to set custom weights for the records associated with the target gateway.

```YAML
    - value: GCP
      weight: 20
```        

So any gateway targeted by a DNSPolicy with the above definition that is placed on a `ManagedCluster` with the `kuadrant.io/lb-attribute-custom-weight` set with a value of GCP will get an A record with a weight of 20 




### Status

DNSPolicy should have a ready condition that reflect that the DNSrecords have been created and configured as expected. In the case that there is an invalid policy, the status message should reflect this and indicate to the user that the old DNS has been preserved.

We will also want to add a status condition to the gateway status indicating it is effected by this policy. Gateway API recommends the following status condition 

```yaml 

- type: gateway.networking.k8s.io/PolicyAffected
  status: True 
  message: "DNSPolicy has been applied"
  reason: PolicyApplied
  ...
```

https://github.com/kubernetes-sigs/gateway-api/pull/2128/files#diff-afe84021d0647e83f420f99f5d18b392abe5ec82d68f03156c7534de9f19a30aR888


## Example Policies

### Round Robin (the default policy)

```yaml 

apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
name: RoundRobinPolicy
spec:
  targetRef: # defaults to gateway gvk and currrent namespace
    name: gateway-name
  loadBalancing:
    weighted:
     default: 10
``` 

### GEO (Round Robin)

```yaml 

apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
name: GEODNS
spec:
  targetRef: # defaults to gateway gvk and currrent namespace
    name: gateway-name
  loadBalancing:
    weighted:
     default: 10
    GEO:
     default: IE
``` 


### Custom

```yaml 

apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
name: SendMoreToAzure
spec:
  targetRef: # defaults to gateway gvk and currrent namespace
    name: gateway-name
  loadBalancing:
    weighted:
     default: 10
     custom:
     - attribute: cloud
       value: Azure #any record associated with a gateway on a cluster without this value gets the default
       weight: 30

``` 


### GEO with Custom Weights

```yaml 

apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
name: GEODNSAndSendMoreToAzure
spec:
  targetRef: # defaults to gateway gvk and currrent namespace
    name: gateway-name
  loadBalancing:
    weighted:
     default: 10
     custom:
     - attribute: cloud
       value: Azure
       weight: 30
    GEO:
      default: IE
``` 



## Considerations and Limitations

You cannot have a different load balancing strategy for each listener within a gateway. So in the following gateway definition

``` yaml

spec:
    gatewayClassName: kuadrant-multi-cluster-gateway-instance-per-cluster
    listeners:
    - allowedRoutes:
        namespaces:
          from: All
      hostname: myapp.hcpapps.net
      name: api
      port: 443
      protocol: HTTPS
    - allowedRoutes:
        namespaces:
          from: All
      hostname: other.hcpapps.net
      name: api
      port: 443
      protocol: HTTPS      

```

The DNS policy targeting this gateway will apply to both myapp.hcpapps.net and other.hcpapps.net

However there is still significant value even with this limitation. This limitation is something we will likely revisit in the future


## Background Docs

[DNS Provider Support](multiple-dns-provider-support.md) 

[AWS DNS](assets/multiple-dns-provider-support/aws/aws.md)

[Google DNS](assets/multiple-dns-provider-support/google/google.md)

[Azure DNS](assets/multiple-dns-provider-support/azure/azure.md)


[Direct Policy Attachment](https://gateway-api.sigs.k8s.io/references/policy-attachment/#direct-policy-attachment) 
