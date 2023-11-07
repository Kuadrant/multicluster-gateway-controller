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
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/utils"
)

const (
	singleCluster = "kudarant.io/single"
)

func (r *DNSPolicyReconciler) reconcileDNSRecords(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	log.V(3).Info("reconciling dns records")
	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(1).Info("reconcileDNSRecords: gateway with invalid policy ref", "key", gw.Key())
		if err := r.deleteGatewayDNSRecords(ctx, gw.Gateway, dnsPolicy); err != nil {
			return fmt.Errorf("error deleting dns records for gw %v: %w", gw.Gateway.Name, err)
		}
	}

	// Reconcile DNSRecords for each gateway directly referred by the policy (existing and new)
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(1).Info("reconcileDNSRecords: gateway with valid or missing policy ref", "key", gw.Key())
		if err := r.reconcileGatewayDNSRecords(ctx, gw.Gateway, dnsPolicy); err != nil {
			return fmt.Errorf("error reconciling dns records for gateway %v: %w", gw.Gateway.Name, err)
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) reconcileGatewayDNSRecords(ctx context.Context, gw *gatewayapiv1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	log := crlog.FromContext(ctx)

	gatewayWrapper, err := utils.NewGatewayWrapper(gw)
	if err != nil {
		return err
	}

	if err := r.dnsHelper.removeDNSForDeletedListeners(ctx, gatewayWrapper.Gateway); err != nil {
		log.V(3).Info("error removing DNS for deleted listeners")
		return err
	}

	clusterGatewayAddresses := getClusterGatewayAddresses(gatewayWrapper)

	log.V(3).Info("checking gateway for attached routes ", "gateway", gatewayWrapper.Name, "clusters", clusterGatewayAddresses)

	for _, listener := range gatewayWrapper.Spec.Listeners {
		var clusterGateways []dns.ClusterGateway
		var mz, err = r.dnsHelper.getManagedZoneForListener(ctx, gatewayWrapper.Namespace, listener)
		if err != nil {
			return err
		}
		listenerHost := *listener.Hostname
		if listenerHost == "" {
			log.Info("skipping listener no hostname assigned", listener.Name, "in ns ", gatewayWrapper.Namespace)
			continue
		}
		for clusterName, gatewayAddresses := range clusterGatewayAddresses {
			// Only consider host for dns if there's at least 1 attached route to the listener for this host in *any* gateway

			log.V(3).Info("checking downstream", "listener ", listener.Name)
			attached := listenerTotalAttachedRoutes(gatewayWrapper, clusterName, listener)

			if attached == 0 {
				log.V(1).Info("no attached routes for ", "listener", listener, "cluster ", clusterName)
				continue
			}
			log.V(3).Info("hostHasAttachedRoutes", "host", listener.Name, "hostHasAttachedRoutes", attached)

			cg, err := r.buildClusterGateway(ctx, clusterName, gatewayAddresses, gatewayWrapper.Gateway)
			if err != nil {
				return fmt.Errorf("get cluster gateway failed: %s", err)
			}

			clusterGateways = append(clusterGateways, cg)
		}

		if len(clusterGateways) == 0 {
			// delete record
			log.V(3).Info("no cluster gateways, deleting DNS record", " for listener ", listener.Name)
			if err := r.dnsHelper.deleteDNSRecordForListener(ctx, gatewayWrapper, listener); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete dns record for listener %s : %s", listener.Name, err)
			}
			return nil
		}
		dnsRecord, err := r.dnsHelper.createDNSRecordForListener(ctx, gatewayWrapper.Gateway, dnsPolicy, mz, listener)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return fmt.Errorf("failed to create dns record for listener host %s : %s ", *listener.Hostname, err)
		}
		if k8serrors.IsAlreadyExists(err) {
			dnsRecord, err = r.dnsHelper.getDNSRecordForListener(ctx, listener, gatewayWrapper)
			if err != nil {
				return fmt.Errorf("failed to get dns record for host %s : %s ", listener.Name, err)
			}
		}

		mcgTarget, err := dns.NewMultiClusterGatewayTarget(gatewayWrapper.Gateway, clusterGateways, dnsPolicy.Spec.LoadBalancing)
		if err != nil {
			return fmt.Errorf("failed to create multi cluster gateway target for listener %s : %s ", listener.Name, err)
		}

		log.Info("setting dns dnsTargets for gateway listener", "listener", dnsRecord.Name, "values", mcgTarget)
		probes, err := r.dnsHelper.getDNSHealthCheckProbes(ctx, mcgTarget.Gateway, dnsPolicy)
		if err != nil {
			return err
		}
		mcgTarget.RemoveUnhealthyGatewayAddresses(probes, listener)
		if err := r.dnsHelper.setEndpoints(ctx, mcgTarget, dnsRecord, listener, dnsPolicy.Spec.RoutingStrategy); err != nil {
			return fmt.Errorf("failed to add dns record dnsTargets %s %v", err, mcgTarget)
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) deleteGatewayDNSRecords(ctx context.Context, gateway *gatewayapiv1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	return r.deleteDNSRecordsWithLabels(ctx, commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteDNSRecords(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy) error {
	return r.deleteDNSRecordsWithLabels(ctx, policyDNSRecordLabels(client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteDNSRecordsWithLabels(ctx context.Context, lbls map[string]string, namespace string) error {
	log := crlog.FromContext(ctx)

	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(lbls), Namespace: namespace}
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

func (r *DNSPolicyReconciler) buildClusterGateway(ctx context.Context, clusterName string, gatewayAddresses []gatewayapiv1.GatewayAddress, targetGW *gatewayapiv1.Gateway) (dns.ClusterGateway, error) {
	var target dns.ClusterGateway
	singleClusterAddresses := make([]gatewayapiv1.GatewayAddress, len(gatewayAddresses))

	var metaObj client.Object
	if clusterName != singleCluster {
		mc := &clusterv1.ManagedCluster{}
		if err := r.Client().Get(ctx, client.ObjectKey{Name: clusterName}, mc, &client.GetOptions{}); err != nil {
			return target, err
		}
		metaObj = mc
	} else {
		metaObj = targetGW
	}

	for i, addr := range gatewayAddresses {
		addrType, multicluster := utils.AddressTypeToSingleCluster(addr)

		if !multicluster {
			addrType = *addr.Type
		}

		singleClusterAddresses[i] = gatewayapiv1.GatewayAddress{
			Type:  &addrType,
			Value: addr.Value,
		}
	}
	target = *dns.NewClusterGateway(metaObj, singleClusterAddresses)

	return target, nil
}

func getClusterGatewayAddresses(gw *utils.GatewayWrapper) map[string][]gatewayapiv1.GatewayAddress {
	clusterAddrs := make(map[string][]gatewayapiv1.GatewayAddress, len(gw.Status.Addresses))

	for _, address := range gw.Status.Addresses {
		//Default to Single Cluster (Normal Gateway Status)
		cluster := singleCluster
		addressValue := address.Value

		//Check for Multi Cluster (MGC Gateway Status)
		if gw.IsMultiCluster() {
			tmpCluster, tmpAddress, found := strings.Cut(address.Value, "/")
			//If this fails something is wrong and the value hasn't been set correctly
			if found {
				cluster = tmpCluster
				addressValue = tmpAddress
			}
		}

		if _, ok := clusterAddrs[cluster]; !ok {
			clusterAddrs[cluster] = []gatewayapiv1.GatewayAddress{}
		}

		clusterAddrs[cluster] = append(clusterAddrs[cluster], gatewayapiv1.GatewayAddress{
			Type:  address.Type,
			Value: addressValue,
		})
	}

	return clusterAddrs
}

func listenerTotalAttachedRoutes(upstreamGateway *utils.GatewayWrapper, downstreamCluster string, specListener gatewayv1beta1.Listener) int {
	for _, statusListener := range upstreamGateway.Status.Listeners {
		// for Multi Cluster (MGC Gateway Status)
		if upstreamGateway.IsMultiCluster() {
			clusterName, listenerName, found := strings.Cut(string(statusListener.Name), ".")
			if !found {
				return 0
			}
			if clusterName == downstreamCluster && listenerName == string(specListener.Name) {
				return int(statusListener.AttachedRoutes)
			}
		}
		// Single Cluster (Normal Gateway Status)
		if string(statusListener.Name) == string(specListener.Name) {
			return int(statusListener.AttachedRoutes)
		}
	}

	return 0
}
