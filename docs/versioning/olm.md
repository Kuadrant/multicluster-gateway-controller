## How to create a MGC OLM bundle, catalog and how to install MGC via OLM

:exclamation: NOTE: You can supply different env vars to the following make commands these include:

    * Version using the env var VERSION 
    * Tag via the env var IMAGE_TAG for tags not following the semantic format.
    * Image registry via the env var REGISTRY
    * Registry org via the env var ORG

    For example
```
    make bundle-build-push VERISON=2.0.1
    make catalog-build-push IMAGE_TAG=asdf
```

### Creating the bundle
    

1. Generate build and push the OLM bundle manifests for MGC, run the following make target:
    ```
    make bundle-build-push
    ```
### Creating the catalog

1. Build and push the catalog image
    ```
    make catalog-build-push
    ```
### Installing the operator via OLM catalog

1. Create a namespace:
```bash
   cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: multi-cluster-gateways-system
EOF
```

2. Create a catalog source:
 ```bash
    cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: mgc-catalog
  namespace: olm
spec:
  sourceType: grpc
  image: quay.io/kuadrant/multicluster-gateway-controller-catalog:v6.5.4
  grpcPodConfig:
    securityContextConfig: restricted
  displayName: mgc-catalog
  publisher: Red Hat
EOF
```

3. Create a subscription
```bash
    cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: multicluster-gateway-controller
  namespace: multi-cluster-gateways-system
spec:
  channel: alpha
  name: multicluster-gateway-controller
  source: mgc-catalog
  sourceNamespace: olm
  installPlanApproval: Automatic
EOF
```
4. Create a operator group
```bash
    cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: og-mgc
  namespace: multi-cluster-gateways-system
EOF
 ```
    For more information on each of these OLM resources please see the offical [docs](https://sdk.operatorframework.io/docs/olm-integration/tutorial-bundle/#further-reading)

