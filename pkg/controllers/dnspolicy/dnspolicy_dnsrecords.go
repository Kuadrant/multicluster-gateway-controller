package dnspolicy

import (
	"context"
	"fmt"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
)

func (r *DNSPolicyReconciler) reconcileDNSRecords(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(1).Info("reconcileDNSRecords: gateway with invalid policy ref", "key", gw.Key())
		//ToDo Since gateways own DNSRecords I don't think there is anything to do here?
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

	managedHosts, err := r.HostService.GetManagedHosts(ctx, gatewayAccessor)
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
	if len(placed) == 0 {
		//nothing to do
		log.V(3).Info("reconcileDNSRecords gateway has not been placed on to any downstream clusters nothing to do")
		return nil
	}

	for _, mh := range managedHosts {
		gwAddresses := []gatewayv1beta1.GatewayAddress{}
		for _, downstreamCluster := range clusters {
			// Only consider host for dns if there's at least 1 attached route to the listener for this host in *any* gateway
			listener := gatewayAccessor.GetListenerByHost(mh.Host)
			if listener == nil {
				log.V(3).Info("no downstream listener found", "host ", mh.Host)
				continue
			}
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
			addresses, err := r.Placement.GetAddresses(ctx, gateway, downstreamCluster)
			if err != nil {
				return fmt.Errorf("get addresses failed: %s", err)
			}
			gwAddresses = append(gwAddresses, addresses...)
		}

		if len(gwAddresses) == 0 {
			// delete record
			log.V(3).Info("no endpoints deleting DNS record", " for host ", mh.Host)
			if mh.DnsRecord != nil {
				if err := r.Client().Delete(ctx, mh.DnsRecord); client.IgnoreNotFound(err) != nil {
					return fmt.Errorf("failed to deleted dns record for managed host %s : %s", mh.Host, err)
				}
			}
			return nil
		}
		var dnsRecord, err = r.HostService.CreateDNSRecord(ctx, mh.Subdomain, mh.ManagedZone, gateway)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return fmt.Errorf("failed to create dns record for host %s : %s ", mh.Host, err)
		}
		if k8serrors.IsAlreadyExists(err) {
			dnsRecord, err = r.HostService.GetDNSRecord(ctx, mh.Subdomain, mh.ManagedZone, gateway)
			if err != nil {
				return fmt.Errorf("failed to get dns record for host %s : %s ", mh.Host, err)
			}
		}

		log.Info("setting dns gwAddresses for gateway listener", "listener", dnsRecord.Name, "values", gwAddresses)

		if err := r.HostService.SetEndpoints(ctx, gwAddresses, dnsRecord, dnsPolicy); err != nil {
			return fmt.Errorf("failed to add dns record gwAddresses %s %v", err, gwAddresses)
		}

	}
	return nil
}
