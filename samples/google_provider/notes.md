
make local-setup OCM_SINGLE=true MGC_WORKLOAD_CLUSTERS_COUNT=1
(export $(cat ./controller-config.env | xargs) && export $(cat ./aws-credentials.env | xargs) && make build-controller install run-controller)
./scratch/kuadrant_dnspolicy/test.sh
./scratch/kuadrant_dnspolicy/test_cleanup.sh




Install gcloud cli:

https://cloud.google.com/sdk/docs/install#rpm

#### ADC (Application Default Credentials)

https://cloud.google.com/docs/authentication/application-default-credentials

```bash
gcloud auth application-default login
export GOOGLE_APPLICATION_CREDENTIALS=/home/mnairn/.config/gcloud/application_default_credentials.json
```

#### Service account

```bash
DNS_SA_NAME="external-dns-sa"
DNS_SA_EMAIL="$DNS_SA_NAME@${GKE_PROJECT_ID}.iam.gserviceaccount.com"

# create GSA used to access the Cloud DNS zone
gcloud iam service-accounts create $DNS_SA_NAME --display-name $DNS_SA_NAME

# assign google service account to dns.admin role in cloud-dns project
gcloud projects add-iam-policy-binding $DNS_PROJECT_ID \
  --member serviceAccount:$DNS_SA_EMAIL --role "roles/dns.admin"
```

Create ManagedZone

```bash
kubectl create secret generic mgc-google-credentials --type=kuadrant.io/google --from-file=GOOGLE=/home/mnairn/.config/gcloud/application_default_credentials.json --from-literal=PROJECT_ID=it-cloud-gcp-rd-midd-san -n multi-cluster-gateways
kubectl apply -f samples/google_provider/mn.google.hcpapps.net-managedzone.yaml -n multi-cluster-gateways
kubectl get secrets -n multi-cluster-gateways
NAME                     TYPE                 DATA   AGE
mgc-aws-credentials      kuadrant.io/aws      3      25h
mgc-google-credentials   kuadrant.io/google   1      85s
kubectl get managedzones -n multi-cluster-gateways
NAME                    DOMAIN NAME             ID                                  RECORD COUNT   NAMESERVERS                                                                                                                             READY
mgc-dev-mz              mn.hcpapps.net          /hostedzone/Z04114632NOABXYWH93QU   8              ["ns-2005.awsdns-58.co.uk","ns-627.awsdns-14.net","ns-1160.awsdns-17.org","ns-263.awsdns-32.com"]                                       True
mn.google.hcpapps.net   mn.google.hcpapps.net   mn-google-hcpapps-net               -1             ["ns-cloud-e4.googledomains.com.","ns-cloud-e4.googledomains.com.","ns-cloud-e4.googledomains.com.","ns-cloud-e4.googledomains.com."]   True
```

Create DNSRecord

```bash
kubectl apply -f samples/google_provider/geo.mn.google.hcpapps.net-dnsrecord.yaml -n multi-cluster-gateways
```

Create ManagedZone with parent
```bash
kubectl apply -f samples/google_provider/test1.mn.google.hcpapps.net-managedzone.yaml -n multi-cluster-gateways
```

Create DNSRecord

```bash
kubectl apply -f samples/google_provider/myapp.test1.mn.google.hcpapps.net-dnsrecord.yaml -n multi-cluster-gateways
```

```bash
kubectl tree managedzones mn.google.hcpapps.net -n multi-cluster-gateways
```

Delete Parent ManagedZone
```bash
kubectl delete managedzone mn.google.hcpapps.net -n multi-cluster-gateways --cascade=foreground
```