//go:build unit

package fake

import (
	"context"
	"fmt"
	"strings"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

type FakeHostService struct {
	controlClient client.Client
}

func NewTestHostService(client client.Client) *FakeHostService {
	return &FakeHostService{controlClient: client}
}

func (h *FakeHostService) SetEndpoints(_ context.Context, _ *MultiClusterGatewayTarget, _ *v1alpha1.DNSRecord) error {
	return nil
}

func (h *FakeHostService) GetDNSRecordsFor(_ context.Context, _ traffic.Interface) ([]*v1alpha1.DNSRecord, error) {
	return nil, nil
}

func (h *FakeHostService) CleanupDNSRecords(_ context.Context, _ traffic.Interface) error {
	return nil
}

func (h *FakeHostService) GetManagedHosts(ctx context.Context, traffic traffic.Interface) ([]v1alpha1.ManagedHost, error) {
	managed := []v1alpha1.ManagedHost{}
	for _, host := range traffic.GetHosts() {
		managedZone, subDomain, err := h.GetManagedZoneForHost(ctx, host, traffic)
		if err != nil {
			return nil, err
		}
		if managedZone == nil {
			// its ok for no managedzone to be present as this could be a CNAME or externally managed host
			continue
		}
		dnsRecord, err := h.GetDNSRecord(ctx, subDomain, managedZone, traffic)
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

func (h *FakeHostService) CreateDNSRecord(_ context.Context, subDomain string, _ *v1alpha1.ManagedZone, _ metav1.Object) (*v1alpha1.DNSRecord, error) {
	if subDomain == Cluster {
		return nil, fmt.Errorf(FailCreateDNSSubdomain)
	}
	record := v1alpha1.DNSRecord{}
	return &record, nil
}

func (h *FakeHostService) GetDNSRecord(ctx context.Context, subDomain string, managedZone *v1alpha1.ManagedZone, _ metav1.Object) (*v1alpha1.DNSRecord, error) {
	if subDomain == FailFetchDANSSubdomain {
		return &v1alpha1.DNSRecord{}, fmt.Errorf(FailFetchDANSSubdomain)
	}
	record := &v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedZone.Spec.DomainName,
			Namespace: managedZone.GetNamespace(),
		},
	}

	if err := h.controlClient.Get(ctx, client.ObjectKeyFromObject(record), record); err != nil {
		return nil, err
	}
	return record, nil
}

func (h *FakeHostService) GetDNSRecordManagedZone(ctx context.Context, dnsRecord *v1alpha1.DNSRecord) (*v1alpha1.ManagedZone, error) {

	if dnsRecord.Spec.ManagedZoneRef == nil {
		return nil, fmt.Errorf("no managed zone configured for : %s", dnsRecord.Name)
	}

	managedZone := &v1alpha1.ManagedZone{}

	err := h.controlClient.Get(ctx, client.ObjectKey{Namespace: dnsRecord.Namespace, Name: dnsRecord.Spec.ManagedZoneRef.Name}, managedZone)
	if err != nil {
		return nil, err
	}

	return managedZone, nil
}

func (h *FakeHostService) GetManagedZoneForHost(ctx context.Context, host string, t traffic.Interface) (*v1alpha1.ManagedZone, string, error) {
	hostParts := strings.SplitN(host, ".", 2)
	if len(hostParts) < 2 {
		return nil, "", fmt.Errorf("unable to parse host : %s on traffic resource : %s", host, t.GetName())
	}
	subDomain := hostParts[0]
	zone := v1alpha1.ManagedZoneList{}

	err := h.controlClient.List(ctx, &zone, client.InNamespace(t.GetNamespace()))
	if err != nil {
		return nil, "", err
	}
	if len(zone.Items) == 0 {
		return nil, subDomain, nil
	}
	return &zone.Items[0], subDomain, nil
}

func (h *FakeHostService) AddEndpoints(_ context.Context, gateway traffic.Interface, _ *v1alpha1.DNSRecord) error {
	hosts := gateway.GetHosts()
	for _, host := range hosts {
		if host == FailEndpointsHostname {
			return fmt.Errorf(FailEndpointsHostname)
		}
	}
	return nil
}
