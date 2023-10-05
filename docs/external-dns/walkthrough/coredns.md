# CoreDNS

```bash
kubectl delete managedzone -A --all
kubectl delete dnsrecord -A --all
kubectl delete gateway -A --all
```

```bash
./bin/kustomize build config/external-dns/coredns/etcd --enable-helm --helm-command ./bin/helm --load-restrictor LoadRestrictionsNone | kubectl apply -f -
```

```bash
kubectl get svc mctc-etcd -n default
NAME        TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
mctc-etcd   ClusterIP   10.96.165.158   <none>        2379/TCP,2380/TCP   20s
```

```bash
./bin/kustomize build config/external-dns/coredns --enable-helm --helm-command ./bin/helm --load-restrictor LoadRestrictionsNone | kubectl apply -f -
```

Get node IP:
```bash
nodeIP=$(kubectl get nodes -o json | jq -r ".items[] | select(.metadata.name == \"mctc-control-plane-control-plane\").status | .addresses[] | select(.type == \"InternalIP\").address")
echo $nodeIP
172.32.0.2
```

Get Service node port:
```bash
kubectl get svc mctc-coredns -n external-dns
NAME           TYPE       CLUSTER-IP     EXTERNAL-IP   PORT(S)                     AGE
mctc-coredns   NodePort   10.96.162.64   <none>        53:30130/UDP,53:30130/TCP   6m12s
```

Start Controller:
```bash
make build-controller install && ./bin/controller --dns-provider=coredns
```

Create a managed zone (coredns.hcpapps.local)
```bash
kubectl apply -f docs/external-dns/walkthrough/managedzone_coredns.hcpapps.local.yaml -n multi-cluster-traffic-controller-system
kubectl get managedzones -n multi-cluster-traffic-controller-system
NAME             DOMAIN NAME             ID    RECORD COUNT   NAMESERVERS   READY
coredns-dev-mz   coredns.hcpapps.local
````

Setup GatewayClass and Gateway (mmyapp.coredns.hcpapps.local)

```bash
kubectl apply -f docs/external-dns/walkthrough/gatewayclass.yaml
kubectl create namespace mctc-tenant
kubectl apply -n mctc-tenant -f docs/external-dns/walkthrough/gateway_myapp.coredns.hcpapps.local.yaml
```

Create a record and check it works:
```bash
kubectl apply -f docs/external-dns/walkthrough/dnsrecord_myapp.coredns.hcpapps.local.yaml 
dig @172.32.0.2 -p 31541 myapp.coredns.hcpapps.local +short
3.3.3.3
2.2.2.2
1.1.1.1
```
