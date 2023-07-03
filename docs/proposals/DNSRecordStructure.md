DNSRecord is our API for expressing DNS endpoints via a kube CRD based API. It is managed by the multi-cluster gateway controller based on the desired state expressed in higher level APIs such as the Gateway or a DNSPolicy. In order to provide our feature set, we need to carefully consider how we structure our records and the types of records we need. This document proposes a partcicular structure based on the requirements and feature set we have.


## Requirements

We want to be able to support Gateway definitions that use the following listener definitions:

- wildcard: `*.example.com` and fully qualified listener host `www.example.com` definitions with the notable exception of fully wildcarded ie `*` as we cannot provide any DNS or TLS for something with no defined hostname.
- listeners that have HTTPRoute defined on less than all of the clusters where the listener is available. IE we don't want to send traffic to clusters where there is no HTTPRoute attached to the listener.
- Gateway instances that provide IPs that are deployed alongside instances on different infra that provide host names causing the addresses types on each of gateway instance to be different (IPAddress or HostAddress). 
- We want to provide GEO based DNS as a feature of DNSPolicy and so our DNSRecord structure must support this.
- We want to offer default weighted and custom weighted DNS as part of DNSPolicy
- We want to allow root or apex domain to be used as listener hosts


## Proposal

For each listerner defined in a gateway, we will create a set of records with the following rules.

**none apex domain:**

We will have a generated lb (load balancer) dns name that we will use as a CNAME for the listener hostname. This DNS name is not intended for use within a HTTPRoute but is instead just a DNS construct. This will allow us to set up additional CNAME records for that DNS name in the future that are returned based a GEO location. These DNS records will also be CNAMES pointing to specific gateway dns names, this will allow us to setup a weighted response. So the first layer CNAME handles balancing based on geo, the second layer handles balancing based on weighting. 

                                            shop.example.com
                                            |             |
                                          (IE)          (AUS)
                                    CNAME lb.shop..      lb.shop..
                                        |     |         |      |
                                     (w 100) (w 200)   (w 100) (w100)
                                    CNAME g1.lb.. g2.lb..   g3.lb..  g4.lb..
                                    A 192..   A 81..  CNAME  aws.lb   A 82..

When there is no geo strategy defined within the DNSPolicy, we will put everything into a default geo (IE a catch all record) `default.lb-{guid}.{listenerHost}` but set the routing policy to GEO allowing us to add more geo based records in the future if the gateway admin decides to move to a geo strategy as their needs grow. 


To ensure this lb dns name is unique and does not clash we will use a short guid as part of the subdomain so `lb-{guid}.{listenerHost}.` this guid will be based on the gateway name and gateway namespace in the control plane.

For a geo strategy we will add a geo record with a prefix to the lb subdomain based on the geo code. When there is no geo we will use `default` as the prefix. `{geo-code}.lb-{guid}.{listenerHost}`.
Finally for each gateway instance on a target cluster we will add a `{spokeClusterName}.lb-{guid}.{listenerHost}`


To allow for a mix of hostname and IP address types, we will always use a CNAME . So we will create a dns name for IPAddress with the following structure: `{guid}.lb-{guid}.{listenerHost}` where the first guid will be based on the cluster name where the gateway is placed.


### Apex Domains

An apex domain is the domain at the apex or root of a zone. These are handled differently by DNS as they often have NS and SOA records. Generally it is not possible to set up a CNAME for apex domain (although some providers alow it).

If a listener is added to a gateway that is an apex domain, we can only add A records for that domain to keep ourselves compliant with as many providers as possible.
If a listener is the apex domain, we will setup A records for that domain (favouring gateways with an IP address or resolving the IP behnd a host) but there will be no special balancing/weighting done. Instead we will expect that the owner of that will setup a HTTPRoute with a 301 permanent redirect sending users from the apex domain e.g example.com to something like: www.example.com where the www subdomain based listener would use the rules of the none apex domains and be where advanced geo and weighted strategis are applied.

- gateway listener host name : example.com 
    -  example.com A 81.17.241.20

### Geo Agnostic (everything is in a default * geo catch all) 

This is the type of DNS Record structure that would back our default DNSPolicy.

- gateway listener host name : www.example.com 

    **DNSRecords:**
    -  www.example.com CNAME lb-1ab1.www.example.com 
    -  lb-1ab1.www.example.com CNAME geolocation * default.lb-1ab1.www.example.com 
    -  default.lb-1ab1.www.example.com CNAME weigted 100 1bc1.lb-1ab1.www.example.com
    -  default.lb-1ab1.www.example.com CNAME weigted 100 aws.lb.com
    -  1bc1.lb-1ab1.www.example.com A 192.22.2.1


So in the above example working up from the bottom, we have a mix of hostname and IP based addresses for the gateway instance. We have 2 evenally weighted records that balance between the two available gateways, then next we have the geo based record that is set to a default catch all as no geo has been specified then finally we have the actual listener hostname that points at our DNS based load balancer name.


DNSRecord Yaml

```yaml 
apiVersion: kuadrant.io/v1alpha1
kind: DNSRecord
metadata:
  name: {gateway-name}-{listenerName}
  namespace: multi-cluster-gateways
spec:
  dnsName: www.example.com
  managedZone:
    name: mgc-dev-mz
  endpoints:
    - dnsName: www.example.com
      recordTTL: 300
      recordType: CNAME
      targets:
        - lb-1ab1.www.example.com
    - dnsName: lb-1ab1.www.example.com
      recordTTL: 300
      recordType: CNAME
      setIdentifier: mygateway-multicluster-gateways
      providerSpecific:
        - name: "geolocation-country-code"
          value: "*"
      targets:
        - default.lb-1ab1.www.example.com
    - dnsName: default.lb-1ab1.www.example.com
      recordTTL: 300
      recordType: CNAME
      setIdentifier: cluster1
      providerSpecific:
        - name: "weight"
          value: "100"
      targets:
        - 1bc1.lb-1ab1.www.example.com
    - dnsName: default.lb-a1b2.shop.example.com
      recordTTL: 300
      recordType: CNAME
      setIdentifier: cluster2
      providerSpecific:
        - name: "weight"
          value: "100"
      targets:
        - aws.lb.com
    - dnsName: 1bc1.lb-1ab1.www.example.com
      recordTTL: 60
      recordType: A
      targets:
        - 192.22.2.1
```


### geo specific

Once the end user selects to use a geo strategy via the DNSPolicy, we then need to restructure our DNS to add in our geo specific records. Here the default record

lb short code is {gw name + gw namespace}
gw short code is {cluster name}

- gateway listener host : shop.example.com  

    **DNSRecords:**
    -  shop.example.com CNAME lb-a1b2.shop.example.com 
    -  lb-a1b2.shop.example.com  CNAME geolocation ireland ie.lb-a1b2.shop.example.com 
    -  lb-a1b2.shop.example.com  geolocation australia aus.lb-a1b2.shop.example.com 
    -  lb-a1b2.shop.example.com  geolocation default ie.lb-a1b2.shop.example.com  (set by the default geo option)
    -  ie.lb-a1b2.shop.example.com   CNAME weigted 100 ab1.lb-a1b2.shop.example.com 
    -  ie.lb-a1b2.shop.example.com   CNAME weigted 100 aws.lb.com
    -  aus.lb-a1b2.shop.example.com  CNAME weigted 100 ab2.lb-a1b2.shop.example.com 
    -  aus.lb-a1b2.shop.example.com  CNAME weigted 100 ab3.lb-a1b2.shop.example.com 
    -  ab1.lb-a1b2.shop.example.com  A 192.22.2.1 192.22.2.5
    -  ab2.lb-a1b2.shop.example.com  A 192.22.2.3
    -  ab3.lb-a1b2.shop.example.com  A 192.22.2.4

In the above example we move from a default catch all to geo sepcific setup. Based on a DNSPolicy that specifies IE as the default geo location. We leave the `default` subdomain in place to allow for clients that may still be using that and set up geo specific subdomains that allow us to route traffic based on its origin. In this example we are loadbalancing across 2 geos and 4 clusters


### WildCards

In the examples we have used fully qualifed domain names, however sometimes it may be required to use a wildcard subdomain. example:

- gateway listener host : *.example.com  

To support these we need to change the name of the DNSRecord away from the name of the listener as the k8s resource does not allow * in the name. 

To do this we will set the dns record resource name to be a combination of `{gateway-name}-{listenerName}`

to keep a record of the host this is for we will set a top level property named `dnsName`. You can see an example in the DNSRecord above.




## Pros

This setup allows us a powerful set of features and flexibility

## Cons

With this CNAME based approach we are increasing the number of DNS lookups required to get to an IP which will increase the cost and add a small amount of latency. To counteract this, we will set a reasonably high TTL (at least 5 mins) for our CNAMES and (2 mins) for A records