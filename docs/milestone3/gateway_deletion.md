## Gateway deletion

When a gateway is marked for deletion in the control plane the following should happen:

### Workload clusters:
1. The corresponding gateway in the workload clusters are also deleted.

### Control plane clusters:
2. **DNS Record deletion**:

    Gateways and DNS records have a 1:1 relationship, when the gateway gets marked for deleted the corresponding DNS record also gets marked for deletion. This then triggers the DNS record to be removed from the managed zone in the DNS provider (currently only route 53 is accepted).
3. **Certs and secrets**:

    When a gateway is created a cert is also created for the host in the gateway, this is also removed when the gateway is deleted.