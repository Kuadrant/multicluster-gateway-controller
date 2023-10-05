# External DNS

## AWS Route53

## Google Cloud DNS

## Azure DNS

## CoreDNS

Uses the [etcd plugin] (https://coredns.io/plugins/etcd/)

### ETCD
```bash
./bin/kustomize build config/external-dns/coredns/etcd --enable-helm --helm-command ./bin/helm --load-restrictor LoadRestrictionsNone | kubectl apply -f -
```

```bash
kubectl logs -f statefulsets/mctc-etcd -n default
```

```bash
kubectl get svc -A
NAMESPACE     NAME                 TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)                  AGE
default       kubernetes           ClusterIP   10.96.0.1      <none>        443/TCP                  4m22s
default       mctc-etcd            ClusterIP   10.96.127.47   <none>        2379/TCP,2380/TCP        2m24s
default       mctc-etcd-headless   ClusterIP   None           <none>        2379/TCP,2380/TCP        2m24s
kube-system   kube-dns             ClusterIP   10.96.0.10     <none>        53/UDP,53/TCP,9153/TCP   4m20s
```

```bash
kubectl port-forward -n default svc/mgc-etcd 2379:2379&        
[1] 281331                                                                                                                                                                                                                                    
Forwarding from 127.0.0.1:2379 -> 2379                                                                                                                                    
Forwarding from [::1]:2379 -> 2379
```


```bash
etcdctl --write-out=table --endpoints=http://127.0.0.1:2379 endpoint status
Handling connection for 2379
Handling connection for 2379
+-----------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
|       ENDPOINT        |        ID        | VERSION | DB SIZE | IS LEADER | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS |
+-----------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
| http://127.0.0.1:2379 | f05c50700a1ab8f1 |   3.5.9 |   20 kB |      true |      false |         3 |         32 |                 32 |        |
+-----------------------+------------------+---------+---------+-----------+------------+-----------+------------+--------------------+--------+
```

### CoreDNS

```bash
 ./bin/kustomize build config/external-dns/coredns --enable-helm --helm-command ./bin/helm --load-restrictor LoadRestrictionsNone | kubectl apply -f -
```

```bash
kubectl logs -f deployments/mgc-coredns -n coredns
```

```bash
kubectl get svc -n coredns
NAME           TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)                     AGE
mctc-coredns   NodePort   10.96.197.211   <none>        53:31541/UDP,53:31825/TCP   21m
```

```bash
nodeIP=$(kubectl get nodes -o json | jq -r ".items[] | select(.metadata.name == \"mgc-control-plane-control-plane\").status | .addresses[] | select(.type == \"InternalIP\").address")
echo $nodeIP
172.32.0.2
```

```bash
etcdctl put /skydns/local/hcpapps/coredns/myapp '{"host":"1.1.1.1","ttl":60}'
Handling connection for 2379
OK
E0518 11:43:47.032989  281331 portforward.go:391] error copying from local connection to remote stream: read tcp4 127.0.0.1:2379->127.0.0.1:58888: read: connection reset by peer
```

```bash
$ etcdctl get /skydns/local/hcpapps/coredns --prefix
Handling connection for 2379
/skydns/local/hcpapps/coredns/myapp/2aeea441
{"host":"1.1.1.1","ttl":20,"targetstrip":1}
E0522 12:31:40.123374  278088 portforward.go:391] error copying from local connection to remote stream: read tcp4 127.0.0.1:2379->127.0.0.1:48470: read: connection reset by peer

```


// the port here is the port 53 forwarded port for the mgc-coredns service
```bash
dig @172.32.0.2 -p 31541 google.com +short
209.85.202.139
209.85.202.102
209.85.202.113
209.85.202.138
209.85.202.100
209.85.202.101
dig @172.32.0.2 -p 31541 myapp.coredns.hcpapps.local +short
1.1.1.1
```

### External DNS

Update `etcdEndpoints`:
```yaml
valuesInline:
  coredns:
    etcdEndpoints: "http://10.96.127.47:2379"
```


```bash
 ./bin/kustomize build config/external-dns/coredns --enable-helm --helm-command ./bin/helm --load-restrictor LoadRestrictionsNone | kubectl apply -f -
```

```bash
kubectl logs -f deployments/mctc-external-dns -n external-dns
```

```bash
etcdctl del /skydns/local/hcpapps/coredns/myapp
Handling connection for 2379
1
E0518 11:56:04.681820  281331 portforward.go:391] error copying from local connection to remote stream: read tcp4 127.0.0.1:2379->127.0.0.1:48934: read: connection reset by peer
```

```bash
kubectl apply -f docs/external-dns/walkthrough/coredns/dnsrecord_myapp.coredns.hcpapps.local.yaml 
dig @172.32.0.2 -p 31541 myapp.coredns.hcpapps.local +short
3.3.3.3
2.2.2.2
1.1.1.1
```








