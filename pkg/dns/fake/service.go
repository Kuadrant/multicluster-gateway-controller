//go:build unit

package fake

import (
	"context"
	"fmt"

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

func (h *FakeHostService) AddEndpoints(_ context.Context, gateway traffic.Interface, _ *v1alpha1.DNSRecord) error {
	hosts := gateway.GetHosts()
	for _, host := range hosts {
		if host == FailEndpointsHostname {
			return fmt.Errorf(FailEndpointsHostname)
		}
	}
	return nil
}
