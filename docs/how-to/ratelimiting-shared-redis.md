# Deploying/Configuring Redis, Limitador and Rate limit policies.

## Introduction
The following document is going to show you how to deploy Redis as storage for Limitador, configure Limitador itself and how to configure and setup Rate Limit Policies against a `HTTP route` using Limitador .

## Requirements
* Kind
* Kuadrant operator [Walkthrough to install Kuadrant can be found here](https://github.com/Kuadrant/multicluster-gateway-controller/docs/how-to's/kuadrant-addon-walkthrough.md)
* Gateways setup [Walkthrough to setup gateways in you clusters can be found here](https://github.com/Kuadrant/multicluster-gateway-controller/docs/how-to's/ocm-control-plane-walkthrough.md)


 ## Installation and Setup
1. Clone this repo locally 
2. Run through the walkthroughs from requirements

## Open terminal sessions
For this walkthrough, we're going to be continuing on from a previous walkthrough that uses the following multiple terminal sessions/windows, all using `multicluster-gateway-controller` as the `pwd`.

Open three windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub/Spoke Cluster)
* `T2` (Hub Cluster Where we'll run our controller locally (needed for previous walkthrough))
* `T3` (Workloads cluster)

## Configuring limitador in spoke clusters
1. In `T1` get the ip address of your control-plane cluster using:
    ``` bash
    kubectl get nodes -o wide
    ```
1. If needs be update the URL located in `config/kuadrant/redis/limitador` to include the ip address from above step.
1. In the clusters that have Kuadrant operator installed i.e `T1 & T3` run the following to configure limitador to use Redis as storage rather then local cluster storage:
    ```bash
    kustomize build config/kuadrant/limitador/ | kubectl apply -f -
    ```
## Configuring Rate Limit Policies
1. In `T1 & T3 both spoke clusters` run the following command to create a Rate Limit Policy for the HTTP route created in the walkthrough linked above called `Open Cluster Management and Multi-Cluster gateways`. The policy is limiting the route to have 8 successful requests in 10 seconds, these values can be changed to whatever you want.

    ```bash
    kubectl apply -f - <<EOF
    apiVersion: kuadrant.io/v1beta1
    kind: RateLimitPolicy
    metadata:
    name: echo-rlp
    spec:
    targetRef:
        group: gateway.networking.k8s.io
        kind: HTTPRoute
        name: prod-web
    rateLimits:
        - configurations:
        - actions:
            - generic_key:
                descriptor_key: "limited"
                descriptor_value: "1"
        - rules:
        - hosts: [ "replace.this" ]
        limits:
            - conditions:
                - 'limited == "1"'
            maxValue: 8
            seconds: 10
    EOF
    ```
1.  In `T1 and T3` test the RLP you can run the following command:
    ```bash
    while true; do curl -k -s -o /dev/null -w "%{http_code}\n"  replace.this.with.host && sleep 1; done
    ```
2. You should see your host be limited to whatever limit you've chosen. This will be across **all** clusters. Meaning if you trying make a curl request to both clusters at the same time, it will maintain the limit and wont reset allowoing successful requests when it should be limited.

