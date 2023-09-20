# TLS Policy

The TLSPolicy is a [GatewayAPI](https://gateway-api.sigs.k8s.io/) policy that uses `Direct Policy Attachment` as defined in the [policy attachment mechanism](https://gateway-api.sigs.k8s.io/v1alpha2/references/policy-attachment/) standard.
This policy is used to provide tls for gateway listeners by managing the lifecycle of tls certificates using [`CertManager`](https://cert-manager.io), and is a policy implementation of [`securing gateway resources`](https://cert-manager.io/docs/usage/gateway/). 

## Terms

- [`GatewayAPI`](https://gateway-api.sigs.k8s.io/): resources that model service networking in Kubernetes.
- [`Gateway`](https://gateway-api.sigs.k8s.io/api-types/gateway/): Kubernetes Gateway resource. 
- [`CertManager`](https://cert-manager.io): X.509 certificate management for Kubernetes and OpenShift. 
- [`TLSPolicy`](https://github.com/Kuadrant/multicluster-gateway-controller/blob/main/config/crd/bases/kuadrant.io_dnspolicies.yaml): Kuadrant policy for managing tls certificates with certificate manager.


## TLS Provider Setup

A TLSPolicy acts against a target Gateway by processing its listeners for appropriately configured [tls sections](https://cert-manager.io/docs/usage/gateway/#generate-tls-certs-for-selected-tls-blocks).

If for example a Gateway is created with a listener with a hostname of `echo.apps.hcpapps.net`:
```yaml
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
      hostname: echo.apps.hcpapps.net
      port: 443
      protocol: HTTPS
      tls:
        mode: Terminate
        certificateRefs:
          - name: apps-hcpapps-tls
            kind: Secret
```

## TLSPolicy creation and attachment

The TLSPolicy requires a reference to an existing [CertManager Issuer](https://cert-manager.io/docs/configuration/). 
If we create a [self-signed cluster](https://cert-manager.io/docs/configuration/selfsigned/) issuer with the following:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: selfsigned-cluster-issuer
spec:
  selfSigned: {}
```

We can then create and attach a TLSPolicy to start managing tls certificates for it:

```yaml
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
    name: selfsigned-cluster-issuer
```

### Target Reference
- `targetRef` field is taken from [policy attachment's target reference API](https://gateway-api.sigs.k8s.io/v1alpha2/references/policy-attachment/#target-reference-api). It can only target one resource at a time. Fields included inside:
- `Group` is the group of the target resource. Only valid option is `gateway.networking.k8s.io`.
- `Kind` is kind of the target resource. Only valid options are `Gateway`.
- `Name` is the name of the target resource.
- `Namespace` is the namespace of the referent. Currently only local objects can be referred so value is ignored.

### Issuer Reference
- `issuerRef` field is required and is a reference to a [CertManager Issuer](https://cert-manager.io/docs/configuration/). Fields included inside:
- `Group` is the group of the target resource. Only valid option is `cert-manager.io`.
- `Kind` is kind of issuer. Only valid options are `Issuer` and `ClusterIssuer`.
- `Name` is the name of the target issuer.

The example TLSPolicy shown above would create a [CertManager Certificate](https://cert-manager.io/docs/usage/certificate/) like the following:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    gateway: prod-web
    gateway-namespace: multi-cluster-gateways
    kuadrant.io/tlspolicy: prod-web
    kuadrant.io/tlspolicy-namespace: multi-cluster-gateways
  name: apps-hcpapps-tls
  namespace: multi-cluster-gateways
spec:
  dnsNames:
  - echo.apps.hcpapps.net
  issuerRef:
    group: cert-manager.io
    kind: ClusterIssuer
    name: selfsigned-cluster-issuer
  secretName: apps-hcpapps-tls
  secretTemplate:
    labels:
      gateway: prod-web
      gateway-namespace: multi-cluster-gateways
      kuadrant.io/tlspolicy: prod-web
      kuadrant.io/tlspolicy-namespace: multi-cluster-gateways
  usages:
  - digital signature
  - key encipherment
```

And valid tls secrets generated and synced out to workload clusters:

```bash
kubectl get secrets -A | grep apps-hcpapps-tls
kuadrant-multi-cluster-gateways   apps-hcpapps-tls                    kubernetes.io/tls               3      6m42s
multi-cluster-gateways            apps-hcpapps-tls                    kubernetes.io/tls               3      7m12s
```

## Let's Encrypt Issuer for Route53 hosted domain

Any type of Issuer that is supported by CertManager can be referenced in the TLSPolicy. The following shows how you would create a TLSPolicy that uses [let's encypt](https://letsencrypt.org/) to create production certs for a domain hosted in AWS Route53.

Create a secret containing AWS access key and secret:
```bash
kubectl create secret generic le-aws-credentials --from-literal=AWS_ACCESS_KEY_ID=<AWS_ACCESS_KEY_ID> --from-literal=AWS_SECRET_ACCESS_KEY=<AWS_SECRET_ACCESS_KEY> -n multi-cluster-gateways
```

Create a new Issuer:
```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: le-production
  namespace: multi-cluster-gateways
spec:
  acme:
    email: <YOUR EMAIL>
    preferredChain: ""
    privateKeySecretRef:
      name: le-production
    server: https://acme-v02.api.letsencrypt.org/directory
    solvers:
      - dns01:
          route53:
            hostedZoneID: <YOUR HOSTED ZONE ID>
            region: us-east-1
            accessKeyIDSecretRef:
              key: AWS_ACCESS_KEY_ID
              name: le-aws-credentials
            secretAccessKeySecretRef:
              key: AWS_SECRET_ACCESS_KEY
              name: le-aws-credentials
```

Create a TLSPolicy:
```yaml
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
    kind: Issuer
    name: le-production
```
