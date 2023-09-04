# Kuadrant DNSPolicy Demo

## Goals
* Show changes in how MGC manages DNS resources through a direct attachment DNS policy
* Show changes to the DNS Record structure
* Show weighted load balancing strategy and how it can be configured
* Show geo load balancing strategy and how it can be configured

## Setup

```bash
# make local-setup OCM_SINGLE=true MGC_WORKLOAD_CLUSTERS_COUNT=2
```

```bash
./install.sh
(export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build-controller install run-controller)
```
## Preamble

Three managed clusters labeled as ingress clusters
```bash
kubectl get managedclusters --show-labels
```

Show managed zone
```bash
kubectl get managedzones -n multi-cluster-gateways
```

Show gateway created on the hub
```bash
kubectl get gateway -n multi-cluster-gateways
```
Show gateways 
```bash
# Check gateways
kubectl --context kind-mgc-control-plane get gateways -A
kubectl --context kind-mgc-workload-1 get gateways -A
kubectl --context kind-mgc-workload-2 get gateways -A
```

Show application deployed to each cluster
```bash
curl -k -s -o /dev/null -w "%{http_code}\n" https://bfa.jm.hcpapps.net --resolve 'bfa.jm.hcpapps.net:443:172.31.200.0'
curl -k -s -o /dev/null -w "%{http_code}\n" https://bfa.jm.hcpapps.net --resolve 'bfa.jm.hcpapps.net:443:172.31.201.0'
curl -k -s -o /dev/null -w "%{http_code}\n" https://bfa.jm.hcpapps.net --resolve 'bfa.jm.hcpapps.net:443:172.31.202.0'
```

Show status of gateway on the hub:
```bash
kubectl get gateway prod-web -n multi-cluster-gateways -o=jsonpath='{.status}'
```

## DNSPolicy using direct attachment

Explain the changes that have been made to the dns reconciliation, that it now uses direct policy attachement and that a DNSPOlicy must be created and attached to a target gateway before any dns updates will be made for a gateway. 

Show no dnsrecord
```bash
kubectl --context kind-mgc-control-plane get dnsrecord -n multi-cluster-gateways
```

Show no response for host
```bash
# Warning, will cache for 5 mins!!!!!!
curl -k https://bfa.jm.hcpapps.net
```

Show no dnspolicy
```bash
kubectl --context kind-mgc-control-plane get dnspolicy -n multi-cluster-gateways
```

Create dnspolicy
```bash
cat resources/dnspolicy_prod-web-default.yaml
kubectl --context kind-mgc-control-plane apply -f resources/dnspolicy_prod-web-default.yaml -n multi-cluster-gateways
```

```bash
# Check policy attachment
kubectl --context kind-mgc-control-plane get gateway prod-web -n multi-cluster-gateways -o=jsonpath='{.metadata.annotations}'
```

Show dnsrecord created
```bash
kubectl --context kind-mgc-control-plane get dnsrecord -n multi-cluster-gateways
```





Show response for host
```bash
curl -k https://bfa.jm.hcpapps.net
```

## DNS Record Structure

Show the new record structure

```bash
kubectl get dnsrecord prod-web-api -n multi-cluster-gateways -o=jsonpath='{.spec.endpoints}'
```

## Weighted loadbalancing by default

Show and update default weight in policy (Show result sin Route53)
```bash
kubectl --context kind-mgc-control-plane edit dnspolicy prod-web -n multi-cluster-gateways
```

"A DNSPolicy with an empty `loadBalancing` spec, or with a `loadBalancing.weighted.defaultWeight` set and nothing else produces a set of records grouped and weighted to produce a [Round Robin](https://en.wikipedia.org/wiki/Round-robin_DNS) routing strategy where all target clusters will have an equal chance of being returned in DNS queries."

## Custom Weighting

Edit dnsPolicy and add custom weights:
```bash
kubectl --context kind-mgc-control-plane edit dnspolicy prod-web -n multi-cluster-gateways
```

```yaml
spec:
  loadBalancing:
    weighted:
      custom:
      - value: AWS
        weight: 200
      - value: GCP
        weight: 10
      defaultWeight: 100
```

Add custom weight labels
```bash
kubectl get managedclusters --show-labels
kubectl label --overwrite managedcluster kind-mgc-workload-1 kuadrant.io/lb-attribute-custom-weight=AWS
kubectl label --overwrite managedcluster kind-mgc-workload-2 kuadrant.io/lb-attribute-custom-weight=GCP
```

## Geo load balancing

Edit dnsPolicy and add default geo:
```bash
kubectl --context kind-mgc-control-plane edit dnspolicy prod-web -n multi-cluster-gateways
```

```yaml
spec:
  loadBalancing:
    geo:
      defaultGeo: US
    weighted:
      custom:
      - value: AWS
        weight: 20
      - value: GCP
        weight: 200
      defaultWeight: 100
```

Add geo labels
```bash
kubectl get managedclusters --show-labels
kubectl label --overwrite managedcluster kind-mgc-workload-1 kuadrant.io/lb-attribute-geo-code=FR
kubectl label --overwrite managedcluster kind-mgc-workload-2 kuadrant.io/lb-attribute-geo-code=ES


Checkout that DNS:

https://www.whatsmydns.net/#A/bfa.jm.hcpapps.net