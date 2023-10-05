# Google

```bash
kubectl delete managedzone -A --all
kubectl delete dnsrecord -A --all
kubectl delete gateway -A --all
```

```bash
./bin/kustomize build config/external-dns/google --enable-helm --helm-command ./bin/helm --load-restrictor LoadRestrictionsNone | kubectl apply -f -
```

Start Controller:
```bash
(export GOOGLE_APPLICATION_CREDENTIALS=/home/mnairn/go/src/github.com/kuadrant/multi-cluster-traffic-controller/config/external-dns/google/credentials.json && make build-controller install && ./bin/controller --dns-provider=google)
```

Create a managed zone (google.hcpapps.local)
```bash
kubectl apply -f docs/external-dns/walkthrough/managedzone_google.hcpapps.net.yaml -n multi-cluster-traffic-controller-system
kubectl get managedzones -n multi-cluster-traffic-controller-system
NAME             DOMAIN NAME             ID    RECORD COUNT   NAMESERVERS   READY
coredns-dev-mz   coredns.hcpapps.local
````

Setup GatewayClass and Gateway (mmyapp.google.hcpapps.local)

```bash
kubectl apply -f docs/external-dns/walkthrough/gatewayclass.yaml
kubectl create namespace mctc-tenant
kubectl apply -n mctc-tenant -f docs/external-dns/walkthrough/gateway_myapp.google.hcpapps.local.yaml
```

Create a record and check it works:
```bash
kubectl apply -f docs/external-dns/walkthrough/dnsrecord_myapp.google.hcpapps.net.yaml 
dig myapp.google.hcpapps.net +short
3.3.3.3
2.2.2.2
1.1.1.1
```
