package dns

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/publicsuffix"

	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

const (
	DefaultTTL            = 60
	DefaultCnameTTL       = 300
	LabelRecordID         = "kuadrant.io/record-id"
	LabelGatewayReference = "kuadrant.io/Gateway-uid"
)

var (
	ErrNoManagedZoneForHost = fmt.Errorf("no managed zone for host")
	ErrAlreadyAssigned      = fmt.Errorf("managed host already assigned")
)

type Service struct {
	controlClient client.Client
}

func NewService(controlClient client.Client) *Service {
	return &Service{controlClient: controlClient}
}

// func (s *Service) GetManagedHosts(ctx context.Context, traffic traffic.Interface) ([]v1alpha1.ManagedHost, error) {
// 	var managed []v1alpha1.ManagedHost
// 	for _, host := range traffic.GetHosts() {
// 		managedZone, subDomain, err := s.GetManagedZoneForHost(ctx, host, traffic)
// 		if err != nil && !errors.Is(err, ErrNoManagedZoneForHost) {
// 			return nil, err
// 		}
// 		if managedZone == nil {
// 			// its ok for no managedzone to be present as this could be a CNAME or externally managed host
// 			continue
// 		}
// 		dnsRecord, err := s.GetDNSRecord(ctx, subDomain, managedZone, traffic)
// 		if err != nil && !k8serrors.IsNotFound(err) {
// 			return nil, err
// 		}
// 		managedHost := v1alpha1.ManagedHost{
// 			Host:        host,
// 			Subdomain:   subDomain,
// 			ManagedZone: managedZone,
// 			DnsRecord:   dnsRecord,
// 		}

// 		managed = append(managed, managedHost)
// 	}
// 	return managed, nil
// }

// // GetDNSRecord returns a v1alpha1.DNSRecord, if one exists, for the given subdomain in the given v1alpha1.ManagedZone.
// // It needs a reference string to enforce DNS record serving a single traffic.Interface owner
// func (s *Service) GetDNSRecord(ctx context.Context, subDomain string, managedZone *v1alpha1.ManagedZone, owner metav1.Object) (*v1alpha1.DNSRecord, error) {
// 	managedHost := strings.ToLower(fmt.Sprintf("%s.%s", subDomain, managedZone.Spec.DomainName))

// 	dnsRecord := &v1alpha1.DNSRecord{}
// 	if err := s.controlClient.Get(ctx, client.ObjectKey{Name: managedHost, Namespace: managedZone.GetNamespace()}, dnsRecord); err != nil {
// 		if k8serrors.IsNotFound(err) {
// 			log.Log.V(1).Info("no dnsrecord found for host ", "host", managedHost)
// 		}
// 		return nil, err
// 	}
// 	if dnsRecord.GetLabels()[LabelGatewayReference] != string(owner.GetUID()) {
// 		return nil, fmt.Errorf("attempting to get a DNSrecord for a host already in use by a different traffic object. Host: %s", managedHost)
// 	}
// 	return dnsRecord, nil
// }

// SetEndpoints sets the endpoints for the given MultiClusterGatewayTarget
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

func (s *Service) SetEndpoints(ctx context.Context, mcgTarget *MultiClusterGatewayTarget, dnsRecord *v1alpha1.DNSRecord) error {

	old := dnsRecord.DeepCopy()
	gwListenerHost := dnsRecord.Name

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
	lbName := strings.ToLower(fmt.Sprintf("lb-%s.%s", mcgTarget.GetShortCode(), gwListenerHost))

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
				endpoint = createOrUpdateEndpoint(clusterLbName, ipValues, v1alpha1.ARecordType, "", DefaultTTL, currentEndpoints)
				clusterEndpoints = append(clusterEndpoints, endpoint)
				hostValues = append(hostValues, clusterLbName)
			}

			for _, hostValue := range hostValues {
				endpoint = createOrUpdateEndpoint(geoLbName, []string{hostValue}, v1alpha1.CNAMERecordType, hostValue, DefaultTTL, currentEndpoints)
				endpoint.SetProviderSpecific(ProviderSpecificWeight, strconv.Itoa(cgwTarget.GetWeight()))
				clusterEndpoints = append(clusterEndpoints, endpoint)
			}
		}
		if len(clusterEndpoints) == 0 {
			continue
		}
		newEndpoints = append(newEndpoints, clusterEndpoints...)

		//Create lbName CNAME (lb-a1b2.shop.example.com -> default.lb-a1b2.shop.example.com)
		endpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, string(geoCode), DefaultCnameTTL, currentEndpoints)

		//Deal with the default geo endpoint first
		if geoCode.IsDefaultCode() {
			defaultEndpoint = endpoint
			// continue here as we will add the `defaultEndpoint` later
			continue
		} else if (geoCode == mcgTarget.GetDefaultGeo()) || defaultEndpoint == nil {
			// Ensure that a `defaultEndpoint` is always set, but the expected default takes precedence
			defaultEndpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, "default", DefaultCnameTTL, currentEndpoints)
		}

		if geoCode.IsContinentCode() {
			endpoint.SetProviderSpecific(ProviderSpecificGeoContinentCode, string(geoCode))
		} else if geoCode.IsCountryCode() {
			endpoint.SetProviderSpecific(ProviderSpecificGeoCountryCode, string(geoCode))
		}
		newEndpoints = append(newEndpoints, endpoint)
	}

	if len(newEndpoints) > 0 {
		// Add the `defaultEndpoint`, this should always be set by this point if `newEndpoints` isn't empty
		defaultEndpoint.SetProviderSpecific(ProviderSpecificGeoCountryCode, "*")
		newEndpoints = append(newEndpoints, defaultEndpoint)
		//Create gwListenerHost CNAME (shop.example.com -> lb-a1b2.shop.example.com)
		endpoint = createOrUpdateEndpoint(gwListenerHost, []string{lbName}, v1alpha1.CNAMERecordType, "", DefaultCnameTTL, currentEndpoints)
		newEndpoints = append(newEndpoints, endpoint)
	}

	sort.Slice(newEndpoints, func(i, j int) bool {
		return newEndpoints[i].SetID() < newEndpoints[j].SetID()
	})
	dnsRecord.Spec.Endpoints = newEndpoints

	if !equality.Semantic.DeepEqual(old, dnsRecord) {
		return s.controlClient.Update(ctx, dnsRecord)
	}
	return nil
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

// GetManagedZoneForHost returns a ManagedZone and subDomain for the given host if one exists.
//
// Currently, this returns the first matching ManagedZone found in the traffic resources own namespace
// func (s *Service) GetManagedZoneForHost(ctx context.Context, host string, t traffic.Interface) (*v1alpha1.ManagedZone, string, error) {
// 	var managedZones v1alpha1.ManagedZoneList
// 	if err := s.controlClient.List(ctx, &managedZones, client.InNamespace(t.GetNamespace())); err != nil {
// 		log.FromContext(ctx).Error(err, "unable to list managed zones in traffic resource NS")
// 		return nil, "", err
// 	}
// 	return FindMatchingManagedZone(host, host, managedZones.Items)
// }

func FindMatchingManagedZone(originalHost, host string, zones []v1alpha1.ManagedZone) (*v1alpha1.ManagedZone, string, error) {
	if len(zones) == 0 {
		return nil, "", fmt.Errorf("%w : %s", ErrNoManagedZoneForHost, host)
	}
	host = strings.ToLower(host)
	//get the TLD from this host
	tld, _ := publicsuffix.PublicSuffix(host)

	//The host is just the TLD, or the detected TLD is not an ICANN TLD
	if host == tld {
		return nil, "", fmt.Errorf("no valid zone found for host: %v", originalHost)
	}

	hostParts := strings.SplitN(host, ".", 2)
	if len(hostParts) < 2 {
		return nil, "", fmt.Errorf("no valid zone found for host: %s", originalHost)
	}

	zone, ok := slice.Find(zones, func(zone v1alpha1.ManagedZone) bool {
		return strings.ToLower(zone.Spec.DomainName) == host
	})

	if ok {
		subdomain := strings.Replace(strings.ToLower(originalHost), "."+strings.ToLower(zone.Spec.DomainName), "", 1)
		return &zone, subdomain, nil
	} else {
		parentDomain := hostParts[1]
		return FindMatchingManagedZone(originalHost, parentDomain, zones)
	}

}

// // CleanupDNSRecords removes all DNS records that were created for a provided traffic.Interface object
// func (s *Service) CleanupDNSRecords(ctx context.Context, owner traffic.Interface) error {
// 	recordsToCleaunup := &v1alpha1.DNSRecordList{}
// 	selector, _ := labels.Parse(fmt.Sprintf("%s=%s", LabelGatewayReference, owner.GetUID()))

// 	if err := s.controlClient.List(ctx, recordsToCleaunup, &client.ListOptions{LabelSelector: selector}); err != nil {
// 		return err
// 	}
// 	for _, record := range recordsToCleaunup.Items {
// 		if err := s.controlClient.Delete(ctx, &record); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }
