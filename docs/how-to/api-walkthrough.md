# API Walkthrough

## Introduction

This document will detail the setup of a reference architecture to support a number of API management use-cases connecting Kuadrant with other projects in the wider API management on Kubernetes ecosystem.

## Platform Engineer Steps (Part 1)


<!-- TODO: Copy formatting & env var info from the MGC Getting Started guide -->

Export the following env vars:

    ```
    export KUADRANT_AWS_ACCESS_KEY_ID=<key_id>
    export KUADRANT_AWS_SECRET_ACCESS_KEY=<secret>
    export KUADRANT_AWS_REGION=<region>
    export KUADRANT_AWS_DNS_PUBLIC_ZONE_ID=<zone>
    export KUADRANT_ZONE_ROOT_DOMAIN=<domain>
    ```

Run the following command, choosing `aws` as the dns provider:

<!-- TODO: Change to a curl command that fetches everything remotely -->

    ```bash
    MGC_LOCAL_QUICKSTART_SCRIPTS_MODE=true MGC_BRANCH=api-upstream ./hack/quickstart-setup-api.sh
    ```

### Create a gateway

<!-- TODO: Create Gateway & TLSPolicy as part of quickstart, if possible -->


View the ManagedZone, Gateway and TLSPolicy:

    ```bash
    kubectl --context kind-mgc-control-plane describe managedzone mgc-dev-mz -n multi-cluster-gateways
    kubectl --context kind-mgc-control-plane describe gateway -n multi-cluster-gateways
    kubectl --context kind-mgc-control-plane describe tlspolicy -n multi-cluster-gateways
    ```

### Guard Rails: Show Constraint warnings about missing policies ( DNS, AuthPolicy, RLP)

<!-- TODO: Instructions how to get to the dashboard -->

### Create missing Policies

<!-- TODO: Guard Rails: Show Constraint warnings about missing policies ( DNS, AuthPolicy, RLP) -->

Create a DNSPolicy:

<!-- TODO: Import dnspolicy from platform-engineer repo into this repo -->

    ```bash
    envsubst < ./resources/dnspolicy.yaml | kubectl --context kind-mgc-control-plane apply -f -
    kubectl --context kind-mgc-control-plane describe dnspolicy prod-web -n multi-cluster-gateways
    ```

View ns entries in Route 53 DNS Zone

<!-- TODO: Instructions how to find ns entries in route53 zone -->

Create and configure a Gateway-wide RateLimitPolicy

<!-- TODO: Import ratelimitpolicy from platform-engineer repo into this repo -->

    ```bash
    kubectl --context kind-mgc-control-plane apply -f ./resources/ratelimitpolicy.yaml
    kubectl --context kind-mgc-control-plane describe ratelimitpolicy prod-web -n multi-cluster-gateways
    ```

Create and configure a Gateway-wide AuthPolicy

<!-- TODO: Import authpolicy from platform-engineer repo into this repo -->

    ```bash
    kubectl --context kind-mgc-control-plane apply -f ./resources/authpolicy.yaml
    kubectl --context kind-mgc-control-plane describe authpolicy gw-auth -n multi-cluster-gateways
    ```

### Platform Overview

Open Platform Engineer Dashboard in Grafana to see:

<!-- TODO: Instructions how to get to the dashboard -->

* Gateways & Policies - Gateway Policies created
* No Route Policies yet (as no APIs/Apps deployed yet) - Can see a TLSPolicy, DNSPolicy, AuthPolicy and RateLimitPolicy
* Constraints & Violations - Highlight no more violations
* APIs Summary - Highlight there are no APIs yet

## App Developer Steps

### API Setup

TODO

* deploy the petstore app to 1 cluster
* Open api spec in apicurio studio, showing x-kuadrant extensions & making requests (with swagger) to the show rate limit policy
* Modify x-kuadrant extension to change rate limit
* Export spec and generate resources with kuadrantctl
* Apply generated resources to petstore app in cluster
* Back in apicurio studio, modify x-kuadrant extension to add auth to /store/inventory endpoint
* Export spec, generate resources and reapply to cluster
* Verify auth policy via swagger

### Multicluster Bonanza

TODO

* deploy the petstore to 2nd cluster (assuming deployed via manifestwork or argocd, can update a placement)

e.g.

    ```bash
    kubectl --context kind-mgc-control-plane patch placement petstore -n argocd --type='json' -p='[{"op": "add", "path": "/spec/clusterSets/-", "value": "petstore-region-us"}, {"op": "replace", "path": "/spec/numberOfClusters", "value": 2}]'
    ```

Describe the DNSPolicy

    ```bash
    kubectl --context kind-mgc-control-plane describe dnspolicy prod-web -n multi-cluster-gateways
    ```

Show ManagedCluster labelling

    ```bash
    kubectl --context kind-mgc-control-plane get managedcluster -A -o custom-columns="NAME:metadata.name,URL:spec.managedClusterClientConfigs[0].url,REGION:metadata.labels.kuadrant\.io/lb-attribute-geo-code"
    ```

Show DNS resolution per geo region

TODO

Show rate limiting working on both clusters/apps.

### App Developer Overview: Show API traffic & impact of AuthPolicy & Rate Limit Policy

Open the App Developer Dashboard

<!-- TODO: Instructions how to get to the dashboard -->

* List of APIs, corresponding to our HTTPRoute coming from our OAS spec

## Platform Engineer Steps (Part 2)

### Platform Overview

Open Platform Engineer Dashboard in Grafana to see:

<!-- TODO: Instructions how to get to the dashboard -->

* Gateways & Policies - new resources created by App Developer shown
* Constraints & Violations - no violations
* APIs Summary - API created by App Developer shown, including summary traffic
