
# External DNS POC Walkthrough

## Walkthrough Goals

* Demonstrate integration with [External DNS](https://github.com/kubernetes-sigs/external-dns) for DNSRecord reconciliation.

## Environment Setup

### Control Plane Cluster

```bash
make local-setup MCTC_WORKLOAD_CLUSTERS_COUNT=2
```

#### Gateway Setup

Export the hostname you want to use for you Application, and copy the example gateway.
For example, if you set your `ZONE_ROOT_DOMAIN` as `example.com`, you could set `MYAPP_HOST` to `myapp.example.com` below to use it as your Application host.

```bash
export MYAPP_HOST=myapp.gc.hcpapps.net
```

Create managed zones
Create a new managed zone
```bash
kubectl apply -f docs/external-dns/walkthrough/managedzone_gc.hcapps.net.yaml -n multi-cluster-traffic-controller-system
kubectl get managedzones -A
NAMESPACE                                 NAME     DOMAIN NAME      ID                    RECORD COUNT   NAMESERVERS                                                                                                                             READY
multi-cluster-traffic-controller-system   dev-mz   gc.hcpapps.net   8543679041015546449   -1             ["ns-cloud-c4.googledomains.com.","ns-cloud-c4.googledomains.com.","ns-cloud-c4.googledomains.com.","ns-cloud-c4.googledomains.com."]   True
````

Deploy the gateway (Note: You need to change the gateway hostname as required)

```bash
kubectl apply -f docs/external-dns/walkthrough/gatewayclass.yaml
kubectl create namespace mctc-tenant
kubectl apply -n mctc-tenant -f docs/external-dns/walkthrough/gateway.yaml
```

#### Start Controllers


Start mgc
```bash
export KUBECONFIG=./tmp/kubeconfigs/mctc-control-plane.kubeconfig
(export GOOGLE_APPLICATION_CREDENTIALS=/home/mnairn/go/src/github.com/kuadrant/multi-cluster-traffic-controller/config/external-dns/google/credentials.json && make build-controller install && ./bin/controller --dns-provider=google)
```

Start external-dns
```bash
GOOGLE_APPLICATION_CREDENTIALS=/home/mnairn/go/src/github.com/kuadrant/multi-cluster-traffic-controller/config/external-dns/google/credentials.json ./build/external-dns --source=crd --provider=google --google-project=it-cloud-gcp-rd-midd-san --crd-source-apiversion=kuadrant.io/v1alpha1 --crd-source-kind=DNSRecord --log-level=debug --events --interval 20s --txt-prefix=mctc- --registry=txt
```

### Data Plane Cluster 1 (Workload 1)

#### Start Syncer

```bash
export KUBECONFIG=./tmp/kubeconfigs/mctc-workload-1.kubeconfig
METRICS_PORT=8086 HEALTH_PORT=8087 make build-syncer run-syncer
```

#### Deploy Application

Deploy the echo application (Note: You need to change the hostname as required)
```bash
export KUBECONFIG=./tmp/kubeconfigs/mctc-workload-1.kubeconfig
kubectl apply -f docs/external-dns/walkthrough/echo-application.yaml -n mctc-downstream
```

Tail app logs
```bash
kubectl logs -f deployments/echo -n mctc-downstream | xargs -IL date +"%H%M%S: L"
````

### Data Plane Cluster 2 (Workload 2)

#### Start Syncer

```bash
export KUBECONFIG=./tmp/kubeconfigs/mctc-workload-2.kubeconfig
METRICS_PORT=8088 HEALTH_PORT=8089 make build-syncer run-syncer
```

#### Deploy Application

Deploy the echo application (Note: You need to chnage the hostname as required)
```bash
export KUBECONFIG=./tmp/kubeconfigs/mctc-workload-2.kubeconfig
kubectl apply -f docs/external-dns/walkthrough/echo-application.yaml -n mctc-downstream
```

Tail app logs
```bash
kubectl logs -f deployments/echo -n mctc-downstream | xargs -IL date +"%H%M%S: L"
````


## Application Verification

Verify the Application host can be reached with curl. You should see 200 responses in the command output.

```bash
while true; do curl -k -s -o /dev/null -w "%{http_code}\n"  https://${MYAPP_HOST} && sleep 1; done
```
