package dnspolicy

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/utils"
)

func (r *DNSPolicyReconciler) reconcileDNSRecords(ctx context.Context, dnsPolicy *v1alpha2.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
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

func (r *DNSPolicyReconciler) reconcileGatewayDNSRecords(ctx context.Context, gateway *gatewayapiv1.Gateway, dnsPolicy *v1alpha2.DNSPolicy) error {
	log := crlog.FromContext(ctx)

	gw := utils.NewGatewayWrapper(gateway)
	if err := gw.Validate(); err != nil {
		return err
	}

	if err := r.dnsHelper.removeDNSForDeletedListeners(ctx, gw.Gateway); err != nil {
		log.V(3).Info("error removing DNS for deleted listeners")
		return err
	}

	clusterGateways := gw.GetClusterGateways()

	dnsProvider, err := r.DNSProvider(ctx, dnsPolicy)
	if err != nil {
		return err
	}
	zoneList, err := dnsProvider.ListZones()
	if err != nil {
		return err
	}
	log.V(1).Info("got zones", "zoneList", zoneList)

	for _, listener := range gw.Spec.Listeners {
		listenerHost := *listener.Hostname
		if listenerHost == "" {
			log.Info("skipping listener no hostname assigned", listener.Name, "in ns ", gw.Namespace)
			continue
		}

		var zone *dns.Zone
		zone, _, err = findMatchingZone(string(listenerHost), string(listenerHost), zoneList)
		if err != nil {
			log.V(1).Info("skipping listener no matching zone for host", "listenerHost", listenerHost)
			continue
		}
		log.V(1).Info("found zone for listener host", "zone", zone, "listenerHost", listenerHost)

		listenerGateways := slice.Filter(clusterGateways, func(cgw utils.ClusterGateway) bool {
			hasAttachedRoute := false
			for _, statusListener := range cgw.Status.Listeners {
				if string(statusListener.Name) == string(listener.Name) {
					hasAttachedRoute = int(statusListener.AttachedRoutes) > 0
					break
				}
			}
			return hasAttachedRoute
		})

		if len(listenerGateways) == 0 {
			// delete record
			log.V(1).Info("no cluster gateways, deleting DNS record", " for listener ", listener.Name)
			if err := r.dnsHelper.deleteDNSRecordForListener(ctx, gw, listener); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete dns record for listener %s : %s", listener.Name, err)
			}
			return nil
		}
		dnsRecord, err := r.dnsHelper.createDNSRecordForListener(ctx, gw.Gateway, dnsPolicy, listener, zone)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return fmt.Errorf("failed to create dns record for listener host %s : %s ", *listener.Hostname, err)
		}
		if k8serrors.IsAlreadyExists(err) {
			dnsRecord, err = r.dnsHelper.getDNSRecordForListener(ctx, listener, gw)
			if err != nil {
				return fmt.Errorf("failed to get dns record for host %s : %s ", listener.Name, err)
			}
		}

		mcgTarget, err := dns.NewMultiClusterGatewayTarget(gw.Gateway, listenerGateways, dnsPolicy.Spec.LoadBalancing)
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

func (r *DNSPolicyReconciler) deleteGatewayDNSRecords(ctx context.Context, gateway *gatewayapiv1.Gateway, dnsPolicy *v1alpha2.DNSPolicy) error {
	return r.deleteDNSRecordsWithLabels(ctx, commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteDNSRecords(ctx context.Context, dnsPolicy *v1alpha2.DNSPolicy) error {
	return r.deleteDNSRecordsWithLabels(ctx, policyDNSRecordLabels(client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteDNSRecordsWithLabels(ctx context.Context, lbls map[string]string, namespace string) error {
	log := crlog.FromContext(ctx)

	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(lbls), Namespace: namespace}
	recordsList := &v1alpha2.DNSRecordList{}
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
