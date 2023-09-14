# Creating and using a ManagedZone resource.

## What is a ManagedZone
A ManagedZone is a reference to a [DNS zone](https://en.wikipedia.org/wiki/DNS_zone). 
By creating a ManagedZone we are instructing the MGC about a domain or subdomain that can be used as a host by any gateways in the same namespace.
These gateways can use a subdomain of the ManagedZone.

If a gateway attempts to a use a domain as a host, and there is no matching ManagedZone for that host, then that host on that gateway will fail to function.

A gateway's host will be matched to any ManagedZone that the host is a subdomain of, i.e. `test.api.hcpapps.net` will be matched by any ManagedZone (in the same namespace) of: `test.api.hcpapps.net`, `api.hcpapps.net` or `hcpapps.net`.

When MGC wants to create the DNS Records for a host, it will create them in the most exactly matching ManagedZone.
e.g. given the zones `hcpapps.net` and `api.hcpapps.net` the DNS Records for the host `test.api.hcpapps.net` will be created in the `api.hcpapps.net` zone.

### Delegation
Delegation allows you to give control of a subdomain of a root domain to MGC while the root domain has it's DNS zone elsewhere.

In the scenario where a root domain has a zone outside Route53, e.g. `external.com`, and a ManagedZone for `delegated.external.com` is required, the following steps can be taken:
- Create the ManagedZone for `delegated.external.com` and wait until the status is updated with an array of nameservers (e.g. `ns1.hcpapps.net`, `ns2.hcpapps.net`). 
- Copy these nameservers to your root zone for `external.com`, you can create a NS record for each nameserver against the `delegated.external.com` record.

For example:
```
delegated.external.com. 3600 IN NS ns1.hcpapps.net.
delegated.external.com. 3600 IN NS ns2.hcpapps.net.
```

Now, when MGC creates a DNS record in it's Route53 zone for `delegated.external.com`, it will be resolved correctly.
### Creating a ManagedZone
To create a `ManagedZone`, you will first need to create a DNS provider Secret. To create one, see our [DNS Provider](./dns-provider.md) setup guide, and make note of your provider's secret name.


#### Example ManagedZone
To create a bew `ManagedZone` with AWS Route, with a DNS Provider secret named `my-aws-credentials`:

```bash
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: my-test-aws-zone
  namespace: multi-cluster-gateways
spec:
  domainName: mydomain.example.com
  description: "My Managed Zone"
  dnsProviderSecretRef:
    name: my-aws-credentials
    namespace: multicluster-gateway-controller-system
EOF
```

This will create a new Zone in AWS, for `mydomain.example.com`, using the DNS Provider credentials in the `my-aws-credentials` Secret.

If you'd like to create a `ManagedZone` for an _existing_ zone in AWS, note its Zone ID and run:

```bash
kubectl apply -f - <<EOF
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: my-test-aws-zone
  namespace: multi-cluster-gateways
spec:
  id: MYZONEID
  domainName: mydomain.example.com
  description: "My Managed Zone"
  dnsProviderSecretRef:
    name: my-aws-credentials
    namespace: multicluster-gateway-controller-system
EOF
```

**Note:** as an `id` was specified, the Managed Gateway Controller will not re-create this zone, nor will it delete it if this `ManagedZone` is deleted.


### Current limitations
At the moment the MGC is given credentials to connect to the DNS provider at startup using environment variables, because of that, MGC is limited to one provider type (Route53), and all zones must be in the same Route53 account.

There are plans to make this more customizable and dynamic in the future, [work tracked here](https://github.com/Kuadrant/multicluster-gateway-controller/issues/228).

## Spec of a ManagedZone
The ManagedZone is a simple resource with an uncomplicated API, see a sample [here](../../config/samples/kuadrant.io_v1alpha1_managedzone.yaml).

### Mandatory fields
The ManagedZone spec has 2 required fields: `domainName` and `dnsProviderSecretRef`:

```yaml
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: my-test-aws-zone
  namespace: multi-cluster-gateways
spec:
  domainName: mydomain.example.com
  description: "My Managed Zone"
  dnsProviderSecretRef:
    name: my-aws-credentials
    namespace: multicluster-gateway-controller-system
```

#### Spec fields
| Key                    | Example Value                                                             | Required? | Description                                |
|------------------------|---------------------------------------------------------------------------|-----------|--------------------------------------------|
| `dnsProviderSecretRef` | `name: my-credential, namespace: multicluster-gateway-controller-system ` | Required  | Ref to DNS Provider Secret                 |
| `domainName`           | `myapps.example.com`                                                      | Required  | Root Domain Name for this ManagedZone      |
| `id`                   | `Z0WDADW1234`                                                             | Optional  | Zone ID for an existing Zone in GCP or AWS |

#### Additional notes on spec fields

* `dnsProviderSecretRef` 
  * This is a reference to a `Secret` that contains a credential for accessing the DNS Provider. See [DNSProvider](dns-provider.md) for more details.
* `id`
  * By setting the `id`, you are referring to an existing zone in the DNS provider, which MGC will use to manage the DNS of this zone.
  * By leaving the `id` empty, MGC will create a zone in the DNS provider, and store the reference in this field.
* `description`
  * This is simply a human-readable label/description of this resource (e.g. "Use this zone for the staging environment").
* `ParentManagedZone`
  * This allows a zone to be owned by another zone (e.g test.api.domain.com could be owned by api.domain.com), MGC will use this owner relationship to manage the NS values for the subdomain in the parent domain.  
  * Note that for this to work, both the owned and owner zones must be in the Route53 account accessible by MGC.
