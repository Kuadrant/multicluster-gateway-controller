# Open Cluster Management and Multi-Cluster gateways


This document will walk you through using Open Cluster Management (OCM) and Kuadrant to configure and deploy a multi-cluster gateway. 
You will also deploy a simple application that uses that gateway for ingress and protects that applications endpoints with a rate limit policy. 
We will start with a single cluster and move to multiple clusters to illustrate how a single gateway definition can be used across multiple clusters and highlight the automatic TLS integration and also the automatic DNS load balancing between gateway instances.


## Requirements

- [Kind](https://kind.sigs.k8s.io/)
- go >= 1.20
- openssl >= 3
- AWS account with Route 53 enabled
- https://github.com/chipmk/docker-mac-net-connect (for macos users)

**Note:** this walkthrough will setup a zone in your AWS account and make changes to it for DNS purposes

**Note:** `replace.this` is a place holder that you will need to replace with your own domain

## Installation and Setup
- Clone this repo locally 
- Setup a `./controller-config.env` file in the root of the repo with the following key values

```bash
# this sets up your default managed zone
AWS_DNS_PUBLIC_ZONE_ID=<AWS ZONE ID>
# this is the domain at the root of your zone (foo.example.com)
ZONE_ROOT_DOMAIN=<replace.this>
LOG_LEVEL=1
```   

- setup a `./aws-credentials.env` with credentials to access route 53

For example:

```bash
AWS_ACCESS_KEY_ID=<access_key_id>
AWS_SECRET_ACCESS_KEY=<secret_access_key>
AWS_REGION=eu-west-1
```

## Open terminal sessions
For this walkthrough, we're going to use multiple terminal sessions/windows, all using `multicluster-gateway-controller` as the `pwd`.

Open three windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub Cluster)
* `T2` (Where we'll run our controller locally)
* `T3` (Workloads cluster)

To setup a local instance, in `T1`, run:

```bash
make local-setup OCM_SINGLE=true MGC_WORKLOAD_CLUSTERS_COUNT=1
```

> Note: Linux users may encounter the following error:
> `ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
> make: *** [Makefile:75: local-setup] Error 1ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
> make: *** [Makefile:75: local-setup] Error 1` 
> This is a known issue with Kind. [Follow the steps here](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files) to resolve it.

Once this is completed your kubeconfig context should be set to the hub cluster. 

> **Optional Step:** If you need to reset this run the following in `T1`:
> ```bash
> kind export kubeconfig --name=mgc-control-plane --kubeconfig=$(pwd)/local/kube/control-plane.yaml && export KUBECONFIG=$(pwd)/local/kube/control-plane.yaml
> ```

In the hub cluster (`T1`) we are going to label the control plane managed cluster as an Ingress cluster:

```bash
kubectl label managedcluster kind-mgc-control-plane ingress-cluster=true
```

Next, in `T1`, create the ManagedClusterSet that uses the ingress label to select clusters:

```bash
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

Next, in `T1` we need to bind this cluster set to our multi-cluster-gateways namespace so that we can use those clusters to place gateways on:

```bash
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

In order to place our gateways onto clusters, we need to setup a placement resource. Again, in `T1`, run:

```bash
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
 
Lastly, we will set up our multi-cluster gateway class. In `T1`, run:

```bash
kubectl create -f hack/ocm/gatewayclass.yaml
```

### Start the Gateway Controller

In `T2` run the following to start the Gateway Controller:

```bash
(export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build-controller install run-controller)
```

### Check the managed zone

Lets ensure our `managedzone` is present. In `T1`, run the following:

```bash
export KUBECONFIG=$(pwd)/local/kube/control-plane.yaml
kubectl get managedzone -n multi-cluster-gateways
```
```
NAME          DOMAIN NAME      ID                                  RECORD COUNT   NAMESERVERS                                                                                        READY
mgc-dev-mz   replace.this   /hostedzone/Z08224701SVEG4XHW89W0   7              ["ns-1414.awsdns-48.org","ns-1623.awsdns-10.co.uk","ns-684.awsdns-21.net","ns-80.awsdns-10.com"]   True
```

### Create a gateway

We know will create a multi-cluster gateway definition in the hub cluster. In `T1`, run the following: 

**Important**: Make sure to replace `sub.replace.this` with a subdomain of your root domain.

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: prod-web
  namespace: multi-cluster-gateways
spec:
  gatewayClassName: kuadrant-multi-cluster-gateway-instance-per-cluster
  listeners:
  - allowedRoutes:
      namespaces:
        from: All
    name: api
    hostname: sub.replace.this
    port: 443
    protocol: HTTPS
EOF
```

### Place the gateway

In the hub cluster there will be a single gateway definition but no actual gateway for handling traffic yet.

This is because we haven't placed the gateway yet onto any of our ingress clusters (in this case the hub and ingress cluster are the same)

To place the gateway, we need to add a placement label to gateway resource to instruct the gateway controller where we want this gateway instantiated. In `T1`, run:

```bash
kubectl label gateway prod-web "cluster.open-cluster-management.io/placement"="http-gateway" -n multi-cluster-gateways

```

Now on the hub cluster you should find there is a configured gateway and instantiated gateway. In `T1`, run:

```bash
kubectl get gateway -A
```
```
kuadrant-multi-cluster-gateways   prod-web   istio                                         172.32.200.0                29s
multi-cluster-gateways            prod-web   kuadrant-multi-cluster-gateway-instance-per-cluster                  True         2m42s
```

The instantiated gateway in this case is handled by Istio and has been assigned the 172.x address. You can definition of this gateway is handled in multi-cluster-gateways namespace. 
As we are in a single cluster you can see both. Later on we will add in another ingress cluster and in that case you will only see the instantiated gateway.


Additionally you should be able to see a secret containing a self signed certificate. It is possible to also use a letsencrypt certificate, but for simplicity in this walkthrough we are using a self signed cert. In `T1`, run:

```bash
kubectl get secrets -n kuadrant-multi-cluster-gateways
```
```
sub.replace.this   kubernetes.io/tls   3      4m33s
```

The listener is configured to use this TLS secret also. So now our gateway has been placed and is running in the right locations with the right configuration and TLS has been setup for the HTTPS listeners.

So what about DNS how do we bring traffic to these gateways?


### Create and attach a HTTPRoute

We only configure DNS once a HTTPRoute has been attached to a listener in the gateway. In `T1`, using the following command in the hub cluster, you will see we currently have no DNSRecord resources.

```bash
kubectl get dnsrecord -A
```
```
No resources found
```

Lets create a simple echo app with a HTTPRoute in one of the gateway clusters. Remember to replace the hostnames. Again we are creating this in the single hub cluster for now. In `T1`, run:

```bash
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
  - "sub.replace.this"  
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

Once this is done, the Kuadrant multi-cluster gateway controller will pick up that a HTTPRoute has been attached to the Gateway it is managing from the hub and it will setup a DNS record to start bringing traffic to that gateway for the host defined in that listener.

You should now see a DNSRecord resource in the hub cluster. In `T1`, run:

```bash
kubectl get dnsrecord -A
```
```
NAMESPACE                NAME                 READY
multi-cluster-gateways   sub.replace.this   True
```

You should also be able to see there is only 1 endpoint added which corresponds to address assigned to the gateway where the HTTPRoute was created. In `T1`, run:

```bash
kubectl get dnsrecord -n multi-cluster-gateways -o=yaml
```

Give DNS a minute or two to update. You should then be able to execute

```bash
dig sub.replace.this 
```
and get back the correct A record. You should also be able to curl that endpoint

```bash
curl -k https://sub.replace.this

# Request served by echo-XXX-XXX
```

### Introducing the second cluster

So now we have a working gateway with DNS and TLS configured. Let place this gateway on a second cluster and bring traffic to that gateway also.

First add the second cluster to the clusterset, by running the following in `T1`:

```bash
kubectl label managedcluster kind-mgc-workload-1 ingress-cluster=true
```

This has added our workload-1 cluster to the ingress clusterset. Next we need to modify our placement. In `T1`, run:

```bash
kubectl edit placement http-gateway -n multi-cluster-gateways
```

This will open your `$EDITOR` - in here, edit the spec change the `numberOfClusters` to be 2, and save.

In `T3` window execute the following to see the gateway on the workload-1 cluster:

```bash
kind export kubeconfig --name=mgc-workload-1 --kubeconfig=$(pwd)/local/kube/workload1.yaml && export KUBECONFIG=$(pwd)/local/kube/workload1.yaml
kubectl get gateways -A
```
```
NAMESPACE                         NAME       CLASS   ADDRESS        PROGRAMMED   AGE
kuadrant-multi-cluster-gateways   prod-web   istio   172.32.201.0                90s
```

So now we have second ingress cluster configured with the same Gateway. 

In `T3`, targeting the second cluster, go ahead and create the HTTPRoute in the second gateway cluster.

```bash
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
  - "sub.replace.this"  
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

Now if you move back to the hub context in `T1` and take a look at the dnsrecord, you will see we now have two A records configured:


```bash
kubectl get dnsrecord -n multi-cluster-gateways -o=yaml
```

If you want you can use ```watch dig sub.replace.this ``` to see the DNS switching between the two addresses 
