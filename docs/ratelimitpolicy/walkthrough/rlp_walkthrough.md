
# Rate Limit Policy Walkthrough

## Walkthrough Goals

* Define a Gateway in the control plane with a HTTPS listener, and have it instantiated in 2 workload clusters
* Deploy an Application with a HTTPRoute to the 2 workload clusters, and have the route attached to the Gateway listener
* Define a RateLimitPolicy in the control plane and target the Gateway, and have it instantiated in 2 workload clusters
* Adjust the Rate Limits for each cluster using the injected Rate Limit cluster attributes.
* Define custom attributes for each workload cluster and show how they are synced to each workload cluster and can be used to adjust Rate Limits.

## Environment Setup

### Control Plane Cluster

```bash
make local-setup MGC_WORKLOAD_CLUSTERS_COUNT=2
```

#### Gateway Setup

Export the hostname you want to use for you Application, and copy the example gateway.
For example, if you set your `ZONE_ROOT_DOMAIN` as `example.com`, you could set `MYAPP_HOST` to `myapp.example.com` below to use it as your Application host.

```bash
export MYAPP_HOST=myapp.mn.hcpapps.net
```

Deploy the gateway (Note: You need to change the gateway hostname as required)

```bash
kubectl apply -f docs/ratelimitpolicy/walkthrough/gatewayclass.yaml
kubectl create namespace mgc-tenant
kubectl apply -n mgc-tenant -f docs/ratelimitpolicy/walkthrough/gateway.yaml
```

#### Start Controller

```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-control-plane.kubeconfig
(export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build-controller install run-controller)
```
### Data Plane Cluster 1 (Workload 1)

#### Start Syncer

```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-workload-1.kubeconfig
METRICS_PORT=8086 HEALTH_PORT=8087 make build-syncer run-syncer
```

#### Deploy Application

Deploy the echo application (Note: You need to change the hostname as required)
```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-workload-1.kubeconfig
kubectl apply -f docs/ratelimitpolicy/walkthrough/echo-application.yaml -n mgc-downstream
```

Bump kuadrant so it picks up on gateways created after kuadrant was started (Known issue with kuadrant)

```bash
kubectl annotate kuadrant/mgc -n kuadrant-system updateme=`date +%s` --overwrite
kubectl get gateway example-gateway -n mgc-downstream -o json | jq .metadata.annotations
{
    "kuadrant.io/gateway-cluster-label-selector": "type=test",
    "kuadrant.io/namespace": "kuadrant-system"                                                                                                                                 
}
```

Tail app logs
```bash
kubectl logs -f deployments/echo -n mgc-downstream | xargs -IL date +"%H%M%S: L"
````

### Data Plane Cluster 2 (Workload 2)

#### Start Syncer

```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-workload-2.kubeconfig
METRICS_PORT=8088 HEALTH_PORT=8089 make build-syncer run-syncer
```

#### Deploy Application

Deploy the echo application (Note: You need to chnage the hostname as required)
```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-workload-2.kubeconfig
kubectl apply -f docs/ratelimitpolicy/walkthrough/echo-application.yaml -n mgc-downstream
```

Bump kuadrant so it picks up on gateways created after kuadrant was started (Known issue with kuadrant)

```bash
kubectl annotate kuadrant/mgc -n kuadrant-system updateme=`date +%s` --overwrite
kubectl get gateway example-gateway -n mgc-downstream -o json | jq .metadata.annotations
{
    "kuadrant.io/gateway-cluster-label-selector": "type=test",
    "kuadrant.io/namespace": "kuadrant-system"                                                                                                                                 
}
```

Tail app logs
```bash
kubectl logs -f deployments/echo -n mgc-downstream | xargs -IL date +"%H%M%S: L"
````


## Application Verification

Verify the Application host can be reached with curl. You should see 200 responses in the command output.

```bash
while true; do curl -k -s -o /dev/null -w "%{http_code}\n"  https://${MYAPP_HOST} && sleep 1; done
```

## Add a RateLimitPolicy in the control plane (Global limit set)

Create a rate limit policy (Note: You need to change the hostname as required).
```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-control-plane.kubeconfig
kubectl apply -f docs/ratelimitpolicy/walkthrough/ratelimitpolicy.yaml -n mgc-tenant
```

Verify the spec is unmodified.
```bash
kubectl get ratelimitpolicy echo-rlp -n mgc-tenant -o json | jq .spec
```

Verify the target Gateway is set as an owner.
```bash
kubectl get ratelimitpolicy echo-rlp -n mgc-tenant -o json | jq .metadata.ownerReferences
```

Verify sync and patch annotations are added for all clusters.
```bash
kubectl get ratelimitpolicy echo-rlp -n mgc-tenant -o json | jq .metadata.annotations
```

Check workload cluster RLP is synced correctly.
```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-workload-1.kubeconfig
kubectl get ratelimitpolicy echo-rlp -n mgc-downstream -o yaml
```

```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-workload-2.kubeconfig
kubectl get ratelimitpolicy echo-rlp -n mgc-downstream -o yaml
```

Observe the app is now being limited. Global limit set, every ~8th request, regardless of which cluster it hits should be rejected.

```bash
while true; do curl -k -s -o /dev/null -w "%{http_code}\n"  https://${MYAPP_HOST} && sleep 1; done
```

To ensure an even distribution of requests across both clusters the following can be used.

```bash
while true; do curl -k -s -o /dev/null -w "mgc-workload-1: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.200.0" | egrep --color "\b(429)\b|$" && sleep 1 && curl -k -s -o /dev/null -w "mgc-workload-2: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.201.0" | egrep --color "\b(429)\b|$" && sleep 1; done
```

## Add a limit using the cluster attribute

Update the rate limit policy (Note: You need to change the hostname as required).
```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-control-plane.kubeconfig
kubectl apply -f docs/ratelimitpolicy/walkthrough/ratelimitpolicy-cluster_attr_limit.yaml -n mgc-tenant
```

Observe the app is now being limited as expected. Requests going to `mgc-workload-1` should be limited to 1 every 10 seconds, `mgc-workload-2` should have no limits.

```bash
while true; do curl -k -s -o /dev/null -w "mgc-workload-1: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.200.0" | egrep --color "\b(429)\b|$" && sleep 1 && curl -k -s -o /dev/null -w "mgc-workload-2: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.201.0" | egrep --color "\b(429)\b|$" && sleep 1; done
```


## Add a limit using custom attributes

Add attributes to the cluster secrets:

```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-control-plane.kubeconfig
kubectl annotate --overwrite secret mgc-workload-1 "kuadrant.io/attribute-cloud=aws" -n argocd
kubectl annotate --overwrite secret mgc-workload-2 "kuadrant.io/attribute-cloud=gcp" -n argocd
```
`
Update the rate limit policy (Note: You need to change the hostname as required).
```bash
export KUBECONFIG=./tmp/kubeconfigs/mgc-control-plane.kubeconfig
kubectl apply -f docs/ratelimitpolicy/walkthrough/ratelimitpolicy-custom_attr_limit.yaml -n mgc-tenant
```

Observe the app is now being limited as expected. Requests going to `mgc-workload-1` (aws) should be limited to 2 every 10 seconds, `mgc-workload-2` (gcp) should be limited to 8 every 10 seconds.

```bash
while true; do curl -k -s -o /dev/null -w "mgc-workload-1: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.200.0" | egrep --color "\b(429)\b|$" && sleep 1 && curl -k -s -o /dev/null -w "mgc-workload-2: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.201.0" | egrep --color "\b(429)\b|$" && sleep 1; done
```

Check `mgc-workload-1` only, aws should be limited to 2 every 10 seconds,

```bash
while true; do curl -k -s -o /dev/null -w "mgc-workload-1: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.200.0" | egrep --color "\b(429)\b|$" && sleep 1; done
```

Check `mgc-workload-2` only, gcp should be limited to 8 every 10 seconds,

```bash
while true; do curl -k -s -o /dev/null -w "mgc-workload-2: %{http_code}\n" https://${MYAPP_HOST} --resolve "${MYAPP_HOST}:443:172.32.201.0" | egrep --color "\b(429)\b|$" && sleep 1; done
```
