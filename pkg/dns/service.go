package dns

import (
	"context"
	"errors"
	"fmt"
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

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
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

func (s *Service) GetManagedHosts(ctx context.Context, traffic traffic.Interface) ([]v1alpha1.ManagedHost, error) {
	var managed []v1alpha1.ManagedHost
	for _, host := range traffic.GetHosts() {
		managedZone, subDomain, err := s.GetManagedZoneForHost(ctx, host, traffic)
		if err != nil && !errors.Is(err, ErrNoManagedZoneForHost) {
			return nil, err
		}
		if managedZone == nil {
			// its ok for no managedzone to be present as this could be a CNAME or externally managed host
			continue
		}
		dnsRecord, err := s.GetDNSRecord(ctx, subDomain, managedZone, traffic)
		if err != nil && !k8serrors.IsNotFound(err) {
			return nil, err
		}
		managedHost := v1alpha1.ManagedHost{
			Host:        host,
			Subdomain:   subDomain,
			ManagedZone: managedZone,
			DnsRecord:   dnsRecord,
		}

		managed = append(managed, managedHost)
	}
	return managed, nil
}

// CreateDNSRecord creates a new DNSRecord, if one does not already exist, in the given managed zone with the given subdomain.
// Needs traffic.Interface owner to block other traffic objects from accessing this record
func (s *Service) CreateDNSRecord(ctx context.Context, subDomain string, managedZone *v1alpha1.ManagedZone, owner metav1.Object) (*v1alpha1.DNSRecord, error) {
	managedHost := strings.ToLower(fmt.Sprintf("%s.%s", subDomain, managedZone.Spec.DomainName))

	dnsRecord := v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedHost,
			Namespace: managedZone.Namespace,
			Labels: map[string]string{
				LabelRecordID:         subDomain,
				LabelGatewayReference: string(owner.GetUID()),
			},
		},
		Spec: v1alpha1.DNSRecordSpec{
			ManagedZoneRef: &v1alpha1.ManagedZoneReference{
				Name: managedZone.Name,
			},
		},
	}
	if err := controllerutil.SetOwnerReference(owner, &dnsRecord, s.controlClient.Scheme()); err != nil {
		return nil, err
	}
	err := controllerutil.SetControllerReference(managedZone, &dnsRecord, s.controlClient.Scheme())
	if err != nil {
		return nil, err
	}

	err = s.controlClient.Create(ctx, &dnsRecord, &client.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}
	//host may already be present
	if err != nil && k8serrors.IsAlreadyExists(err) {
		err = s.controlClient.Get(ctx, client.ObjectKeyFromObject(&dnsRecord), &dnsRecord)
		if err != nil {
			return nil, err
		}
	}
	return &dnsRecord, nil
}

// GetDNSRecord returns a v1alpha1.DNSRecord, if one exists, for the given subdomain in the given v1alpha1.ManagedZone.
// It needs a reference string to enforce DNS record serving a single traffic.Interface owner
func (s *Service) GetDNSRecord(ctx context.Context, subDomain string, managedZone *v1alpha1.ManagedZone, owner metav1.Object) (*v1alpha1.DNSRecord, error) {
	managedHost := strings.ToLower(fmt.Sprintf("%s.%s", subDomain, managedZone.Spec.DomainName))

	dnsRecord := &v1alpha1.DNSRecord{}
	if err := s.controlClient.Get(ctx, client.ObjectKey{Name: managedHost, Namespace: managedZone.GetNamespace()}, dnsRecord); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Log.V(1).Info("no dnsrecord found for host ", "host", managedHost)
		}
		return nil, err
	}
	if dnsRecord.GetLabels()[LabelGatewayReference] != string(owner.GetUID()) {
		return nil, fmt.Errorf("attempting to get a DNSrecord for a host already in use by a different traffic object. Host: %s", managedHost)
	}
	return dnsRecord, nil
}

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
		newEndpoints []*v1alpha1.Endpoint
		endpoint     *v1alpha1.Endpoint
	)

	lbName := strings.ToLower(fmt.Sprintf("lb-%s.%s", mcgTarget.GetShortCode(), gwListenerHost))
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
			endpoint.SetProviderSpecific(ProviderSpecificGeoCountryCode, "*")
		case geoCode.IsContinentCode():
			endpoint.SetProviderSpecific(ProviderSpecificGeoContinentCode, string(geoCode))
		case geoCode.IsCountryCode():
			endpoint.SetProviderSpecific(ProviderSpecificGeoCountryCode, string(geoCode))
		}

		//Create the default geo if this geo matches the default policy geo and we haven't just created it
		if !geoCode.IsDefaultCode() && geoCode == mcgTarget.getDefaultGeo() {
			endpoint = createOrUpdateEndpoint(lbName, []string{geoLbName}, v1alpha1.CNAMERecordType, "default", DefaultCnameTTL, currentEndpoints)
			newEndpoints = append(newEndpoints, endpoint)
			endpoint.SetProviderSpecific(ProviderSpecificGeoCountryCode, "*")
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
				endpoint.SetProviderSpecific(ProviderSpecificWeight, strconv.Itoa(cgwTarget.GetWeight()))
				newEndpoints = append(newEndpoints, endpoint)
			}

		}
	}

	dnsRecord.Spec.Endpoints = newEndpoints

	if equality.Semantic.DeepEqual(old.Spec, dnsRecord.Spec) {
		return nil
	}

	return s.controlClient.Update(ctx, dnsRecord, &client.UpdateOptions{})
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
func (s *Service) GetManagedZoneForHost(ctx context.Context, host string, t traffic.Interface) (*v1alpha1.ManagedZone, string, error) {
	var managedZones v1alpha1.ManagedZoneList
	if err := s.controlClient.List(ctx, &managedZones, client.InNamespace(t.GetNamespace())); err != nil {
		log.FromContext(ctx).Error(err, "unable to list managed zones in traffic resource NS")
		return nil, "", err
	}
	return FindMatchingManagedZone(host, host, managedZones.Items)
}

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

// CleanupDNSRecords removes all DNS records that were created for a provided traffic.Interface object
func (s *Service) CleanupDNSRecords(ctx context.Context, owner traffic.Interface) error {
	recordsToCleaunup := &v1alpha1.DNSRecordList{}
	selector, _ := labels.Parse(fmt.Sprintf("%s=%s", LabelGatewayReference, owner.GetUID()))

	if err := s.controlClient.List(ctx, recordsToCleaunup, &client.ListOptions{LabelSelector: selector}); err != nil {
		return err
	}
	for _, record := range recordsToCleaunup.Items {
		if err := s.controlClient.Delete(ctx, &record); err != nil {
			return err
		}
	}
	return nil
}
