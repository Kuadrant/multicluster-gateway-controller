package dnspolicy

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"golang.org/x/net/publicsuffix"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	DefaultTTL             = 60
	DefaultCnameTTL        = 300
	LabelGatewayReference  = "kuadrant.io/Gateway-uid"
	LabelListenerReference = "kuadrant.io/listener-name"
	LabelPolicyReference   = "kuadrant.io/policy-id"
)

var (
	ErrNoManagedZoneForHost = fmt.Errorf("no managed zone for host")
	ErrAlreadyAssigned      = fmt.Errorf("managed host already assigned")
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

// removeDNSForDeletedListeners remove any DNSRecords that are associated with listeners that no longer exist in this gateway
func (r *DNSPolicyReconciler) removeDNSForDeletedListeners(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway) error {
	log := crlog.FromContext(ctx)
	log.Info("DNSPolicyReconciler.removeDNSForDeletedListeners", "gateway", upstreamGateway.Name)
	dnsList := &v1alpha1.DNSRecordList{}
	labelSelector := &client.MatchingLabels{
		LabelGatewayReference: string(upstreamGateway.GetUID()),
	}
	if err := r.Client().List(ctx, dnsList, labelSelector); err != nil {
		return err
	}

	for _, dns := range dnsList.Items {
		listenerExists := false
		for _, listener := range upstreamGateway.Spec.Listeners {
			if listener.Name == gatewayv1beta1.SectionName(dns.Labels[LabelListenerReference]) {
				listenerExists = true
			}
		}
		if !listenerExists {
			if err := r.Client().Delete(ctx, &dns, &client.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil

}

func (r *DNSPolicyReconciler) reconcileGatewayDNSRecords(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	log := crlog.FromContext(ctx)
	if err := r.removeDNSForDeletedListeners(ctx, upstreamGateway); err != nil {
		return err
	}

	placed, err := r.Placement.GetPlacedClusters(ctx, upstreamGateway)
	if err != nil {
		return err
	}
	clusters := placed.UnsortedList()
	log.Info("DNSPolicyReconciler.reconcileResources", "clusters", clusters)

	log.V(3).Info("checking gateway for attached routes ", "gateway", upstreamGateway.Name, "clusters", placed)
	if len(placed) == 0 {
		//nothing to do
		log.V(3).Info("reconcileDNSRecords gateway has not been placed on to any downstream clusters nothing to do")
		return nil
	}

	for _, listener := range upstreamGateway.Spec.Listeners {
		var clusterGateways []dns.ClusterGateway
		var mz, err = r.getManagedZoneForListener(ctx, upstreamGateway.Namespace, listener)
		if err != nil {
			return err
		}
		for _, downstreamCluster := range clusters {
			log.V(3).Info("checking downstream", "listener ", listener.Name)
			attached, err := r.Placement.ListenerTotalAttachedRoutes(ctx, upstreamGateway, string(listener.Name), downstreamCluster)
			if err != nil {
				log.Error(err, "failed to get total attached routes for listener ", "listener", listener.Name)
				continue
			}
			if attached == 0 {
				log.V(3).Info("no attached routes for ", "listener", listener.Name, "cluster ", downstreamCluster)
				continue
			}
			cg, err := r.Placement.GetClusterGateway(ctx, upstreamGateway, downstreamCluster)
			if err != nil {
				return fmt.Errorf("get cluster gateway failed: %s", err)
			}
			clusterGateways = append(clusterGateways, cg)
			if len(clusterGateways) == 0 {
				// delete record
				log.V(3).Info("no cluster gateways, deleting DNS record", " for gateway ", upstreamGateway.Name, "and listener", listener.Name)
				if err := r.deleteDNSRecordForListener(upstreamGateway.Namespace, listener); client.IgnoreNotFound(err) != nil {
					return fmt.Errorf("failed to deleted dns record for managed host %s : %s", listener.Name, err)
				}
				return nil
			}
		}
		dnsRecord, err := r.createListenerDNSRecord(ctx, dnsPolicy.GetObjectMeta().GetUID(), upstreamGateway, mz, listener)
		if err := client.IgnoreAlreadyExists(err); err != nil {
			return fmt.Errorf("failed to create dns record for listener %s : %s ", listener.Name, err)
		}
		if k8serrors.IsAlreadyExists(err) {
			if err := r.Client().Get(ctx, client.ObjectKeyFromObject(dnsRecord), dnsRecord); err != nil {
				return fmt.Errorf("failed to get dns record for listener %s : %s ", listener.Name, err)
			}
		}

		mcgTarget := dns.NewMultiClusterGatewayTarget(upstreamGateway, clusterGateways, dnsPolicy.Spec.LoadBalancing)
		log.Info("setting dns dnsTargets for gateway listener", "listener", listener.Name, "values", mcgTarget)

		if err := r.SetEndpoints(ctx, mcgTarget, dnsRecord, &listener); err != nil {
			return fmt.Errorf("failed to add dns record dnsTargets %s %v", err, mcgTarget)
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) SetEndpoints(ctx context.Context, mcgTarget *dns.MultiClusterGatewayTarget, dnsRecord *v1alpha1.DNSRecord, listener *gatewayv1beta1.Listener) error {

	old := dnsRecord.DeepCopy()
	gwListenerHost := string(*listener.Hostname)
	cnameHost := gwListenerHost
	if isWildCardListener(*listener) {
		fmt.Println("is a wildcard ***")
		cnameHost = strings.Replace(gwListenerHost, "*.", "", -1)
	}

	//Health Checks currently modify endpoints so we have to keep existing ones in order to not lose health check ids
	currentEndpoints := make(map[string]*v1alpha1.Endpoint, len(dnsRecord.Spec.Endpoints))
	for _, endpoint := range dnsRecord.Spec.Endpoints {
		currentEndpoints[endpoint.SetID()] = endpoint
	}

	var (
		newEndpoints []*v1alpha1.Endpoint
		endpoint     *v1alpha1.Endpoint
	)

	lbName := strings.ToLower(fmt.Sprintf("lb-%s.%s", mcgTarget.GetShortCode(), cnameHost))
	//Create gwListenerHost CNAME (shop.example.com -> lb-a1b2.shop.example.com)
	endpoint = createOrUpdateEndpoint(gwListenerHost, []string{lbName}, v1alpha1.CNAMERecordType, "", DefaultCnameTTL, currentEndpoints)
	newEndpoints = append(newEndpoints, endpoint)

	for geoCode, cgwTargets := range mcgTarget.GroupTargetsByGeo() {
		geoLbName := strings.ToLower(fmt.Sprintf("%s.%s", geoCode, lbName))
		//Create lbName CNAME (lb-a1b2.shop.example.com -> default.lb-a1b2.shop.example.com)
		endpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, string(geoCode), DefaultCnameTTL, currentEndpoints)
		newEndpoints = append(newEndpoints, endpoint)

		switch {
		case geoCode.IsDefaultCode():
			endpoint.SetProviderSpecific(dns.ProviderSpecificGeoCountryCode, "*")
		case geoCode.IsContinentCode():
			endpoint.SetProviderSpecific(dns.ProviderSpecificGeoContinentCode, string(geoCode))
		case geoCode.IsCountryCode():
			endpoint.SetProviderSpecific(dns.ProviderSpecificGeoCountryCode, string(geoCode))
		}

		//Create the default geo if this geo matches the default policy geo and we haven't just created it
		if !geoCode.IsDefaultCode() && geoCode == mcgTarget.GetDefaultGeo() {
			endpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, "default", DefaultCnameTTL, currentEndpoints)
			newEndpoints = append(newEndpoints, endpoint)
			endpoint.SetProviderSpecific(dns.ProviderSpecificGeoCountryCode, "*")
		}

		for _, cgwTarget := range cgwTargets {
			var ipValues []string
			var hostValues []string
			for _, gwa := range cgwTarget.GatewayAddresses {
				if *gwa.Type == gatewayv1beta1.IPAddressType {
					ipValues = append(ipValues, gwa.Value)
				} else {
					hostValues = append(hostValues, gwa.Value)
				}
			}

			if len(ipValues) > 0 {
				clusterLbName := strings.ToLower(fmt.Sprintf("%s.%s", cgwTarget.GetShortCode(), lbName))
				endpoint = createOrUpdateEndpoint(clusterLbName, ipValues, v1alpha1.ARecordType, "", DefaultTTL, currentEndpoints)
				newEndpoints = append(newEndpoints, endpoint)
				hostValues = append(hostValues, clusterLbName)
			}

			for _, hostValue := range hostValues {
				endpoint = createOrUpdateEndpoint(geoLbName, []string{hostValue}, v1alpha1.CNAMERecordType, hostValue, DefaultTTL, currentEndpoints)
				endpoint.SetProviderSpecific(dns.ProviderSpecificWeight, strconv.Itoa(cgwTarget.GetWeight()))
				newEndpoints = append(newEndpoints, endpoint)
			}

		}
	}

	dnsRecord.Spec.Endpoints = newEndpoints

	if equality.Semantic.DeepEqual(old.Spec, dnsRecord.Spec) {
		return nil
	}

	return r.Client().Update(ctx, dnsRecord, &client.UpdateOptions{})
}

func createOrUpdateEndpoint(dnsName string, targets v1alpha1.Targets, recordType v1alpha1.DNSRecordType, setIdentifier string,
	recordTTL v1alpha1.TTL, currentEndpoints map[string]*v1alpha1.Endpoint) (endpoint *v1alpha1.Endpoint) {
	ok := false
	endpointID := dnsName + setIdentifier
	if endpoint, ok = currentEndpoints[endpointID]; !ok {
		endpoint = &v1alpha1.Endpoint{}
		if setIdentifier != "" {
			endpoint.SetIdentifier = setIdentifier
		}
	}
	endpoint.DNSName = dnsName
	endpoint.RecordType = string(recordType)
	endpoint.Targets = targets
	endpoint.RecordTTL = recordTTL
	return endpoint
}

func dnsRecordName(owner metav1.Object, l gatewayv1beta1.Listener) string {
	return fmt.Sprintf("%s-%s", owner.GetName(), l.Name)
}

func (r *DNSPolicyReconciler) createListenerDNSRecord(ctx context.Context, policyID types.UID, owner metav1.Object, mz *v1alpha1.ManagedZone, listener gatewayv1beta1.Listener) (*v1alpha1.DNSRecord, error) {
	//managedHost := strings.ToLower(fmt.Sprintf("%s.%s", subDomain, managedZone.Spec.DomainName))

	log := crlog.FromContext(ctx)
	recordName := dnsRecordName(owner, listener)
	log.Info("creating dns for gateway listener", "listener", listener.Name)
	dnsRecord := v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recordName,
			Namespace: owner.GetNamespace(),
			Labels: map[string]string{
				LabelListenerReference: string(listener.Name),
				LabelGatewayReference:  string(owner.GetUID()),
				LabelPolicyReference:   string(policyID),
			},
		},
		Spec: v1alpha1.DNSRecordSpec{
			ManagedZoneRef: &v1alpha1.ManagedZoneReference{
				Name: mz.Name,
			},
		},
	}
	if err := controllerutil.SetOwnerReference(owner, &dnsRecord, r.Client().Scheme()); err != nil {
		return &dnsRecord, err
	}
	if err := controllerutil.SetControllerReference(mz, &dnsRecord, r.Client().Scheme()); err != nil {
		return &dnsRecord, err
	}

	err := r.Client().Create(ctx, &dnsRecord, &client.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return &dnsRecord, err
	}
	//host may already be present
	if err != nil && k8serrors.IsAlreadyExists(err) {
		err = r.Client().Get(ctx, client.ObjectKeyFromObject(&dnsRecord), &dnsRecord)
		if err != nil {
			return &dnsRecord, err
		}
	}
	return &dnsRecord, nil
}

func (r *DNSPolicyReconciler) getManagedZoneForListener(ctx context.Context, ns string, listener gatewayv1beta1.Listener) (*v1alpha1.ManagedZone, error) {
	var managedZones v1alpha1.ManagedZoneList
	if err := r.Client().List(ctx, &managedZones, client.InNamespace(ns)); err != nil {
		log.FromContext(ctx).Error(err, "unable to list managed zones for gateway ", "in ns", ns)
		return nil, err
	}
	host := string(*listener.Hostname)
	return FindMatchingManagedZone(host, host, managedZones.Items)
}

func (r *DNSPolicyReconciler) deleteDNSRecordForListener(namespace string, listner gatewayv1beta1.Listener) error {

	return nil

}

func (r *DNSPolicyReconciler) getDNSRecordForListener(ctx context.Context, namespace string, listener gatewayv1beta1.Listener) (*v1alpha1.DNSRecord, error) {
	dnsrecord := &v1alpha1.DNSRecordList{}
	labelSelector := &client.MatchingLabels{
		LabelListenerReference: string(listener.Name),
	}
	if err := r.Client().List(ctx, dnsrecord, labelSelector); err != nil {
		return nil, err
	}
	if len(dnsrecord.Items) == 0 {
		return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "kuadrant.io", Resource: "DNSRecord"}, fmt.Sprintf("failed to find dns record for listener %s", listener.Name))
	}
	if len(dnsrecord.Items) > 1 {
		return nil, fmt.Errorf("more than one dnsrecord found for a listener")
	}
	return &dnsrecord.Items[0], nil

}

func (r *DNSPolicyReconciler) getDNSRecordManagedZone(ctx context.Context, dnsRecord *v1alpha1.DNSRecord) (*v1alpha1.ManagedZone, error) {

	if dnsRecord.Spec.ManagedZoneRef == nil {
		return nil, fmt.Errorf("no managed zone configured for : %s", dnsRecord.Name)
	}

	managedZone := &v1alpha1.ManagedZone{}

	err := r.Client().Get(ctx, client.ObjectKey{Namespace: dnsRecord.Namespace, Name: dnsRecord.Spec.ManagedZoneRef.Name}, managedZone)
	if err != nil {
		return nil, err
	}

	return managedZone, nil
}

// FindMatchingManagedZone recursively looks for a matching zone in the same ns as the gateway
func FindMatchingManagedZone(originalHost, host string, zones []v1alpha1.ManagedZone) (*v1alpha1.ManagedZone, error) {
	if len(zones) == 0 {
		return nil, fmt.Errorf("%w : %s", ErrNoManagedZoneForHost, host)
	}
	host = strings.ToLower(host)
	//get the TLD from this host
	tld, _ := publicsuffix.PublicSuffix(host)

	//The host is just the TLD, or the detected TLD is not an ICANN TLD
	if host == tld {
		return nil, fmt.Errorf("no valid zone found for host: %v", originalHost)
	}

	hostParts := strings.SplitN(host, ".", 2)
	if len(hostParts) < 2 {
		return nil, fmt.Errorf("no valid zone found for host: %s", originalHost)
	}

	zone, ok := slice.Find(zones, func(zone v1alpha1.ManagedZone) bool {
		return strings.ToLower(zone.Spec.DomainName) == host
	})

	if ok {
		return &zone, nil
	}
	parentDomain := hostParts[1]
	return FindMatchingManagedZone(originalHost, parentDomain, zones)

}

func isWildCardListener(l gatewayv1beta1.Listener) bool {
	return strings.HasPrefix(string(*l.Hostname), "*")
}
