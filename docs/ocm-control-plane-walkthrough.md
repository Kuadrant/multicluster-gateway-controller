# Open Cluster Management and Multi-Cluster gateways


This document will walk you through using Open Cluster Management and Kuadrant to deploy and integrate with a multi-cluster gateway. 


## Requirements

- Kind
- AWS Route 53 (for now)

## Installation and Setup

- Clone this repo locally 
- run `make local-setup OCM_SINGLE=true MCTC_WORKLOAD_CLUSTERS_COUNT=1`
- setup a `./controller-config.env` with the following key values

```
    # this sets up your default managed zone
    AWS_DNS_PUBLIC_ZONE_ID=<AWS ZONE ID>
    ZONE_ROOT_DOMAIN=some.domain.com
    LOG_LEVEL=1
```   

- setup a `./aws-credentials.env` with credentials to access route 53

For example:

```
AWS_ACCESS_KEY_ID=<access_key_id>
AWS_SECRET_ACCESS_KEY=<secret_access_key>
AWS_REGION=eu-west-1
```


session/window 1
```
kind export kubeconfig --name=mctc-workload-1 --kubeconfig=$(pwd)/local/kube/workload1.yaml && export KUBECONFIG=$(pwd)/local/kube/workload1.yaml
```

Let make these clusters usable by the gateway controller:

In the hub cluster context execute the following commands:

```
k label managedcluster kind-mctc-control-plane ingress-cluster=true

```

Next create the managed clusterset. This is a group of clusters that meet certain criterea. 

```
kubectl apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1beta2
kind: ManagedClusterSet
metadata:
  name: gateway-clusters
spec:
  clusterSelector:
    labelSelector: 
      matchLabels:
        ingress-cluster: "true"
    selectorType: LabelSelector
EOF
```    

Next bind this cluster set to our multi-cluster-gateways namespace (setup with make local-setup)

```
kubectl apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1beta2
kind: ManagedClusterSetBinding
metadata:
  name: gateway-clusters
  namespace: multi-cluster-gateways
spec:
  clusterSet: gateway-clusters
EOF
```


### Create a placement for our gateways

In order to place our gateways onto clusters, we need to setup a placement resource:

```
kubectl apply -f - <<EOF
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: http-gateway
  namespace: multi-cluster-gateways
spec:
  numberOfClusters: 1
  clusterSets:
    - gateway-clusters
EOF
```    

### Create the gateway class
 
```
kubectl create -f hack/ocm/gatewayclass.yaml
```

### Start the gateway controller

in the terminal with the hub cluster context and from the root of the repo run:

```
(export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build-controller install run-controller)

```

### Create a gateway

replace the some.domain.com with a subdomain of your root domain

```
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: prod-web
  namespace: multi-cluster-gateways
spec:
  gatewayClassName: mctc-gw-istio-external-instance-per-cluster
  listeners:
  - allowedRoutes:
      namespaces:
        from: All
    name: api
    hostname: test.cb.hcpapps.net
    port: 443
    protocol: HTTPS
EOF
```

### Place the gateway

On the gateway clusters, you should see there is still no gateways setup. This is because we haven't placed the gateway yet

To place the gateway, we need to add a placement label to gateway resource

```
k label gateway prod-web "cluster.open-cluster-management.io/placement"="http-gateway" -n multi-cluster-gateways

```

Now on each of the gateway clusters you should find there is a configured gateway

```
k get gateway -A
multi-cluster-gateways   prod-web   istio   172.32.200.0                17m
```

And the appropriate certificate secrets

```
 k get secrets -n multi-cluster-gateways
NAME                 TYPE                DATA   AGE
some.domain.com   kubernetes.io/tls   3      19m

```

So now our gateway has been placed and is running in the right locations with the right configuration and TLS has been setup for the HTTPS listeners.

So what about DNS how do we bring traffic to these gateways.


### Create and attach a HTTPRoute

using the following command in the hub cluster, you will see we currently have no DNSRecord resources 

```
k get dnsrecord -A
No resources found
```

DNSRecords are populated only when a HTTPRoute is attached to a gateway. To do this we will create a HTTPRoute in one of the gateway clusters.

```

kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: my-route
spec:
  parentRefs:
  - kind: Gateway
    name: prod-web
    namespace: kuadrant-multi-cluster-gateways
  hostnames:
  - "test.cb.hcpapps.net"  
  rules:
  - backendRefs:
    - name: echo
      port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: echo
spec:
  ports:
    - name: http-port
      port: 8080
      targetPort: http-port
      protocol: TCP
  selector:
    app: echo     
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo
  template:
    metadata:
      labels:
        app: echo
    spec:
      containers:
        - name: echo
          image: docker.io/jmalloc/echo-server
          ports:
            - name: http-port
              containerPort: 8080
              protocol: TCP       
EOF
```

Once this is done, the gateway controller will pick up that a HTTPRoute has been attached to the Gateway in a given cluster and it will setup a DNS record to start bringing traffic to that gateway.

You can see this in the hub cluster:

```
k get dnsrecord -A
NAMESPACE                                 NAME                 READY
multi-cluster-gateways   api.cb.hcpapps.net   True

```

you should also be able to see there is only 1 endpoint added which corresponds to cluster where the HTTPRoute was created.

```
kubectl get dnsrecord -n multi-cluster-gateways -o=yaml
```

If you want go ahead and create the HTTPRoute in the second gateway cluster. Looking at the DNSRecord afterwards you will it has now got two endpoints.

### adjusting the placement

In our example placement we picked two clusters. If you edit this and change it to 1 cluster you should see one of the gateways removed


## Clean Up

To clean up you can simply delete the gateway and associated DNSRecord (this will be automatic in the future)