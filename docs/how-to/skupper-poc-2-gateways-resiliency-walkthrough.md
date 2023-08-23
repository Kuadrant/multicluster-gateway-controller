# Skupper proof of concept: 2 clusters & gateways, resiliency walkthrough

## Introduction

This walkthrough shows how Skupper can be used to provide service resiliency
across 2 clusters. Each cluster is running a Gateway with a HttpRoute in front
of an application Service. By leveraging Skupper, the application Service can be
exposed (using the skupper cli) from either cluster. If the Service is
unavailable on the local cluster, it will be routed to another cluster that has
exposed that Service.

<img src="images/skupper-poc-2-gateways-resiliency-walkthrough.png" alt="architecture" width="600"/>

## Requirements

* Local environment has been set up with a hub and spoke cluster, as per the [main walkthrough](./ocm-control-plane-walkthrough.md).
  * The example multi-cluster Gateway has been deployed to both clusters
  * The example echo HttpRoute, Service and Deployment have been deployed to both clusters in the `default` namespace, and the `MGC_SUB_DOMAIN` env var set in your terminal
* [Skupper CLI](https://skupper.io/docs/cli/index.html#installing-cli) has been installed.

## Skupper Setup

Install Skupper on the hub & spoke clusters using the following command

```bash
make skupper-setup
```

Expose the Service in the `default` namespace in both directions over the skupper network

```bash
kubectl config use-context kind-mgc-control-plane
skupper expose deployment/echo --port 8080
kubectl config use-context kind-mgc-workload-1
skupper expose deployment/echo --port 8080
```

Verify the application route can be hit,
taking note of the pod name in the response.

```bash
curl -k https://$MGC_SUB_DOMAIN
Request served by <POD_NAME>
```

Locate the pod that is currently serving requests. It is either in the hub or
spoke cluster. There goal is to scale down the deployment to 0 replicas.

```bash
kubectl config use-context kind-mgc-control-plane
kubectl get po -n default | grep echo

kubectl config use-context kind-mgc-workload-1
kubectl get po -n default | grep echo
```

Run this command to scale down the deployment in the right cluster:

```bash
kubectl scale deployment echo --replicas=0 -n default
```

Verify the application route can still be hit,
and the pod name matches the one that has *not* been scaled down.

```bash
curl -k https://$MGC_SUB_DOMAIN
```

You can also force resolve the DNS result to alternate between the 2 Gateway
clusters to verify requests get routed across the Skupper network.

```bash
curl -k --resolve $MGC_SUB_DOMAIN:443:172.31.200.2 https://$MGC_SUB_DOMAIN
curl -k --resolve $MGC_SUB_DOMAIN:443:172.31.201.2 https://$MGC_SUB_DOMAIN
```

## Known Issues

If you get an error response `no healthy upstream` from curl, there may be a
problem with the skupper network or link. Check back on the output from earlier
commands for any indication of problems setting up the network or link. The
skupper router & service controller logs can be checked in the `default`
namespace in both clusters.
