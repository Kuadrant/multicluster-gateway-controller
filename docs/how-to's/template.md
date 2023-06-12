# Title

## Introduction
blah blah amazing and wonderful feature blah blah gateway blah blah DNS 

## Prerequisities
* A computer
* Electricity
* Kind
* AWS Account
* Route 53 enabled


 ## Installation and Setup
1. Clone this repo locally 
1. Setup a `./controller-config.env` file in the root of the repo with the following key values

    ```bash
    # this sets up your default managed zone
    AWS_DNS_PUBLIC_ZONE_ID=<AWS ZONE ID>
    # this is the domain at the root of your zone (foo.example.com)
    ZONE_ROOT_DOMAIN=<replace.this>
    LOG_LEVEL=1
    ```   

1. setup a `./aws-credentials.env` with credentials to access route 53

    For example:

    ```bash
    AWS_ACCESS_KEY_ID=<access_key_id>
    AWS_SECRET_ACCESS_KEY=<secret_access_key>
    AWS_REGION=eu-west-1
    ```

## Open terminal sessions
For this walkthrough, we're going to use multiple terminal sessions/windows, all using `multicluster-gateway-controller` as the `pwd`.

Open three windows, which we'll refer to throughout this walkthrough as:

* `T1` (Hub Cluster)
* `T2` (Where we'll run our controller locally)
* `T3` (Workloads cluster)

To setup a local instance, in `T1`, run:

## Know bugs
buzzzzz

# Helpful symbols
:sos:
:exclamation:
* for more see https://gist.github.com/rxaviers/7360908