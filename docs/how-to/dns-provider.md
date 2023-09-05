# Configuring a DNS Provider 

In order to be able to interact with supported DNS providers, Kuadrant needs a credential that it can use. This credential is leveraged by the multi-cluster gateway controller in order to create and manage DNS records within zones used by the listeners defined in your gateways.


## Supported Providers

Kuadrant Supports the following DNS providers currently

- AWS route 53 (aws)
- Google DNS (gcp)



## Configuring an AWS Route 53 provider

Kuadant expects a secret with a credential. Below is an example for AWS Route 53. It is important to set the secret type to `aws`

```
apiVersion: v1
data:
  AWS_ACCESS_KEY_ID: XXXXX
  AWS_REGION: XXXXX
  AWS_SECRET_ACCESS_KEY: XXXXX
kind: Secret
metadata:
  name: aws-credentials
  namespace: multicluster-gateway-controller-system
type: kuadrant.io/aws
```


## IAM permissions required 
We have tested using the available policy `AmazonRoute53FullAccess` however it should also be possible to restrict the credential down to a particular zone. More info can be found in the AWS docs 
https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/access-control-managing-permissions.html

## Configuring a Google DNS provider

Kuadant expects a secret with a credential. Below is an example for Google DNS. It is important to set the secret type to `gcp`

```
apiVersion: v1
data:
  GOOGLE: {"client_id": "00000000-00000000000000.apps.googleusercontent.com","client_secret": "d-FL95Q00000000000000","refresh_token": "00000aaaaa00000000-AAAAAAAAAAAAKFGJFJDFKDK","type": "authorized_user"}
  PROJECT_ID: "my-project"
kind: Secret
metadata:
  name: gcp-credentials
  namespace: multicluster-gateway-controller-system
type: kuadrant.io/gcp
```


### Access permissions required
https://cloud.google.com/dns/docs/access-control#dns.admin


### Where to create the secret.

It is recommended that you create the secret in the same namespace as your `ManagedZones`
Now that we have the credential created we have a DNS provdier ready to go and can start using it.

## Using a credential

Once a secret like the one shown above is created, in order for it to be used, it needs to be associated with a `ManagedZone`. 

See [ManagedZone](managedZone.md)

