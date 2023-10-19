# Usage

To use this kustomization file with an OSD/OCP cluster, first get a token for
accessing the thanos-query instance, and generate a grafana datasource from the 
template file

```shell
export SECRET=`oc get secret -n openshift-user-workload-monitoring | grep  prometheus-user-workload-token | head -n 1 | awk '{print $1 }'`
export TOKEN=`echo $(oc get secret $SECRET -n openshift-user-workload-monitoring -o json | jq -r '.data.token') | base64 -d`
envsubst < ./config/prometheus-for-federation/ocp_monitoring/grafana_datasources.yaml.template > ./config/prometheus-for-federation/ocp_monitoring/grafana_datasources.yaml
```

Then apply the resources to the cluster:

```shell
kustomize --load-restrictor LoadRestrictionsNone build ./config/prometheus-for-federation/ocp_monitoring/ --enable-helm | kubectl apply -f -
```

Access Grafana on the exposed route, user/pass is admin/admin by default:

```shell
oc get route grafana -n monitoring
```

# Troubleshooting

If metrics are missing, check the targets in the OCP UI under Observe > Targets.
Each 'User' target corresponds to a ServiceMonitor or PodMonitor detected by the
user-workload-monitoring prometheus operator.
If a target is missing, check the logs of the prometheus operator in the
openshift-user-workload-monitoring namespace.