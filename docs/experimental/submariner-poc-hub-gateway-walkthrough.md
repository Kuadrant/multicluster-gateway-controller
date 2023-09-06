# Submariner proof of concept with a Hub Gateway & 2 Workload Clusters

## Introduction

This walkthrough shows how submariner can be used to provide service resiliency across 2 clusters with a hub cluster as the Gateway.
The hub cluster is running a Gateway with a HttpRoute in front of an application Service.
By leveraging Submariner (and the Multi Cluster Services API), the application Service can be exported (via a ServiceExport resource) from the 2 workload clusters,
and imported (via a ServiceImport resource) to the hub cluster.
This provides a clusterset hostname for the service in the hub cluster e.g. echo.kuadrant-multi-cluster-gateways.svc.clusterset.local
The HttpRoute has a backendRef to a Service that points to this hostname.
If the Service is unavailable in either workload cluster, it will be routed to the other workload cluster.

<img src="../images/submariner/submariner-poc-hub-gateway-diagram.png" alt="architecture" width="600"/>

## Requirements

* Local development environment has been set up as per the main README i.e. local env files have been created with AWS credentials & a zone

>**Note:** :exclamation: this walkthrough will setup a zone in your AWS account and make changes to it for DNS purposes

>**Note:** :exclamation: `replace.this` is a placeholder that you will need to replace with your own domain

## Installation and Setup

For this walkthrough, we're going to use multiple terminal sessions/windows, all using `multicluster-gateway-controller` as the `pwd`.

Open three windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub Cluster)
* `T2` (Where we'll run our controller locally)
* `T3` (Workload cluster 1)
* `T4` (Workload cluster 2)

To setup a local instance with submariner, in `T1`, create kind clusters:

```bash
make local-setup-kind MGC_WORKLOAD_CLUSTERS_COUNT=2
```
And deploy onto the using:
```bash
make local-setup-mgc OCM_SINGLE=true SUBMARINER=true MGC_WORKLOAD_CLUSTERS_COUNT=2
```

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

Next, in `T1` we need to bind this cluster set to our multi-cluster-gateways namespace so that we can use that cluster to place Gateway on:

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

### Create a placement for our Gateway

In order to place our Gateway onto the hub clusters, we need to setup a placement resource. Again, in `T1`, run:

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

### Create the GatewayClass
 
Lastly, we will set up our multi-cluster GatewayClass. In `T1`, run:

```bash
kubectl create -f hack/ocm/gatewayclass.yaml
```

### Start the Gateway Controller

In `T2` run the following to start the Gateway Controller:

```bash
kind export kubeconfig --name=mgc-control-plane --kubeconfig=$(pwd)/local/kube/control-plane.yaml && export KUBECONFIG=$(pwd)/local/kube/control-plane.yaml
make build-controller install run-controller
```

### Create a Gateway

We know will create a multi-cluster Gateway definition in the hub cluster. In `T1`, run the following: 

**Important**: :exclamation: Make sure to replace `sub.replace.this` with a subdomain of your root domain.

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
    tls:
      mode: Terminate
      certificateRefs:
        - name: apps-hcpapps-tls
          kind: Secret
EOF
```

### Enable TLS

1. In `T1`, create a TLSPolicy and attach it to your Gateway:

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: kuadrant.io/v1alpha1
    kind: TLSPolicy
    metadata:
      name: prod-web
      namespace: multi-cluster-gateways
    spec:
      targetRef:
        name: prod-web
        group: gateway.networking.k8s.io
        kind: Gateway
      issuerRef:
        group: cert-manager.io
        kind: ClusterIssuer
        name: glbc-ca   
    EOF
    ```

1. You should now see a Certificate resource in the hub cluster. In `T1`, run:

    ```bash
    kubectl get certificates -A
    ```
   you'll see the following:

   ```
    NAMESPACE                NAME               READY   SECRET             AGE
    multi-cluster-gateways   apps-hcpapps-tls   True    apps-hcpapps-tls   12m
    ```

It is possible to also use a letsencrypt certificate, but for simplicity in this walkthrough we are using a self-signed cert.

### Place the gateway

To place the Gateway, we need to add a placement label to Gateway resource to instruct the Gateway controller where we want this Gateway instantiated. In `T1`, run:

```bash
kubectl label gateways.gateway.networking.k8s.io prod-web "cluster.open-cluster-management.io/placement"="http-gateway" -n multi-cluster-gateways
```

Now on the hub cluster you should find there is a configured Gateway and instantiated Gateway. In `T1`, run:

```bash
kubectl get gateways.gateway.networking.k8s.io -A
```

```
kuadrant-multi-cluster-gateways   prod-web   istio                                         172.31.200.0                29s
multi-cluster-gateways            prod-web   kuadrant-multi-cluster-gateway-instance-per-cluster                  True         2m42s
```

### Deploy the App to the 2 workload clusters

We do this before the HttpRoute is created for the purposes of the walkthrough.
If we don't do it in this order, there may be negative dns caching of the ServiceImport clusterset host resulting in 503 responses.
In `T3`, targeting the 1st workload cluster, go ahead and create Service and Deployment.

```bash
kind export kubeconfig --name=mgc-workload-1 --kubeconfig=$(pwd)/local/kube/workload1.yaml && export KUBECONFIG=$(pwd)/local/kube/workload1.yaml
kubectl create namespace kuadrant-multi-cluster-gateways
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: echo
  namespace: kuadrant-multi-cluster-gateways
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
  namespace: kuadrant-multi-cluster-gateways
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

In `T4`, targeting the 2nd workload cluster, go ahead and create Service and Deployment there too.

```bash
kind export kubeconfig --name=mgc-workload-2 --kubeconfig=$(pwd)/local/kube/workload2.yaml && export KUBECONFIG=$(pwd)/local/kube/workload2.yaml
kubectl create namespace kuadrant-multi-cluster-gateways
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: echo
  namespace: kuadrant-multi-cluster-gateways
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
  namespace: kuadrant-multi-cluster-gateways
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

### Create the ServiceExports and ServiceImports

In `T1`, export the Apps echo service from the workload clusters.

```bash
./bin/subctl export service --kubeconfig ./tmp/kubeconfigs/external/mgc-workload-1.kubeconfig --namespace kuadrant-multi-cluster-gateways echo
./bin/subctl export service --kubeconfig ./tmp/kubeconfigs/external/mgc-workload-2.kubeconfig --namespace kuadrant-multi-cluster-gateways echo
```

In `T1`, verify the ServiceExport was created on both clusters

```bash
kubectl --kubeconfig ./tmp/kubeconfigs/external/mgc-workload-1.kubeconfig get serviceexport echo -n kuadrant-multi-cluster-gateways
kubectl --kubeconfig ./tmp/kubeconfigs/external/mgc-workload-2.kubeconfig get serviceexport echo -n kuadrant-multi-cluster-gateways
```

In `T1`, verify the ServiceImport was created in the hub

```bash
kubectl --kubeconfig ./tmp/kubeconfigs/external/mgc-control-plane.kubeconfig get serviceimport echo -n kuadrant-multi-cluster-gateways
```

### Create and attach a HTTPRoute and Service

Let's create a HTTPRoute and a Service (that uses an externalName) in the hub cluster.
Remember to replace the hostnames. In `T1`, run:

```bash
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: my-route
  namespace: kuadrant-multi-cluster-gateways
spec:
  parentRefs:
  - kind: Gateway
    name: prod-web
    namespace: kuadrant-multi-cluster-gateways
  hostnames:
  - "sub.replace.this"  
  rules:
  - backendRefs:
    - name: echo-import-proxy
      port: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: echo-import-proxy
  namespace: kuadrant-multi-cluster-gateways
spec:
  type: ExternalName
  externalName: echo.kuadrant-multi-cluster-gateways.svc.clusterset.local
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
EOF
```

### Enable DNS

1. In `T1`, create a DNSPolicy and attach it to your Gateway:

```bash
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1alpha1
kind: DNSPolicy
metadata:
  name: prod-web
  namespace: multi-cluster-gateways
spec:
  targetRef:
    name: prod-web
    group: gateway.networking.k8s.io
    kind: Gateway     
EOF
```

Once this is done, the Kuadrant multi-cluster Gateway controller will pick up that a HTTPRoute has been attached to the Gateway it is managing from the hub and it will setup a DNS record to start bringing traffic to that Gateway for the host defined in that listener.

You should now see a DNSRecord and only 1 endpoint added which corresponds to address assigned to the Gateway where the HTTPRoute was created. In `T1`, run:

```bash
kubectl get dnsrecord -n multi-cluster-gateways -o=yaml
```

## Verify the HttpRoute works

At this point you should get a 200 response.
It might take a minute for dns to propagate internally by submariner after importing the services above.

```bash
curl -Ik https://sub.replace.this
```

If DNS is not resolving for you yet, you may get a 503.
In that case you can force resolve the IP to the hub cluster and verify a 200 is returned.

```bash
curl -Ik --resolve sub.replace.this:443:172.31.200.0 https://sub.replace.this
```

## Known issues

At the time of writing, Istio does *not* support adding a ServiceImport as a backendRef directly as per the [Gateway API proposal - GEP-1748](https://gateway-api.sigs.k8s.io/geps/gep-1748/#serviceimport-as-a-backend).
This is why the walkthrough uses a Service of type ExternalName to route to the clusterset host instead.
There is an [issue](https://github.com/istio/istio/issues/44415) questioning the current state of support.

The installation of the `subctl` cli [fails on macs with arm architecture](https://github.com/submariner-io/get.submariner.io/issues/50). The error is `curl: (22) The requested URL returned error: 404`. A workaround for this is to download the amd64 darwin release manually [from the releases page](https://github.com/submariner-io/subctl/releases) and extract it to the `./bin` directory.
