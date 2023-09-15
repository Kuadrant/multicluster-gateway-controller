# Multicluster Gateways Walkthrough

## Introduction
This document will walk you through using Open Cluster Management (OCM) and Kuadrant to configure and deploy a multi-cluster gateway. 

You will also deploy a simple application that uses that gateway for ingress and protects that applications endpoints with a rate limit policy.

We will start with a single cluster and move to multiple clusters to illustrate how a single gateway definition can be used across multiple clusters and highlight the automatic TLS integration and also the automatic DNS load balancing between gateway instances.

## Requirements

- Complete the [Getting Started Guide](https://docs.kuadrant.io/getting-started/).

## Initial Setup

In this walkthrough, we'll deploy test echo services across multiple clusters. Before starting, set up a `MGC_SUB_DOMAIN` variable to simplify templating. This variable will represent the host for each service.

If you followed the [Getting Started Guide](https://docs.kuadrant.io/getting-started/), you would have already set up a `MGC_ZONE_ROOT_DOMAIN` environment variable when creating a `ManagedZone`. For this tutorial, we'll derive a host from this domain.

```bash
export MGC_SUB_DOMAIN=myapp.$MGC_ZONE_ROOT_DOMAIN
```

### Create a gateway

#### Check the managed zone

1. First let's ensure the `managedzone` is present:

    ```bash
    kubectl get managedzone -n multi-cluster-gateways --context kind-mgc-control-plane
    ```
    You should see the following:
    ```
    NAME          DOMAIN NAME      ID                                  RECORD COUNT   NAMESERVERS                                                                                        READY
    mgc-dev-mz   test.hcpapps.net   /hostedzone/Z08224701SVEG4XHW89W0   7              ["ns-1414.awsdns-48.org","ns-1623.awsdns-10.co.uk","ns-684.awsdns-21.net","ns-80.awsdns-10.com"]   True
    ```


You are now ready to begin creating a gateway! :tada:


1. We will now create a multi-cluster gateway definition in the hub cluster:

  ```bash
  kubectl --context kind-mgc-control-plane apply -f - <<EOF
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
      hostname: $MGC_SUB_DOMAIN
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

1. Create a TLSPolicy and attach it to your Gateway:

    ```bash
    kubectl --context kind-mgc-control-plane apply -f - <<EOF
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

2. You should now see a Certificate resource in the hub cluster:

    ```bash
    kubectl --context kind-mgc-control-plane get certificates -A
    ```
    You should see the following:
    
    ```
    NAMESPACE                NAME               READY   SECRET             AGE
    multi-cluster-gateways   apps-hcpapps-tls   True    apps-hcpapps-tls   12m
    ```

It is possible to also use a letsencrypt certificate, but for simplicity in this walkthrough we are using a self-signed cert.

### Place the gateway

In the hub cluster there will be a single gateway definition but no actual gateway for handling traffic yet.

This is because we haven't placed the gateway yet onto any of our ingress clusters (in this case the hub and ingress cluster are the same)

1. To place the gateway, we need to add a placement label to gateway resource to instruct the gateway controller where we want this gateway instantiated:

    ```bash
    kubectl --context kind-mgc-control-plane label gateway prod-web "cluster.open-cluster-management.io/placement"="http-gateway" -n multi-cluster-gateways
    ```

2. Now on the hub cluster you should find there is a configured gateway and instantiated gateway:

    ```bash
    kubectl --context kind-mgc-control-plane get gateway -A
    ```
    you'll see the following:

    ```
    kuadrant-multi-cluster-gateways   prod-web   istio                                         172.31.200.0                29s
    multi-cluster-gateways            prod-web   kuadrant-multi-cluster-gateway-instance-per-cluster                  True         2m42s
    ```

    The instantiated gateway in this case is handled by Istio and has been assigned the 172.x address. You can define this gateway to be handled in the multi-cluster-gateways namespace. 
    As we are in a single cluster you can see both. Later on we will add in another ingress cluster and in that case you will only see the instantiated gateway.

    Additionally, you should be able to see a secret containing a self-signed certificate.

3. There should also be an associated TLS secret:

    ```bash
    kubectl --context kind-mgc-control-plane get secrets -n kuadrant-multi-cluster-gateways
    ```
    you'll see the following:
    ```
    NAME               TYPE                DATA   AGE
    apps-hcpapps-tls   kubernetes.io/tls   3      13m
    ```

The listener is configured to use this TLS secret also. So now our gateway has been placed and is running in the right locations with the right configuration and TLS has been setup for the HTTPS listeners.

So what about DNS how do we bring traffic to these gateways?


### Create and attach a HTTPRoute

1. In the hub cluster, you will see we currently have no DNSRecord resources.

    ```bash
    kubectl --context kind-mgc-control-plane get dnsrecord -A
    ```
    ```
    No resources found
    ```

2. Let's create a simple echo app with a HTTPRoute in one of the gateway clusters. Remember to replace the hostnames. Again we are creating this in the single hub cluster for now:

    ```bash
    kubectl --context kind-mgc-control-plane apply -f - <<EOF
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
      - "$MGC_SUB_DOMAIN"  
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

### Enable DNS

1. Create a DNSPolicy and attach it to your Gateway:

    ```bash
    kubectl --context kind-mgc-control-plane apply -f - <<EOF
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

   Once this is done, the Kuadrant multi-cluster gateway controller will pick up that a HTTPRoute has been attached to the Gateway it is managing from the hub and it will setup a DNS record to start bringing traffic to that gateway for the host defined in that listener.

2. You should now see a DNSRecord resource in the hub cluster:

    ```bash
    kubectl --context kind-mgc-control-plane get dnsrecord -A
    ```
    ```
    NAMESPACE                NAME                 READY
    multi-cluster-gateways   prod-web-api         True
    ```

3. You should also be able to see there is only 1 endpoint added which corresponds to address assigned to the gateway where the HTTPRoute was created:

    ```bash
    kubectl --context kind-mgc-control-plane get dnsrecord -n multi-cluster-gateways -o=yaml
    ```

4. Give DNS a minute or two to update. You should then be able to execute the following and get back the correct A record. 

    ```bash
    dig $MGC_SUB_DOMAIN
    ```
5. You should also be able to curl that endpoint

    ```bash
    curl -k https://$MGC_SUB_DOMAIN

    # Request served by echo-XXX-XXX
    ```

## Introducing the second cluster

So now we have a working gateway with DNS and TLS configured. Let place this gateway on a second cluster and bring traffic to that gateway also.

1. First add the second cluster to the clusterset, by running the following:

    ```bash
    kubectl --context kind-mgc-control-plane label managedcluster kind-mgc-workload-1 ingress-cluster=true
    ```

2. This has added our workload-1 cluster to the ingress clusterset. Next we need to modify our placement to update our `numberOfClusters` to 2. To patch, run:

    ```bash
    kubectl --context kind-mgc-control-plane patch placement http-gateway -n multi-cluster-gateways --type='json' -p='[{"op": "replace", "path": "/spec/numberOfClusters", "value": 2}]'
    ```

3. Run the following to see the gateway on the workload-1 cluster:

    ```bash
    kubectl --context kind-mgc-workload-1 get gateways -A
    ```
    You'll see the following
    ```
    NAMESPACE                         NAME       CLASS   ADDRESS        PROGRAMMED   AGE
    kuadrant-multi-cluster-gateways   prod-web   istio   172.31.201.0                90s
    ```

    So now we have second ingress cluster configured with the same Gateway. 

4. Let's create the HTTPRoute in the second gateway cluster:

    >:exclamation: **Note:** Ensure the `MGC_SUB_DOMAIN` environment variable has been exported in this terminal session before applying this config.
   
    ```bash
    kubectl --context kind-mgc-workload-1 apply -f - <<EOF
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
      - "$MGC_SUB_DOMAIN"  
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

5. If we take a look at the dnsrecord, you will see we now have two A records configured:


  ```bash
  kubectl --context kind-mgc-control-plane get dnsrecord -n multi-cluster-gateways -o=yaml
  ```

## Watching DNS changes
If you want you can use ```watch dig $MGC_SUB_DOMAIN``` to see the DNS switching between the two addresses

## Follow-on Walkthroughs
Some good follow-on walkthroughs that build on this walkthrough

* [Deploying/Configuring Metrics.](../how-to/metrics-walkthrough.md)