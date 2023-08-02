## Getting Started


### Pre-requisites

- [kind](https://kind.sigs.k8s.io/)
- [operator-sdk](https://sdk.operatorframework.io/docs/installation/)
- [yq](https://mikefarah.gitbook.io/yq/v/v3.x/)
- [clusteradm](https://github.com/open-cluster-management-io/clusteradm#install-the-clusteradm-command-line)
- go >= 1.20
- openssl >= 3
- AWS account with Route 53 enabled
- https://github.com/chipmk/docker-mac-net-connect (for macos users)

* Export env-vars with the keys listed below. Fill in your own values as appropriate. You will need access to a domain or subdomain in Route 53 in AWS:

| Env Var                      | Example Value               | Description                                                    |
|------------------------------|-----------------------------|----------------------------------------------------------------|
| `MGC_ZONE_ROOT_DOMAIN`       | `jbloggs.hcpapps.net`       | Hostname for the root Domain                                   |
| `MGC_AWS_DNS_PUBLIC_ZONE_ID` | `Z01234567US0IQE3YLO00`     | AWS Route 53 Zone ID for specified `MGC_ZONE_ROOT_DOMAIN`      | | 
| `MGC_AWS_ACCESS_KEY_ID`      | `AKIA1234567890000000`      | Access Key ID, with access to resources in Route 53            |
| `MGC_AWS_SECRET_ACCESS_KEY`  | `Z01234567US0000000`        | Access Secret Access Key, with access to resources in Route 53 |
| `MGC_AWS_REGION`             | `eu-west-1`                 | AWS Region                                                     |
| `MGC_SUB_DOMAIN`             | `myapp.jbloggs.hcpapps.net` | AWS Region                                                     |

* Alternatively, to set defaults, add the above environment variables to your `.zshrc` or `.bash_profile`.


For the follow-on walkthrough, we're going to use multiple terminal sessions/windows.

Open two windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub Cluster)
* `T2` (Workloads cluster)

* NOTE: MCG_SUB_DOMAIN env var is required in both terminals


### Setup clusters and Multi-cluster Gateway Controller

   ```bash
    curl https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/quickstart-setup.sh | bash
   ```

### What is next

Now that you have 2 kind clusters configured and with multicluster-gateway-controller installed you are ready to begin [creating gateways](https://docs.kuadrant.io/multicluster-gateway-controller/docs/how-to/ocm-control-plane-walkthrough/#create-a-gateway).

