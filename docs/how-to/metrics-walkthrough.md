## Introduction
This document will guide you in installing metrics for your application and provide directions on where to access them. Additionally, it will include dashboards set up to display these metrics. 

## Requirements

- [Docker](https://docs.docker.com/engine/install/)
- [Kind](https://kind.sigs.k8s.io/)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/)
- AWS account with Route 53 enabled

## Installation and Setup
* Export env-vars with the keys listed below. Fill in your own values as appropriate:

  | Env Var                      | Example Value               | Description                                                    |
  |------------------------------|-----------------------------|----------------------------------------------------------------|
  | `MGC_ZONE_ROOT_DOMAIN`       | `jbloggs.hcpapps.net`       | Hostname for the root Domain                                   |
  | `MGC_AWS_DNS_PUBLIC_ZONE_ID` | `Z01234567US0IQE3YLO00`     | AWS Route 53 Zone ID for specified `MGC_ZONE_ROOT_DOMAIN`      | 
  | `MGC_AWS_ACCESS_KEY_ID`      | `AKIA1234567890000000`      | Access Key ID, with access to resources in Route 53            |
  | `MGC_AWS_SECRET_ACCESS_KEY`  | `Z01234567US0000000`        | Access Secret Access Key, with access to resources in Route 53 |
  | `MGC_AWS_REGION`             | `eu-west-1`                 | AWS Region                                                     |
  | `MGC_SUB_DOMAIN`             | `myapp.jbloggs.hcpapps.net` | AWS Region                                                     |

* Alternatively, to set defaults, add the above environment variables to your `.zshrc` or `.bash_profile`. 

## Setting Up Metrics

To establish metrics, simply execute the following script in your terminal:

```bash
    curl https://raw.githubusercontent.com/kuadrant/multicluster-gateway-controller/main/hack/quickstart-metrics.sh | bash
```

This script will initiate the setup process for your metrics configuration.
After the script finishes running, you should see something like:

```
Connect to Thanos Query UI
    URL: https://thanos-query.172.31.0.2.nip.io

Connect to Grafana UI
    URL: https://grafana.172.31.0.2.nip.io
```

You can visit the Grafana dashboard by accessing the provided URL for Grafana UI.

## Viewing Operational Status in Grafana Dashboard

If you are continuing from the previous step, you can monitor the operational status of your system by utilizing the Grafana dashboard.

### Accessing the Grafana Dashboard
To view the operational metrics and status, proceed with the following steps:

Access the Grafana dashboard by clicking or entering the provided URL for the Grafana UI in your web browser.

The Grafana dashboard will provide you with real-time insights and visualizations of your gateway's performance and metrics.

By utilizing the Grafana dashboard, you can effectively monitor the health and behavior of your system, making informed decisions based on the displayed data. This monitoring capability enables you to proactively identify and address any potential issues to ensure the smooth operation of your environment.
