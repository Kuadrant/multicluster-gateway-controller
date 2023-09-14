# Configuring a DNS Provider 

In order to be able to interact with supported DNS providers, Kuadrant needs a credential that it can use. This credential is leveraged by the multi-cluster gateway controller in order to create and manage DNS records within zones used by the listeners defined in your gateways.


## Supported Providers

Kuadrant Supports the following DNS providers currently

- AWS Route 53 (AWS)
- Google Cloud DNS (GCP)

### AWS Route 53 Provider

Kuadant expects a `Secret` with a credential. Below is an example for AWS Route 53. It is important to set the secret type to `aws`:


```bash
kubectl create secret generic my-aws-credentials \
  --namespace=multicluster-gateway-controller-system \
  --type=kuadrant.io/aws \
  --from-literal=AWS_ACCESS_KEY_ID=XXXX \
  --from-literal=AWS_REGION=eu-west-1 \
  --from-literal=AWS_SECRET_ACCESS_KEY=XXX
```

| Key                      | Example Value           | Description                                           |
|--------------------------|-------------------------|-------------------------------------------------------|
| `AWS_REGION`             | `eu-west-1`             | AWS Region                                            |
| `AWS_ACCESS_KEY_ID`      | `XXXX`                  | AWS Access Key ID (see note on permissions below)     |
| `AWS_SECRET_ACCESS_KEY`  | `XXXX`                  | AWS Secret Access Key                                 |

#### AWS IAM Permissions Required 
We have tested using the available policy `AmazonRoute53FullAccess` however it should also be possible to restrict the credential down to a particular zone. More info can be found in the AWS docs:

https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/access-control-managing-permissions.html

### Google Cloud DNS Provider

Kuadant expects a secret with a credential. Below is an example for Google DNS. It is important to set the secret type to `gcp`:

```bash
kubectl create secret generic my-test-gcp-credentials \
  --namespace=multicluster-gateway-controller-system \
  --type=kuadrant.io/gcp \
  --from-literal=PROJECT_ID=xxx \
  --from-file=GOOGLE=$HOME/.config/gcloud/application_default_credentials.json
```

| Env Var      | Example Value                                                                                  | Description                                                                                                           |
|--------------|------------------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------|
| `GOOGLE`     | `{"client_id": "***","client_secret": "***","refresh_token": "***","type": "authorized_user"}` | This is the JSON created from either the credential created by the `gcloud` CLI, or the JSON from the Service account |
| `PROJECT_ID` | `my_project_id`                                                                                | ID to the Google project                                                                                              |


#### Google Cloud DNS Access permissions required
See: https://cloud.google.com/dns/docs/access-control#dns.admin


### Where to create the Secrets

It is recommended that you create the secret in the same namespace as your `ManagedZones`. In the examples above, we've stored these in a namespace called `multicluster-gateway-controller-system`.

Now that we have the credential created we have a DNS provdier ready to go and can start using it.

## Using a Credential

Once a `Secret` like the one shown above is created, in order for it to be used, it needs to be associated with a `ManagedZone`. 

See [ManagedZone](managed-zone.md)


## Geolocation

Geolocation is a feature available in both DNS providers we support. A location is needed for both DNS Providers, please see below for the supported location for the provider you require.

:exclamation:
If a unsupported value is given to a provider, DNS records will **not** be created. Please choose carefully. For more information of what location is right for your needs please read said providers documentation. 

### Locations supported per DNS provider

| Supported     | AWS | GCP |
|---------------|-----|-----|
| Continents    | :white_check_mark: |  :x: |
| Country codes | :white_check_mark: |  :x:  |
| States        | :white_check_mark: |  :x:  |
| Regions       |  :x:  | :white_check_mark: |  

### Continents and country codes supported by AWS Route 53

:**Note:** :exclamation: For more information please the official AWS documentation 

To see all regions supported by AWS Route 53 please see the offical (documentation)[https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values-geo.html]

### Regions supported by Google Cloud DNS

To see all regions supported by Google Cloud DNS, please see the offical (documentation)[https://cloud.google.com/compute/docs/regions-zones]

