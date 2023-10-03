package dnspolicy

import (
	"context"
	"fmt"
	"strings"

	clusterv1 "open-cluster-management.io/api/cluster/v1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

func (r *DNSPolicyReconciler) reconcileDNSRecords(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(1).Info("reconcileDNSRecords: gateway with invalid policy ref", "key", gw.Key())
		err := r.deleteGatewayDNSRecords(ctx, gw.Gateway, dnsPolicy)
		if err != nil {
			return err
		}
	}

	// Reconcile DNSRecords for each gateway directly referred by the policy (existing and new)
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(1).Info("reconcileDNSRecords: gateway with valid and missing policy ref", "key", gw.Key())
		err := r.reconcileGatewayDNSRecords(ctx, gw.Gateway, dnsPolicy)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *DNSPolicyReconciler) reconcileGatewayDNSRecords(ctx context.Context, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	log := crlog.FromContext(ctx)

	if err := r.dnsHelper.removeDNSForDeletedListeners(ctx, gateway); err != nil {
		log.V(3).Info("error removing DNS for deleted listeners")
		return err
	}

	clusters := getGatewayAddresses(gateway)

	log.V(3).Info("checking gateway for attached routes ", "gateway", gateway.Name, "clusters", clusters)

	for _, listener := range gateway.Spec.Listeners {
		var clusterGateways []dns.ClusterGateway
		var mz, err = r.dnsHelper.getManagedZoneForListener(ctx, gateway.Namespace, listener)
		if err != nil {
			return err
		}
		listenerHost := *listener.Hostname
		if listenerHost == "" {
			log.Info("skipping listener no hostname assigned", listener.Name, "in ns ", gateway.Namespace)
			continue
		}
		for _, downstreamCluster := range clusters {
			// Only consider host for dns if there's at least 1 attached route to the listener for this host in *any* gateway

			log.V(3).Info("checking downstream", "listener ", listener.Name)
			attached := listenerTotalAttachedRoutes(gateway, downstreamCluster)

			if attached == 0 {
				log.V(1).Info("no attached routes for ", "listener", listener.Name, "cluster ", downstreamCluster)
				continue
			}
			log.V(3).Info("hostHasAttachedRoutes", "host", listener.Name, "hostHasAttachedRoutes", attached)

			cg, err := r.buildClusterGateway(ctx, gateway, downstreamCluster)
			if err != nil {
				return fmt.Errorf("get cluster gateway failed: %s", err)
			}

			clusterGateways = append(clusterGateways, cg)
		}

		if len(clusterGateways) == 0 {
			// delete record
			log.V(3).Info("no cluster gateways, deleting DNS record", " for listener ", listener.Name)
			if err := r.dnsHelper.deleteDNSRecordForListener(ctx, gateway, listener); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete dns record for listener %s : %s", listener.Name, err)
			}
			return nil
		}
		dnsRecord, err := r.dnsHelper.createDNSRecordForListener(ctx, gateway, dnsPolicy, mz, listener)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return fmt.Errorf("failed to create dns record for listener host %s : %s ", *listener.Hostname, err)
		}
		if k8serrors.IsAlreadyExists(err) {
			dnsRecord, err = r.dnsHelper.getDNSRecordForListener(ctx, listener, gateway)
			if err != nil {
				return fmt.Errorf("failed to get dns record for host %s : %s ", listener.Name, err)
			}
		}

		mcgTarget, err := dns.NewMultiClusterGatewayTarget(gateway, clusterGateways, dnsPolicy.Spec.LoadBalancing)
		if err != nil {
			return fmt.Errorf("failed to create multi cluster gateway target for listener %s : %s ", listener.Name, err)
		}

		log.Info("setting dns dnsTargets for gateway listener", "listener", dnsRecord.Name, "values", mcgTarget)
		probes, err := r.dnsHelper.getDNSHealthCheckProbes(ctx, mcgTarget.Gateway, dnsPolicy)
		if err != nil {
			return err
		}
		mcgTarget.RemoveUnhealthyGatewayAddresses(probes, listener)
		if err := r.dnsHelper.setEndpoints(ctx, mcgTarget, dnsRecord, listener); err != nil {
			return fmt.Errorf("failed to add dns record dnsTargets %s %v", err, mcgTarget)
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) deleteGatewayDNSRecords(ctx context.Context, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	log := crlog.FromContext(ctx)

	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)))}
	recordsList := &v1alpha1.DNSRecordList{}
	if err := r.Client().List(ctx, recordsList, listOptions); err != nil {
		return err
	}

	for _, record := range recordsList.Items {
		if err := r.DeleteResource(ctx, &record); client.IgnoreNotFound(err) != nil {
			log.Error(err, "failed to delete DNSRecord")
			return err
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) buildClusterGateway(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway, downstreamCluster string) (dns.ClusterGateway, error) {
	var target dns.ClusterGateway

	mc := &clusterv1.ManagedCluster{}
	if err := r.Client().Get(ctx, client.ObjectKey{Name: downstreamCluster}, mc, &client.GetOptions{}); err != nil {
		return target, err
	}

	for _, address := range upstreamGateway.Status.Addresses {
		if strings.Contains(address.Value, downstreamCluster) {
			var gatewayAddresses []gatewayv1beta1.GatewayAddress

			addressType := gatewayv1beta1.IPAddressType
			if strings.Contains(string(*address.Type), string(gateway.MultiClusterHostnameAddressType)) {
				addressType = gatewayv1beta1.HostnameAddressType
			}

			tmp := strings.Split(address.Value, "/")
			addressValue := tmp[0]

			if len(tmp) > 0 {
				addressValue = tmp[len(tmp)-1]
			}
			gatewayAddresses = append(gatewayAddresses, gatewayv1beta1.GatewayAddress{
				Type:  &addressType,
				Value: addressValue,
			})
			target = *dns.NewClusterGateway(mc, gatewayAddresses)
		}
	}
	return target, nil
}

func getGatewayAddresses(upstreamGateway *gatewayv1beta1.Gateway) []string {
	var clusters []string

	for _, address := range upstreamGateway.Status.Addresses {
		value := strings.Split(address.Value, "/")
		if strings.Contains(string(*address.Type), string(gateway.MultiClusterHostnameAddressType)) || strings.Contains(string(*address.Type), string(gateway.MultiClusterIPAddressType)) {
			clusters = append(clusters, value[0])
		}
	}

	return clusters
}

func listenerTotalAttachedRoutes(upstreamGateway *gatewayv1beta1.Gateway, downstreamCluster string) int {
	listeners := 0

	for _, listener := range upstreamGateway.Status.Listeners {
		if strings.Contains(string(listener.Name), downstreamCluster) {
			listeners = int(listener.AttachedRoutes)
		}
	}

	return listeners
}
