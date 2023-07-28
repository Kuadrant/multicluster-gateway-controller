package dnspolicy

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
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

	gatewayAccessor := traffic.NewGateway(gateway)

	managedHosts, err := r.dnsHelper.getManagedHosts(ctx, gateway, dnsPolicy)
	if err != nil {
		return err
	}

	placed, err := r.Placement.GetPlacedClusters(ctx, gateway)
	if err != nil {
		return err
	}
	clusters := placed.UnsortedList()
	log.Info("DNSPolicyReconciler.reconcileResources", "clusters", clusters, "managedHosts", managedHosts)

	log.V(3).Info("checking gateway for attached routes ", "gateway", gateway.Name, "clusters", placed, "managed hosts", len(managedHosts))

	for _, mh := range managedHosts {
		listener := gatewayAccessor.GetListenerByHost(mh.Host)
		if listener == nil {
			log.V(3).Info("no downstream listener found", "host ", mh.Host)
			continue
		}
		var clusterGateways []dns.ClusterGateway
		for _, downstreamCluster := range clusters {
			// Only consider host for dns if there's at least 1 attached route to the listener for this host in *any* gateway

			log.V(3).Info("checking downstream", "listener ", listener.Name)
			attached, err := r.Placement.ListenerTotalAttachedRoutes(ctx, gateway, string(listener.Name), downstreamCluster)
			if err != nil {
				log.Error(err, "failed to get total attached routes for listener ", "listener", listener.Name)
				continue
			}
			if attached == 0 {
				log.V(3).Info("no attached routes for ", "listener", listener.Name, "cluster ", downstreamCluster)
				continue
			}
			log.V(3).Info("hostHasAttachedRoutes", "host", mh.Host, "hostHasAttachedRoutes", attached)
			cg, err := r.Placement.GetClusterGateway(ctx, gateway, downstreamCluster)
			if err != nil {
				return fmt.Errorf("get cluster gateway failed: %s", err)
			}
			clusterGateways = append(clusterGateways, cg)
		}

		if len(clusterGateways) == 0 {
			// delete record
			log.V(3).Info("no cluster gateways, deleting DNS record", " for host ", mh.Host)
			if mh.DnsRecord != nil {
				if err := r.Client().Delete(ctx, mh.DnsRecord); client.IgnoreNotFound(err) != nil {
					return fmt.Errorf("failed to deleted dns record for managed host %s : %s", mh.Host, err)
				}
			}
			return nil
		}
		var dnsRecord, err = r.dnsHelper.createDNSRecord(ctx, gateway, dnsPolicy, mh.Subdomain, mh.ManagedZone)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return fmt.Errorf("failed to create dns record for host %s : %s ", mh.Host, err)
		}
		if k8serrors.IsAlreadyExists(err) {
			dnsRecord, err = r.dnsHelper.getDNSRecord(ctx, gateway, dnsPolicy, mh.Subdomain, mh.ManagedZone)
			if err != nil {
				return fmt.Errorf("failed to get dns record for host %s : %s ", mh.Host, err)
			}
		}

		mcgTarget := dns.NewMultiClusterGatewayTarget(gateway, clusterGateways, dnsPolicy.Spec.LoadBalancing)
		log.Info("setting dns dnsTargets for gateway listener", "listener", dnsRecord.Name, "values", mcgTarget)

		if err := r.dnsHelper.setEndpoints(ctx, mcgTarget, dnsRecord, listener); err != nil {
			return fmt.Errorf("failed to add dns record dnsTargets %s %v", err, mcgTarget)
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) deleteGatewayDNSRecords(ctx context.Context, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	log := crlog.FromContext(ctx)

	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(dnsRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)))}
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
