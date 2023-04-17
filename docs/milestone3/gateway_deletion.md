## Gateway deletion

When deleting a gateway it should ***ONLY*** be deleted in the control plane cluster. This will the trigger the following events:

### Workload cluster(s):
1. The corresponding gateway in the workload clusters will also be deleted.

### Control plane cluster(s):
2. **DNS Record deletion**:

    Gateways and DNS records have a 1:1 relationship ***only***, when a gateway gets deleted the corresponding DNS record also gets marked for deletion. This then triggers the DNS record to be removed from the managed zone in the DNS provider (currently only route 53 is accepted).
3. **Certs and secrets deletion **:

    When a gateway is created a cert is also created for the host in the gateway, this is also removed when the gateway is deleted.