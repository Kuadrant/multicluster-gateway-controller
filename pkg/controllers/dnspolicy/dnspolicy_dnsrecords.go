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

	clusterAddresses := getClusterGatewayAddresses(gateway)

	log.V(3).Info("checking gateway for attached routes ", "gateway", gateway.Name, "clusters", clusterAddresses)

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
		for clusterName, clusterAddress := range clusterAddresses {
			// Only consider host for dns if there's at least 1 attached route to the listener for this host in *any* gateway

			log.V(3).Info("checking downstream", "listener ", listener.Name)
			attached := listenerTotalAttachedRoutes(gateway, clusterName, string(listener.Name), clusterAddress)

			if attached == 0 {
				log.V(1).Info("no attached routes for ", "listener", listener.Name, "cluster ", clusterName)
				continue
			}
			log.V(3).Info("hostHasAttachedRoutes", "host", listener.Name, "hostHasAttachedRoutes", attached)

			cg, err := r.buildClusterGateway(ctx, clusterName, clusterAddress)
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

func (r *DNSPolicyReconciler) buildClusterGateway(ctx context.Context, downstreamClusterName string, clusterAddress []gatewayv1beta1.GatewayAddress) (dns.ClusterGateway, error) {
	var target dns.ClusterGateway

	mc := &clusterv1.ManagedCluster{}
	if err := r.Client().Get(ctx, client.ObjectKey{Name: downstreamClusterName}, mc, &client.GetOptions{}); err != nil {
		return target, err
	}

	for _, addr := range clusterAddress {
		addrType := gatewayv1beta1.IPAddressType
		if *addr.Type == gateway.MultiClusterHostnameAddressType {
			addrType = gatewayv1beta1.HostnameAddressType
		}
		addr.Type = &addrType
	}

	target = *dns.NewClusterGateway(mc, clusterAddress)

	return target, nil
}

func getClusterGatewayAddresses(gw *gatewayv1beta1.Gateway) map[string][]gatewayv1beta1.GatewayAddress {
	clusterAddrs := make(map[string][]gatewayv1beta1.GatewayAddress, len(gw.Status.Addresses))

	for _, address := range gw.Status.Addresses {
		var gatewayAddresses []gatewayv1beta1.GatewayAddress

		//addressType := gatewayv1beta1.IPAddressType
		//if *address.Type == gateway.MultiClusterHostnameAddressType {
		//	addressType = gatewayv1beta1.HostnameAddressType
		//}

		//Default to Single Cluster (Normal Gateway Status)
		cluster := "none"
		addressValue := address.Value

		//Check for Multi Cluster (MGC Gateway Status)
		if *address.Type == gateway.MultiClusterIPAddressType || *address.Type == gateway.MultiClusterHostnameAddressType {
			tmpCluster, tmpAddress, found := strings.Cut(address.Value, "/")
			//If this fails something is wrong and the value hasn't been set correctly
			if found {
				cluster = tmpCluster
				addressValue = tmpAddress
			}
		}

		gatewayAddresses = append(gatewayAddresses, gatewayv1beta1.GatewayAddress{
			Type:  address.Type,
			Value: addressValue,
		})
		clusterAddrs[cluster] = gatewayAddresses
	}

	return clusterAddrs
}

func listenerTotalAttachedRoutes(upstreamGateway *gatewayv1beta1.Gateway, downstreamCluster, specListenerName string, addresses []gatewayv1beta1.GatewayAddress) int {
	for _, listener := range upstreamGateway.Status.Listeners {
		// assuming all adresses of the same type on the gateway
		// for Multi Cluster (MGC Gateway Status)
		if *addresses[0].Type == gateway.MultiClusterIPAddressType || *addresses[0].Type == gateway.MultiClusterHostnameAddressType {
			clusterName, listenerName, found := strings.Cut(string(listener.Name), ".")
			if !found {
				return 0
			}
			if clusterName == downstreamCluster && (listenerName == specListenerName || specListenerName == "wildcard") {
				return int(listener.AttachedRoutes)
			}
		}
		// Single Cluster (Normal Gateway Status)
		if string(listener.Name) == specListenerName {
			return int(listener.AttachedRoutes)
		}
	}

	return 0
}
