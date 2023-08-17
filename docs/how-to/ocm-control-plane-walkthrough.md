# Open Cluster Management and Multi-Cluster gateways

## Introduction
This document will walk you through using Open Cluster Management (OCM) and Kuadrant to configure and deploy a multi-cluster gateway. 
You will also deploy a simple application that uses that gateway for ingress and protects that applications endpoints with a rate limit policy. 
We will start with a single cluster and move to multiple clusters to illustrate how a single gateway definition can be used across multiple clusters and highlight the automatic TLS integration and also the automatic DNS load balancing between gateway instances.

## Requirements

The below binary dependencies can be installed using the `make dependencies` command if you've already cloned the repo. If not links are provided.

- [docker](https://docs.docker.com/engine/install/)
- [kind](https://kind.sigs.k8s.io/)
- [operator-sdk](https://sdk.operatorframework.io/docs/installation/)
- [yq](https://mikefarah.gitbook.io/yq/v/v3.x/)
- [clusteradm](https://github.com/open-cluster-management-io/clusteradm#install-the-clusteradm-command-line)
- [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
- [helm](https://helm.sh/docs/intro/install/)
- go >= 1.20
- openssl >= 3
- AWS account with Route 53 enabled
- https://github.com/chipmk/docker-mac-net-connect (for macos users)

>**Note:** :exclamation: this walkthrough will setup a zone in your AWS account and make changes to it for DNS purposes

## Installation and Setup
* Clone this repo locally 
* Set up your DNS Provider by following these [steps](providers/providers.md)

* We're going to use an environment variable, `MGC_SUB_DOMAIN`, throughout this walkthrough. Simply run the below in each window you create:

  For example:
  ```bash
  export MGC_SUB_DOMAIN=myapp.jbloggs.hcpapps.net
  ```

* Alternatively, to set a default, add the above environment variable to your `.zshrc` or `.bash_profile`. To override this as a once-off, simply `export MGC_SUB_DOMAIN`.

## Open terminal sessions

For this walkthrough, we're going to use multiple terminal sessions/windows.

Open two windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub Cluster)
* `T2` (Workloads cluster)

* NOTE: MCG_SUB_DOMAIN env var is required in both terminals

1. To setup a local instance, in `T1`, run:

    ```bash
    curl https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/quickstart-setup.sh | bash
    ```

    > :sos: Linux users may encounter the following error:
    > `ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
    > make: *** [Makefile:75: local-setup] Error 1ERROR: failed to create cluster: could not find a log line that matches "Reached target .*Multi-User System.*|detected cgroup v1"
    > make: *** [Makefile:75: local-setup] Error 1` 
    > This is a known issue with Kind. [Follow the steps here](https://kind.sigs.k8s.io/docs/user/known-issues/#pod-errors-due-to-too-many-open-files) to resolve it.

The script will
- install 2 kind clusters
- a set of components required for the walkthrough, e.g. ocm, istio, MGC itself.
- label the control plane managed cluster as an Ingress cluster
- create the ManagedClusterSet that uses the ingress label to select clusters
- bind this cluster set to our multi-cluster-gateways namespace so that we can use those clusters to place gateways on
- set up a placement resource, in order to place our gateways onto clusters
- lastly, it will set up a multi-cluster gateway class

1. Once this is completed your kubeconfig context should be set to the hub cluster. 

    > **Optional Step:** :thought_balloon: If you need to reset this run the following in `T1`:
    > ```bash
    > kind export kubeconfig --name=mgc-control-plane --kubeconfig=$(pwd)/local/kube/control-plane.yaml && export KUBECONFIG=$(pwd)/local/kube/control-plane.yaml
    > ```
   
### Create a gateway

#### Check the managed zone

1. First let's ensure the `managedzone` is present. In `T1`, run the following:

    ```bash
    kubectl get managedzone -n multi-cluster-gateways
    ```
1. You should see the following:
    ```
    NAME          DOMAIN NAME      ID                                  RECORD COUNT   NAMESERVERS                                                                                        READY
    mgc-dev-mz-aws   test.hcpapps.net   /hostedzone/Z08224701SVEG4XHW89W0   7              ["ns-1414.awsdns-48.org","ns-1623.awsdns-10.co.uk","ns-684.awsdns-21.net","ns-80.awsdns-10.com"]   True
    ```


You are now ready to begin creating a gateway! :tada:


1. We will now create a multi-cluster gateway definition in the hub cluster. In `T1`, run the following:

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

In the hub cluster there will be a single gateway definition but no actual gateway for handling traffic yet.

This is because we haven't placed the gateway yet onto any of our ingress clusters (in this case the hub and ingress cluster are the same)

1. To place the gateway, we need to add a placement label to gateway resource to instruct the gateway controller where we want this gateway instantiated. In `T1`, run:

    ```bash
    kubectl label gateway prod-web "cluster.open-cluster-management.io/placement"="http-gateway" -n multi-cluster-gateways
    ```

2. Now on the hub cluster you should find there is a configured gateway and instantiated gateway. In `T1`, run:

    ```bash
    kubectl get gateway -A
    ```
    you'll see the following:

    ```
    kuadrant-multi-cluster-gateways   prod-web   istio                                         172.31.200.0                29s
    multi-cluster-gateways            prod-web   kuadrant-multi-cluster-gateway-instance-per-cluster                  True         2m42s
    ```

    The instantiated gateway in this case is handled by Istio and has been assigned the 172.x address. You can define this gateway to be handled in the multi-cluster-gateways namespace. 
    As we are in a single cluster you can see both. Later on we will add in another ingress cluster and in that case you will only see the instantiated gateway.

    Additionally, you should be able to see a secret containing a self-signed certificate.

1. In `T1`, run:

    ```bash
    kubectl get secrets -n kuadrant-multi-cluster-gateways
    ```
    you'll see the following:
    ```
    NAME               TYPE                DATA   AGE
    apps-hcpapps-tls   kubernetes.io/tls   3      13m
    ```

The listener is configured to use this TLS secret also. So now our gateway has been placed and is running in the right locations with the right configuration and TLS has been setup for the HTTPS listeners.

So what about DNS how do we bring traffic to these gateways?


### Create and attach a HTTPRoute

1. In `T1`, using the following command in the hub cluster, you will see we currently have no DNSRecord resources.

    ```bash
    kubectl get dnsrecord -A
    ```
    ```
    No resources found
    ```

1. Let's create a simple echo app with a HTTPRoute in one of the gateway clusters. Remember to replace the hostnames. Again we are creating this in the single hub cluster for now. In `T1`, run:

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

   Once this is done, the Kuadrant multi-cluster gateway controller will pick up that a HTTPRoute has been attached to the Gateway it is managing from the hub and it will setup a DNS record to start bringing traffic to that gateway for the host defined in that listener.

1. You should now see a DNSRecord resource in the hub cluster. In `T1`, run:

    ```bash
    kubectl get dnsrecord -A
    ```
    ```
    NAMESPACE                NAME                 READY
    multi-cluster-gateways   myapp.test.hcpapps.net   True
    ```

1. You should also be able to see there is only 1 endpoint added which corresponds to address assigned to the gateway where the HTTPRoute was created. In `T1`, run:

    ```bash
    kubectl get dnsrecord -n multi-cluster-gateways -o=yaml
    ```

1. Give DNS a minute or two to update. You should then be able to execute the following and get back the correct A record. 

    ```bash
    dig $MGC_SUB_DOMAIN
    ```
1. You should also be able to curl that endpoint

    ```bash
    curl -k https://$MGC_SUB_DOMAIN

    # Request served by echo-XXX-XXX
    ```

## Introducing the second cluster

So now we have a working gateway with DNS and TLS configured. Let place this gateway on a second cluster and bring traffic to that gateway also.

1. First add the second cluster to the clusterset, by running the following in `T1`:

    ```bash
    kubectl label managedcluster kind-mgc-workload-1 ingress-cluster=true
    ```

1. This has added our workload-1 cluster to the ingress clusterset. Next we need to modify our placement to update our `numberOfClusters` to 2. To patch, in `T1`, run:

    ```bash
    kubectl patch placement http-gateway -n multi-cluster-gateways --type='json' -p='[{"op": "replace", "path": "/spec/numberOfClusters", "value": 2}]'
    ```

1. In `T2` window execute the following to see the gateway on the workload-1 cluster:

    ```bash
    kind export kubeconfig --name=mgc-workload-1 --kubeconfig=$(pwd)/local/kube/workload1.yaml && export KUBECONFIG=$(pwd)/local/kube/workload1.yaml
    kubectl get gateways -A
    ```
    You'll see the following
    ```
    NAMESPACE                         NAME       CLASS   ADDRESS        PROGRAMMED   AGE
    kuadrant-multi-cluster-gateways   prod-web   istio   172.31.201.0                90s
    ```

    So now we have second ingress cluster configured with the same Gateway. 

1. In `T2`, targeting the second cluster, go ahead and create the HTTPRoute in the second gateway cluster.

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

1. Now if you move back to the hub context in `T1` and take a look at the dnsrecord, you will see we now have two A records configured:


  ```bash
  kubectl get dnsrecord -n multi-cluster-gateways -o=yaml
  ```

## Watching DNS changes
If you want you can use ```watch dig $MGC_SUB_DOMAIN``` to see the DNS switching between the two addresses

## Follow on Walkthroughs
Some good follow on walkthroughs that build on this walkthrough

* [Installing the Kuadrant operator via OCM Addon](https://github.com/Kuadrant/multicluster-gateway-controller/blob/main/docs/how-to/kuadrant-addon-walkthrough.md)
* [Deploying/Configuring Redis, Limitador and Rate limit policies.](https://github.com/Kuadrant/multicluster-gateway-controller/blob/main/docs/how-to/ratelimiting-shared-redis.md)
