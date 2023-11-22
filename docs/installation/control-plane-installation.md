# Setting up MGC in Existing OCM Clusters

This guide will show you how to install and configure the Multi-Cluster Gateway Controller in pre-existing [Open Cluster Management](https://open-cluster-management.io/) configured clusters.

## Prerequisites

- A **hub cluster** running the OCM control plane (v0.11.0 or greater)
- Addons enabled `clusteradm install hub-addon --names application-manager`
- Any number of additional **spoke clusters** that have been configured as OCM [ManagedClusters](https://open-cluster-management.io/concepts/managedcluster/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) (>= v1.14.0)
- Either a pre-existing [cert-manager](https://cert-manager.io/)(>=v1.12.2) installation or the [Kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) and [Helm](https://helm.sh/docs/intro/quickstart/#install-helm) CLIs
- Amazon Web services (AWS) and or Google cloud provider (GCP) credentials. See the [DNS Provider](../dnspolicy/dns-provider.md) guide for obtaining these credentials.

## Configure OCM with RawFeedbackJsonString Feature Gate

All OCM spoke clusters must be configured with the `RawFeedbackJsonString` feature gate enabled. This can be done in two ways:

1. When running the `clusteradm join` command that joins the spoke cluster to the hub:

    Get the `join` flags and token by running

    ```bash
    join=$(clusteradm get token --context kind-test-control-plane | grep -o 'join.*--cluster-name')
    ```
    
    ```bash
    clusteradm $join --feature-gates=RawFeedbackJsonString=true
    ```

2. By patching each spoke cluster's `klusterlet` in an existing OCM install:

    ```bash
    kubectl patch klusterlet klusterlet --type merge --patch '{"spec": {"workConfiguration": {"featureGates": [{"feature": "RawFeedbackJsonString", "mode": "Enable"}]}}}' --context <EACH_SPOKE_CLUSTER>
    ```

## Installing MGC

First, run the following command in the context of your hub cluster to install the Gateway API CRDs:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.0.0/standard-install.yaml"
```

We can then add a `wait` to verify the CRDs have been established:

```bash
kubectl wait --timeout=5m crd/gatewayclasses.gateway.networking.k8s.io crd/gateways.gateway.networking.k8s.io crd/httproutes.gateway.networking.k8s.io --for=condition=Established
```
```
customresourcedefinition.apiextensions.k8s.io/gatewayclasses.gateway.networking.k8s.io condition met
customresourcedefinition.apiextensions.k8s.io/gateways.gateway.networking.k8s.io condition met
customresourcedefinition.apiextensions.k8s.io/httproutes.gateway.networking.k8s.io condition met
```

Then run the following command to install the MGC:

```bash
kubectl apply -k "github.com/kuadrant/multicluster-gateway-controller.git/config/mgc-install-guide?ref=release-0.2"
```

In addition to the MGC, this will also install the Kuadrant add-on manager and a `GatewayClass` from which MGC-managed `Gateways` can be instantiated.

After the configuration has been applied, you can verify that the MGC and add-on manager have been installed and are running:

```bash
kubectl wait --timeout=5m -n multicluster-gateway-controller-system deployment/mgc-controller-manager deployment/mgc-add-on-manager deployment/mgc-policy-controller --for=condition=Available
```
```
deployment.apps/mgc-controller-manager condition met
deployment.apps/mgc-add-on-manager condition met
deployment/mgc-policy-controller condition met
```

We can also verify that the `GatewayClass` has been accepted by the MGC:

```bash
kubectl wait --timeout=5m gatewayclass/kuadrant-multi-cluster-gateway-instance-per-cluster --for=condition=Accepted
```
```
gatewayclass.gateway.networking.k8s.io/kuadrant-multi-cluster-gateway-instance-per-cluster condition met
```

## Creating a ManagedZone

To manage the creation of DNS records, MGC uses [ManagedZone](../dnspolicy/managed-zone.md) resources. A `ManagedZone` can be configured to use DNS Zones on both AWS (Route53), and GCP (Cloud DNS). 

First, depending on the provider you would like to use export the [environment variables detailed here](https://docs.kuadrant.io/multicluster-gateway-controller/docs/getting-started/#config) in a terminal session.

Next, create a secret containing either the AWS or GCP credentials. We'll also create a namespace for your MGC configs:

:**Note:** :exclamation: If you need help getting the configuration for these credentials, see the [DNS Provider](../dnspolicy/dns-provider.md) guide.

#### AWS:
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: multi-cluster-gateways
---
apiVersion: v1
kind: Secret
metadata:
  name: mgc-aws-credentials
  namespace: multi-cluster-gateways
type: "kuadrant.io/aws"
stringData:
  AWS_ACCESS_KEY_ID: ${MGC_AWS_ACCESS_KEY_ID}
  AWS_SECRET_ACCESS_KEY: ${MGC_AWS_SECRET_ACCESS_KEY}
  AWS_REGION: ${MGC_AWS_REGION}
EOF
```
#### GCP
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: multi-cluster-gateways
---
apiVersion: v1
kind: Secret
metadata:
  name: mgc-gcp-credentials
  namespace: multi-cluster-gateways
type: "kuadrant.io/gcp"
stringData:
  GOOGLE: ${GOOGLE}
  PROJECT_ID: ${PROJECT_ID}
EOF
```

A `ManagedZone` can then be created:

#### AWS:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: mgc-dev-mz
  namespace: multi-cluster-gateways
spec:
  id: ${MGC_AWS_DNS_PUBLIC_ZONE_ID}
  domainName: ${MGC_ZONE_ROOT_DOMAIN}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: mgc-aws-credentials
    namespace: multi-cluster-gateways
EOF
```
#### GCP

```bash
cat <<EOF | kubectl apply -f -
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: mgc-dev-mz
  namespace: multi-cluster-gateways
spec:
  id: ${ZONE_NAME}
  domainName: ${ZONE_DNS_NAME}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: mgc-gcp-credentials
    namespace: multi-cluster-gateways
EOF
```

You can now verify that the `ManagedZone` has been created and is in a ready state:

```bash
kubectl get managedzone -n multi-cluster-gateways
```
```
NAME         DOMAIN NAME      ID                                  RECORD COUNT   NAMESERVERS                                                                                         READY
mgc-dev-mz   ef.hcpapps.net   /hostedzone/Z06419551EM30QQYMZN7F   2              ["ns-1547.awsdns-01.co.uk","ns-533.awsdns-02.net","ns-200.awsdns-25.com","ns-1369.awsdns-43.org"]   True
```

## Creating a Cert Issuer

To create a `CertIssuer`, [cert-manager](https://cert-manager.io/) first needs to be installed on your hub cluster. If this has not previously been installed on the cluster you can run the command below to do so:

```bash
kustomize --load-restrictor LoadRestrictionsNone build "github.com/kuadrant/multicluster-gateway-controller.git/config/mgc-install-guide/cert-manager?ref=main" --enable-helm | kubectl apply -f -
```

We will now create a `ClusterIssuer` to be used with `cert-manager`. For simplicity, we will create a self-signed cert issuer here, but [other issuers can also be configured](https://cert-manager.io/docs/configuration/).

```bash
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: mgc-ca
  namespace: cert-manager
spec:
  selfSigned: {}
EOF
```

Verify that the `clusterIssuer` is ready:

```bash
kubectl wait --timeout=5m -n cert-manager clusterissuer/mgc-ca --for=condition=Ready
```
```
clusterissuer.cert-manager.io/mgc-ca condition met
```

## Next Steps

Now that you have MGC installed and configured in your hub cluster, you can now continue with any of these follow-on guides:

- Installing the [Kuadrant Service Protection components](./service-protection-installation.md)