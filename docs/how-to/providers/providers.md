# DNS Providers supported by multi-cluster gateway cluster

## Introduction
The following document tells you everything you need to know about the DNS Provider the multi cluster gateway cluster supports and the specific features we offer that utilises them.

## Current provider supported

In the current iteration of the multi-cluster gateway controller we support both **AWS (Amazon web services) Route 53** and **GCP (Google cloud provider) cloud DNS**. 

### Geolocation

Geolocation is a feature available in both DNS providers we support. A location is needed for both DNS Providers, please see below for the supported location for the provider you require.

:exclamation:
If a unsupported value is given to a provider, DNS records will **not** be created. Please choose carefully. For more information of what location is right for your needs please read said providers documentation. 

## Locations supported per DNS provider

| Supported     | AWS | GCP |
|---------------|-----|-----|
| Continents    | :white_check_mark: |  :x: |
| Country codes | :white_check_mark: |  :x:  |
| States        | :white_check_mark: |  :x:  |
| Regions       |  :x:  | :white_check_mark: |  

## Continents and country codes supported by AWS Route 53

:**Note:** :exclamation: For more information please the official AWS documentation 

To see all regions supported by AWS Route 53 please see the offical (documtation)[https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values-geo.html]

## Regions supported by GCP CLoud DNS

To see all regions supported by GCP Cloud DNS please see the offical (documtation)[https://cloud.google.com/compute/docs/regions-zones]
## Setting up DNS provider

### AWS Route 53

#### Setup a `./controller-config.env` env-var file in the root of the repo with the following keys. Fill in your own values as appropriate. You will need access to a domain or subdomain in Route 53 in AWS:

  ```bash
  AWS_DNS_PUBLIC_ZONE_ID=Z01234567US0IQE3YLO00
  ZONE_ROOT_DOMAIN=jbloggs.hcpapps.net
  LOG_LEVEL=1
  ```

  | Env Var                  | Example Value           | Description                                           |
  |--------------------------|-------------------------|-------------------------------------------------------|
  | `ZONE_ROOT_DOMAIN`       | `jbloggs.hcpapps.net`   | Hostname for the root Domain                          |
  | `AWS_DNS_PUBLIC_ZONE_ID` | `Z01234567US0IQE3YLO00` | AWS Route 53 Zone ID for specified `ZONE_ROOT_DOMAIN` |
  | `LOG_LEVEL`              | `1`                     | Log level for the Controller                          |

#### Setup a `./aws-credentials.env` with credentials to access route 53

  For example:

```bash
AWS_ACCESS_KEY_ID=<access_key_id>
AWS_SECRET_ACCESS_KEY=<secret_access_key>
AWS_REGION=eu-west-1
```

  | Env Var                 | Example Value          | Description                                                    |
  |-------------------------|------------------------|----------------------------------------------------------------|
  | `AWS_ACCESS_KEY_ID`     | `AKIA1234567890000000` | Access Key ID, with access to resources in Route 53            |
  | `AWS_SECRET_ACCESS_KEY` | `Z01234567US0000000`   | Access Secret Access Key, with access to resources in Route 53 |
  | `AWS_REGION`            | `eu-west-1`            | AWS Region                                                     |

### GCP Cloud DNS 

### Application Default Credentials (ADC)
There are 2 types of methods that can be used to create credentials in the format that the MGC uses:
1. User credentials provided by using the gcloud CLI
2. Service account

#### User credentials provided by using the cloud CLI

1. Install Google cloud [cli](https://cloud.google.com/sdk/docs/install)
2. Run the following [commands](https://cloud.google.com/docs/authentication/application-default-credentials#personal)

#### Service Account

To create a google service account please run the following [guide](https://cloud.google.com/docs/authentication/application-default-credentials#attached-sa)


#### Setup a `./gcp-credentials.env` with credentials to access Google Cloud DNS

  For example:

``` bash
GOOGLE={"client_id": "00000000-00000000000000.apps.googleusercontent.com","client_secret": "d-FL95Q00000000000000","refresh_token": "00000aaaaa00000000-AAAAAAAAAAAAKFGJFJDFKDK","type": "authorized_user"}
PROJECT_ID: my_project_id

```

  | Env Var                 | Example Value          | Description                                                    |
  |-------------------------|------------------------|----------------------------------------------------------------|
  | `GOOGLE`     | `{"client_id": "00000000-00000000000000.apps.googleusercontent.com","client_secret": "d-FL95Q00000000000000","refresh_token": "00000aaaaa00000000-AAAAAAAAAAAAKFGJFJDFKDK","type": "authorized_user"}` |  This is the json created from either the credential created by the cli or the json from the Service account             |
  | `PROJECT_ID` | `my_project_id`   | ID to the google project |

#### Setup a `./controller-config.env` env-var file in the root of the repo with the following keys. Fill in your own values as appropriate. You will need access to a domain n in Cloud DNS in GCP:

  ```bash
  ZONE_ROOT_DOMAIN_GCP=jbloggs.google.hcpapps.net
  LOG_LEVEL=1
  ```

  | Env Var                  | Example Value           | Description                                           |
  |--------------------------|-------------------------|-------------------------------------------------------|
  | `ZONE_NAME`       | `jbloggs-google`   | Hostname for the root Domain                          |
  | `ZONE_DNS_NAME` | `jbloggs.google.hcpapps.net`   | Hostname for the root Domain                          |
  | `LOG_LEVEL`              | `1`                     | Log level for the Controller                          |


### Local setup make command
To get the local setup to create the relevant provider secrets based on the above .env files you will need to run the following make local-setup command

```
make local setup
```
