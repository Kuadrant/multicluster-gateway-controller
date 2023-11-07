# Simple Rate Limiting for Application Developers

This user guide walks you through an example of how to configure rate limiting for an endpoint of an application using Kuadrant.

## Requirements

- Complete the [Multicluster Gateways Walkthrough](./multicluster-gateways-walkthrough.md) where you'll have an environment configured with a Gateway that we'll use in this guide.

## Overview

In this guide, we will rate limit a sample REST API called **Toy Store**. In reality, this API is just an echo service that echoes back to the user whatever attributes it gets in the request. The API listens to requests at the hostname `api.$MGC_ZONE_ROOT_DOMAIN`, where it exposes the endpoints `GET /toys*` and `POST /toys`, respectively, to mimic operations of reading and writing toy records.

We will rate limit the `POST /toys` endpoint to a maximum of 5rp10s ("5 requests every 10 seconds").

### ① Deploy the Toy Store API

#### Create the Deployment

> **Note:** You can skip this step and proceed to [Create the HTTPRoute](#create-the-httproute) if you've already deployed the Toy Store API as part of the [AuthPolicy for Application Developers and Platform Engineers](https://docs.kuadrant.io/kuadrant-operator/doc/user-guides/auth-for-app-devs-and-platform-engineers/#2-deploy-the-toy-store-sample-application-persona-app-developer) guide.

Create the deployments for both clusters we've created previously (`kind-mgc-control-plane` & `kind-mgc-workload-1`).

```bash
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
Create a HTTPRoute to route traffic to the services via the Gateways:

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
        value: "/toys"
    backendRefs:
    - name: toystore
      port: 80
  - matches: # it has to be a separate HTTPRouteRule so we do not rate limit other endpoints
    - method: POST
      path:
        type: Exact
        value: "/toys"
    backendRefs:
    - name: toystore
      port: 80
EOF
done
```

Verify the routes work:

```sh
curl -ik https://toystore.$MGC_ZONE_ROOT_DOMAIN/toys
# HTTP/1.1 200 OK
```

Given the two clusters, and our previously created `DNSPolicy`, traffic will load balance between these clusters round-robin style. Load balancing here will be determined in part by DNS TTLs, so it can take a minute or two for requests to flow to both services.

### ② Enforce rate limiting on requests to the Toy Store API

Create a Kuadrant `RateLimitPolicy` to configure rate limiting:

![](https://i.imgur.com/2A9sXXs.png)

```sh
for context in kind-mgc-control-plane kind-mgc-workload-1; do kubectl --context $context apply -f - <<EOF
apiVersion: kuadrant.io/v1beta2
kind: RateLimitPolicy
metadata:
  name: toystore
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: toystore
  limits:
    "create-toy":
      rates:
      - limit: 5
        duration: 10
        unit: second
      routeSelectors:
      - matches: # selects the 2nd HTTPRouteRule of the targeted route
        - method: POST
          path:
            type: Exact
            value: "/toys"
EOF
done
```

> **Note:** It may take a couple of minutes for the RateLimitPolicy to be applied depending on your cluster.

<br/>

Verify the rate limiting works by sending requests in a loop.

Up to 5 successful (`200 OK`) requests every 10 seconds to `POST /toys`, then `429 Too Many Requests`:

```sh
while :; do curl --write-out '%{http_code}' --silent -k --output /dev/null https://toystore.$MGC_ZONE_ROOT_DOMAIN/toys -X POST | egrep --color "\b(429)\b|$"; sleep 1; done
```

Unlimited successful (`200 OK`) to `GET /toys`:

```sh
while :; do curl --write-out '%{http_code}' --silent -k  --output /dev/null https://toystore.$MGC_ZONE_ROOT_DOMAIN/toys | egrep --color "\b(429)\b|$"; sleep 1; done
```

## Next Steps

Here are some good, follow-on guides that build on this walkthrough:

* [AuthPolicy for Application Developers and Platform Engineers](https://docs.kuadrant.io/kuadrant-operator/doc/user-guides/auth-for-app-devs-and-platform-engineers/)
* [Deploying/Configuring Metrics.](./metrics-walkthrough.md)