package dnspolicy

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/publicsuffix"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	LabelGatewayReference  = "kuadrant.io/gateway"
	LabelGatewayNSRef      = "kuadrant.io/gateway-namespace"
	LabelListenerReference = "kuadrant.io/listener-name"
	LabelPolicyReference   = "kuadrant.io/policy-id"
)

var (
	ErrNoManagedZoneForHost = fmt.Errorf("no managed zone for host")
	ErrAlreadyAssigned      = fmt.Errorf("managed host already assigned")
)

type dnsHelper struct {
	client.Client
}

func findMatchingManagedZone(originalHost, host string, zones []v1alpha1.ManagedZone) (*v1alpha1.ManagedZone, string, error) {
	if len(zones) == 0 {
		return nil, "", fmt.Errorf("%w : %s", ErrNoManagedZoneForHost, host)
	}
	host = strings.ToLower(host)
	//get the TLD from this host
	tld, _ := publicsuffix.PublicSuffix(host)

	//The host is a TLD, so we now know `originalHost` can't possibly have a valid `ManagedZone` available.
	if host == tld {
		return nil, "", fmt.Errorf("no valid zone found for host: %v", originalHost)
	}

	hostParts := strings.SplitN(host, ".", 2)
	if len(hostParts) < 2 {
		return nil, "", fmt.Errorf("no valid zone found for host: %s", originalHost)
	}
	parentDomain := hostParts[1]

	// We do not currently support creating records for Apex domains, and a ManagedZone represents an Apex domain, as such
	// we should never be trying to find a managed zone that matches the `originalHost` exactly. Instead, we just continue
	// on to the next possible valid host to try i.e. the parent domain.
	if host == originalHost {
		return findMatchingManagedZone(originalHost, parentDomain, zones)
	}

	zone, ok := slice.Find(zones, func(zone v1alpha1.ManagedZone) bool {
		return strings.ToLower(zone.Spec.DomainName) == host
	})

	if ok {
		subdomain := strings.Replace(strings.ToLower(originalHost), "."+strings.ToLower(zone.Spec.DomainName), "", 1)
		return &zone, subdomain, nil
	}
	return findMatchingManagedZone(originalHost, parentDomain, zones)

}

func commonDNSRecordLabels(gwKey, apKey client.ObjectKey) map[string]string {
	return map[string]string{
		DNSPolicyBackRefAnnotation:                              apKey.Name,
		fmt.Sprintf("%s-namespace", DNSPolicyBackRefAnnotation): apKey.Namespace,
		LabelGatewayNSRef:                                       gwKey.Namespace,
		LabelGatewayReference:                                   gwKey.Name,
	}
}

func (dh *dnsHelper) buildDNSRecordForListener(gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy, targetListener gatewayv1beta1.Listener, managedZone *v1alpha1.ManagedZone) *v1alpha1.DNSRecord {

	dnsRecord := &v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dnsRecordName(gateway.Name, string(targetListener.Name)),
			Namespace: managedZone.Namespace,
			Labels:    commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)),
		},
		Spec: v1alpha1.DNSRecordSpec{
			ManagedZoneRef: &v1alpha1.ManagedZoneReference{
				Name: managedZone.Name,
			},
		},
	}
	dnsRecord.Labels[LabelListenerReference] = string(targetListener.Name)
	return dnsRecord
}

// getDNSRecordForListener returns a v1alpha1.DNSRecord, if one exists, for the given listener in the given v1alpha1.ManagedZone.
// It needs a reference string to enforce DNS record serving a single traffic.Interface owner
func (dh *dnsHelper) getDNSRecordForListener(ctx context.Context, listener gatewayv1beta1.Listener, owner metav1.Object) (*v1alpha1.DNSRecord, error) {
	recordName := dnsRecordName(owner.GetName(), string(listener.Name))
	dnsRecord := &v1alpha1.DNSRecord{}
	if err := dh.Get(ctx, client.ObjectKey{Name: recordName, Namespace: owner.GetNamespace()}, dnsRecord); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Log.V(1).Info("no dnsrecord found for listener ", "listener", listener)
		}
		return nil, err
	}
	return dnsRecord, nil
}

func withGatewayListener[T metav1.Object](gateway common.GatewayWrapper, listener gatewayv1beta1.Listener, obj T) T {
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{})
	}

	obj.GetAnnotations()["dnsrecord-name"] = fmt.Sprintf("%s-%s", gateway.Name, listener.Name)
	obj.GetAnnotations()["dnsrecord-namespace"] = gateway.Namespace

	return obj
}

// setEndpoints sets the endpoints for the given MultiClusterGatewayTarget
//
// Builds an array of v1alpha1.Endpoint resources and sets them on the given DNSRecord. The endpoints expected are calculated
// from the MultiClusterGatewayTarget using the target Gateway (MultiClusterGatewayTarget.Gateway), the LoadBalancing Spec
// from the DNSPolicy attached to the target gateway (MultiClusterGatewayTarget.LoadBalancing) and the list of clusters the
// target gateway is currently placed on (MultiClusterGatewayTarget.ClusterGatewayTargets).
//
// MultiClusterGatewayTarget.ClusterGatewayTarget are grouped by Geo, in the case of Geo not being defined in the
// LoadBalancing Spec (Weighted only) an internal only Geo Code of "default" is used and all clusters added to it.
//
// A CNAME record is created for the target host (DNSRecord.name), pointing to a generated gateway lb host.
// A CNAME record for the gateway lb host is created for every Geo, with appropriate Geo information, pointing to a geo
// specific host.
// A CNAME record for the geo specific host is created for every Geo, with weight information for that target added,
// pointing to a target cluster hostname.
// An A record for the target cluster hostname is created for any IP targets retrieved for that cluster.
//
// Example(Weighted only)
//
// www.example.com CNAME lb-1ab1.www.example.com
// lb-1ab1.www.example.com CNAME geolocation * default.lb-1ab1.www.example.com
// default.lb-1ab1.www.example.com CNAME weighted 100 1bc1.lb-1ab1.www.example.com
// default.lb-1ab1.www.example.com CNAME weighted 100 aws.lb.com
// 1bc1.lb-1ab1.www.example.com A 192.22.2.1
//
// Example(Geo, default IE)
//
// shop.example.com CNAME lb-a1b2.shop.example.com
// lb-a1b2.shop.example.com CNAME geolocation ireland ie.lb-a1b2.shop.example.com
// lb-a1b2.shop.example.com geolocation australia aus.lb-a1b2.shop.example.com
// lb-a1b2.shop.example.com geolocation default ie.lb-a1b2.shop.example.com (set by the default geo option)
// ie.lb-a1b2.shop.example.com CNAME weighted 100 ab1.lb-a1b2.shop.example.com
// ie.lb-a1b2.shop.example.com CNAME weighted 100 aws.lb.com
// aus.lb-a1b2.shop.example.com CNAME weighted 100 ab2.lb-a1b2.shop.example.com
// aus.lb-a1b2.shop.example.com CNAME weighted 100 ab3.lb-a1b2.shop.example.com
// ab1.lb-a1b2.shop.example.com A 192.22.2.1 192.22.2.5
// ab2.lb-a1b2.shop.example.com A 192.22.2.3
// ab3.lb-a1b2.shop.example.com A 192.22.2.4

func (dh *dnsHelper) setEndpoints(ctx context.Context, mcgTarget *dns.MultiClusterGatewayTarget, dnsRecord *v1alpha1.DNSRecord, dnsPolicy *v1alpha1.DNSPolicy, listener gatewayv1beta1.Listener) error {

	old := dnsRecord.DeepCopy()
	gwListenerHost := string(*listener.Hostname)
	cnameHost := gwListenerHost
	if isWildCardListener(listener) {
		cnameHost = strings.Replace(gwListenerHost, "*.", "", -1)
	}

	//Health Checks currently modify endpoints so we have to keep existing ones in order to not lose health check ids
	currentEndpoints := make(map[string]*v1alpha1.Endpoint, len(dnsRecord.Spec.Endpoints))
	for _, endpoint := range dnsRecord.Spec.Endpoints {
		currentEndpoints[endpoint.SetID()] = endpoint
	}

	var (
		newEndpoints    []*v1alpha1.Endpoint
		endpoint        *v1alpha1.Endpoint
		defaultEndpoint *v1alpha1.Endpoint
	)
	lbName := strings.ToLower(fmt.Sprintf("lb-%s.%s", mcgTarget.GetShortCode(), cnameHost))

	for geoCode, cgwTargets := range mcgTarget.GroupTargetsByGeo() {
		geoLbName := strings.ToLower(fmt.Sprintf("%s.%s", geoCode, lbName))
		var clusterEndpoints []*v1alpha1.Endpoint
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
				endpoint = createOrUpdateEndpoint(clusterLbName, ipValues, v1alpha1.ARecordType, "", dns.DefaultTTL, currentEndpoints)
				clusterEndpoints = append(clusterEndpoints, endpoint)
				hostValues = append(hostValues, clusterLbName)
			}

			for _, hostValue := range hostValues {
				endpoint = createOrUpdateEndpoint(geoLbName, []string{hostValue}, v1alpha1.CNAMERecordType, hostValue, dns.DefaultTTL, currentEndpoints)
				endpoint.SetProviderSpecific(dns.ProviderSpecificWeight, strconv.Itoa(cgwTarget.GetWeight()))
				clusterEndpoints = append(clusterEndpoints, endpoint)
			}
		}
		if len(clusterEndpoints) == 0 {
			continue
		}
		newEndpoints = append(newEndpoints, clusterEndpoints...)

		//Create lbName CNAME (lb-a1b2.shop.example.com -> default.lb-a1b2.shop.example.com)
		endpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, string(geoCode), dns.DefaultCnameTTL, currentEndpoints)

		//Deal with the default geo endpoint first
		if geoCode.IsDefaultCode() {
			defaultEndpoint = endpoint
			// continue here as we will add the `defaultEndpoint` later
			continue
		} else if (geoCode == mcgTarget.GetDefaultGeo()) || defaultEndpoint == nil {
			// Ensure that a `defaultEndpoint` is always set, but the expected default takes precedence
			defaultEndpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, "default", dns.DefaultCnameTTL, currentEndpoints)
		}

		endpoint.SetProviderSpecific(dns.ProviderSpecificGeoCode, string(geoCode))

		newEndpoints = append(newEndpoints, endpoint)
	}

	if len(newEndpoints) > 0 {
		// Add the `defaultEndpoint`, this should always be set by this point if `newEndpoints` isn't empty
		defaultEndpoint.SetProviderSpecific(dns.ProviderSpecificGeoCode, string(dns.WildcardGeo))
		newEndpoints = append(newEndpoints, defaultEndpoint)
		//Create gwListenerHost CNAME (shop.example.com -> lb-a1b2.shop.example.com)
		endpoint = createOrUpdateEndpoint(gwListenerHost, []string{lbName}, v1alpha1.CNAMERecordType, "", dns.DefaultCnameTTL, currentEndpoints)
		newEndpoints = append(newEndpoints, endpoint)
	}

	sort.Slice(newEndpoints, func(i, j int) bool {
		return newEndpoints[i].SetID() < newEndpoints[j].SetID()
	})

	probes, err := dh.getDNSHealthCheckProbes(ctx, mcgTarget.Gateway, dnsPolicy)
	if err != nil {
		return err
	}

	// if the checks on endpoints based on probes results in there being no healthy endpoints
	// ready to publish we'll publish the full set so storing those
	var storeEndpoints []*v1alpha1.Endpoint
	storeEndpoints = append(storeEndpoints, newEndpoints...)
	// count will track whether a new endpoint has been removed.
	// first newEndpoints are checked based on probe status and removed if unhealthy true and the consecutive failures are greater than the threshold.
	removedEndpoints := 0
	for i := 0; i < len(newEndpoints); i++ {
		checkProbes := getProbesForEndpoint(newEndpoints[i], probes)
		if len(checkProbes) == 0 {
			continue
		}
		for _, probe := range checkProbes {
			probeHealthy := true
			if probe.Status.Healthy != nil {
				probeHealthy = *probe.Status.Healthy
			}
			// if any probe for any target is reporting unhealthy remove it from the endpoint list
			if !probeHealthy && probe.Spec.FailureThreshold != nil && probe.Status.ConsecutiveFailures >= *probe.Spec.FailureThreshold {
				newEndpoints = append(newEndpoints[:i], newEndpoints[i+1:]...)
				removedEndpoints++
				i--
				break
			}
		}
	}
	// after checkProbes are checked the newEndpoints is looped through until count is 0
	// if any are found that need to be removed because a parent with no children present
	// the count will be incremented so that the newEndpoints will be traversed again such that only when a loop occurs where no
	// endpoints have been removed can we consider the endpoint list to be cleaned
	ipPattern := `\b(?:\d{1,3}\.){3}\d{1,3}\b`
	re := regexp.MustCompile(ipPattern)

	for removedEndpoints > 0 {
	endpointsLoop:
		for i := 0; i < len(newEndpoints); i++ {
			checkEndpoint := newEndpoints[i]
			for _, target := range checkEndpoint.Targets {
				if len(re.FindAllString(target, -1)) > 0 {
					// don't check the children of targets which are ips.
					continue endpointsLoop
				}
			}
			children := getNumChildrenOfParent(newEndpoints, newEndpoints[i])
			if children == 0 {
				newEndpoints = append(newEndpoints[:i], newEndpoints[i+1:]...)
				removedEndpoints++
			}
		}
		removedEndpoints--
	}

	// if there are no healthy endpoints after checking, publish the full set before checks
	if len(newEndpoints) == 0 {
		dnsRecord.Spec.Endpoints = storeEndpoints
	} else {
		dnsRecord.Spec.Endpoints = newEndpoints
	}
	if !equality.Semantic.DeepEqual(old, dnsRecord) {
		return dh.Update(ctx, dnsRecord)
	}
	return nil
}

func getNumChildrenOfParent(endpoints []*v1alpha1.Endpoint, parent *v1alpha1.Endpoint) int {
	return len(findChildren(endpoints, parent))
}

func findChildren(endpoints []*v1alpha1.Endpoint, parent *v1alpha1.Endpoint) []*v1alpha1.Endpoint {
	var foundEPs []*v1alpha1.Endpoint
	for _, endpoint := range endpoints {
		for _, target := range parent.Targets {
			if target == endpoint.DNSName {
				foundEPs = append(foundEPs, endpoint)
			}
		}
	}
	return foundEPs
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

// removeDNSForDeletedListeners remove any DNSRecords that are associated with listeners that no longer exist in this gateway
func (r *dnsHelper) removeDNSForDeletedListeners(ctx context.Context, upstreamGateway *gatewayv1beta1.Gateway) error {
	dnsList := &v1alpha1.DNSRecordList{}
	//List all dns records that belong to this gateway
	labelSelector := &client.MatchingLabels{
		LabelGatewayReference: upstreamGateway.Name,
	}
	if err := r.List(ctx, dnsList, labelSelector, &client.ListOptions{Namespace: upstreamGateway.Namespace}); err != nil {
		return err
	}

	for _, dns := range dnsList.Items {
		listenerExists := false
		for _, listener := range upstreamGateway.Spec.Listeners {
			if listener.Name == gatewayv1beta1.SectionName(dns.Labels[LabelListenerReference]) {
				listenerExists = true
				break
			}
		}
		if !listenerExists {
			if err := r.Delete(ctx, &dns, &client.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}
	return nil

}

func (r *dnsHelper) getManagedZoneForListener(ctx context.Context, ns string, listener gatewayv1beta1.Listener) (*v1alpha1.ManagedZone, error) {
	var managedZones v1alpha1.ManagedZoneList
	if err := r.List(ctx, &managedZones, client.InNamespace(ns)); err != nil {
		log.FromContext(ctx).Error(err, "unable to list managed zones for gateway ", "in ns", ns)
		return nil, err
	}
	host := string(*listener.Hostname)
	mz, _, err := findMatchingManagedZone(host, host, managedZones.Items)
	return mz, err
}

func dnsRecordName(gatewayName, listenerName string) string {
	return fmt.Sprintf("%s-%s", gatewayName, listenerName)
}

func (r *dnsHelper) createDNSRecordForListener(ctx context.Context, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy, mz *v1alpha1.ManagedZone, listener gatewayv1beta1.Listener) (*v1alpha1.DNSRecord, error) {

	log := log.FromContext(ctx)
	log.Info("creating dns for gateway listener", "listener", listener.Name)
	dnsRecord := r.buildDNSRecordForListener(gateway, dnsPolicy, listener, mz)
	if err := controllerutil.SetControllerReference(mz, dnsRecord, r.Scheme()); err != nil {
		return dnsRecord, err
	}

	err := r.Create(ctx, dnsRecord, &client.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return dnsRecord, err
	}
	if err != nil && k8serrors.IsAlreadyExists(err) {
		err = r.Get(ctx, client.ObjectKeyFromObject(dnsRecord), dnsRecord)
		if err != nil {
			return dnsRecord, err
		}
	}
	return dnsRecord, nil
}

func (r *dnsHelper) deleteDNSRecordForListener(ctx context.Context, owner metav1.Object, listener gatewayv1beta1.Listener) error {
	recordName := dnsRecordName(owner.GetName(), string(listener.Name))
	dnsRecord := v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recordName,
			Namespace: owner.GetNamespace(),
		},
	}
	return r.Delete(ctx, &dnsRecord, &client.DeleteOptions{})
}

func isWildCardListener(l gatewayv1beta1.Listener) bool {
	return strings.HasPrefix(string(*l.Hostname), "*")
}

func (dh *dnsHelper) getDNSHealthCheckProbes(ctx context.Context, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) ([]*v1alpha1.DNSHealthCheckProbe, error) {
	list := &v1alpha1.DNSHealthCheckProbeList{}
	if err := dh.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy))),
		Namespace:     dnsPolicy.Namespace,
	}); err != nil {
		return nil, err
	}

	return slice.MapErr(list.Items, func(obj v1alpha1.DNSHealthCheckProbe) (*v1alpha1.DNSHealthCheckProbe, error) {
		return &obj, nil
	})
}

func getProbesForEndpoint(endpoint *v1alpha1.Endpoint, probes []*v1alpha1.DNSHealthCheckProbe) []*v1alpha1.DNSHealthCheckProbe {
	retProbes := []*v1alpha1.DNSHealthCheckProbe{}
	for _, probe := range probes {
		for _, target := range endpoint.Targets {
			if strings.Contains(probe.Name, target) {
				retProbes = append(retProbes, probe)
			}
		}
	}
	return retProbes
}
