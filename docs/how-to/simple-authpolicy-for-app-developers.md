# Authenticated API for Application Developers

This user guide walks you how to configure and protect Gateway API endpoints by declaring Kuadrant `AuthPolicy` custom resources.

## Requirements

- Complete the [Multicluster Gateways Walkthrough](./multicluster-gateways-walkthrough.md), and you'll have an environment configured with a Gateway that we'll use in this guide.

## Setup

### ①  Deploy the Toy Store API

#### Create the Deployment

> **Note:** You can skip this step and proceed to [Create the HTTPRoute](#create-the-httproute) if you've already deployed the Toy Store API as part of [the RateLimitPolicy for App Developers guide](./simple-ratelimitpolicy-for-app-developers.md#-deploy-the-toy-store-api).

Create the deployments for both clusters (`kind-mgc-control-plane` & `kind-mgc-workload-1`) we've created previously in the [Multicluster Gateways Walkthrough](./multicluster-gateways-walkthrough.md):

```sh
for context in kind-mgc-control-plane kind-mgc-workload-1; do kubectl --context $context apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: toystore
  labels:
    app: toystore
spec:
  selector:
    matchLabels:
      app: toystore
  template:
    metadata:
      labels:
        app: toystore
    spec:
      containers:
        - name: toystore
          image: quay.io/3scale/authorino:echo-api
          env:
            - name: PORT
              value: "3000"
          ports:
            - containerPort: 3000
              name: http
  replicas: 1
---
apiVersion: v1
kind: Service
metadata:
  name: toystore
spec:
  selector:
    app: toystore
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: 3000
EOF
done
```

#### Create the HTTPRoute

Create a HTTPRoute to route traffic to the service via our previously configured Gateway:

![](https://i.imgur.com/rdN8lo3.png)

```sh
for context in kind-mgc-control-plane kind-mgc-workload-1; do kubectl --context $context apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: toystore
spec:
  parentRefs:
    - kind: Gateway
      name: prod-web
      namespace: kuadrant-multi-cluster-gateways
  hostnames:
  - toystore.$MGC_ZONE_ROOT_DOMAIN
  rules:
  - matches:
    - method: GET
      path:
        type: PathPrefix
        value: "/cars"
    - method: GET
      path:
        type: PathPrefix
        value: "/dolls"
    backendRefs:
    - name: toystore
      port: 80
  - matches:
    - path:
        type: PathPrefix
        value: "/admin"
    backendRefs:
    - name: toystore
      port: 80
EOF
done
```

Send requests to the unprotected API:

```sh
curl -ik https://toystore.$MGC_ZONE_ROOT_DOMAIN/cars
# HTTP/1.1 200 OK
```

```sh
curl -ik https://toystore.$MGC_ZONE_ROOT_DOMAIN/dolls
# HTTP/1.1 200 OK
```

```sh
curl -ik https://toystore.$MGC_ZONE_ROOT_DOMAIN/admin
# HTTP/1.1 200 OK
```

### ② Protect the API

Create a a Kuadrant `AuthPolicy` to enforce the following auth rules:
- **Authentication:**
  - All users must present a valid API key when accessing the `admin` API
- **Authorization:**
  - `/admin*` routes require user mapped to the `admins` group (`kuadrant.io/groups=admins` annotation added to the Kubernetes API key Secret)

```bash
for context in kind-mgc-control-plane kind-mgc-workload-1; do kubectl --context $context apply -f - <<EOF
apiVersion: kuadrant.io/v1beta1
kind: AuthPolicy
metadata:
  name: toystore
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: toystore
  rules:
  - paths: ["/admin"]
  authScheme:
    identity:
    - name: api-key-users
      apiKey:
        selector:
          matchLabels:
            app: toystore
        allNamespaces: true
      credentials:
        in: authorization_header
        keySelector: APIKEY
    response:
    - name: identity
      json:
        properties:
        - name: userid
          valueFrom:
            authJSON: auth.identity.metadata.annotations.secret\.kuadrant\.io/user-id
      wrapper: envoyDynamicMetadata
EOF
done
```

<details>
  <summary>(Optional) Verify internal custom resources reconciled by Kuadrant</summary>
  <br/>

  Verify the Authorino AuthConfig created in association with the policy:

  ```sh
  kubectl get authconfig/ap-default-toystore -o yaml
  ```
</details>


Create the API key:

```sh
for context in kind-mgc-control-plane kind-mgc-workload-1; do kubectl --context $context apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: api-key-admin-user
  labels:
    authorino.kuadrant.io/managed-by: authorino
    app: toystore
  annotations:
    kuadrant.io/groups: admins
stringData:
  api_key: iamanadmin
type: Opaque
EOF
done
```

Send requests to the API protected by Kuadrant:

```sh
curl -ik https://toystore.$MGC_ZONE_ROOT_DOMAIN/cars
# HTTP/1.1 200 OK - Unprotected API
```

```sh
curl -ik https://toystore.$MGC_ZONE_ROOT_DOMAIN/admin
# HTTP/1.1 401 Unauthorized
```

```sh
curl -ik -H 'Authorization: APIKEY iamanadmin' https://toystore.$MGC_ZONE_ROOT_DOMAIN/admin
# HTTP/1.1 200 OK
```


## Next Steps

Here are some good, follow-on guides that build on this walkthrough:

* [Simple RateLimitPolicy for App Developers](./simple-ratelimitpolicy-for-app-developers.md)
* [Deploying/Configuring Metrics.](../how-to/metrics-walkthrough.md)