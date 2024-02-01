package fake

import (
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/provider"
)

type Provider struct {
	EnsureFunc            func(*v1alpha1.DNSRecord, *v1alpha1.ManagedZone) error
	DeleteFunc            func(*v1alpha1.DNSRecord, *v1alpha1.ManagedZone) error
	EnsureManagedZoneFunc func(*v1alpha1.ManagedZone) (provider.ManagedZoneOutput, error)
	DeleteManagedZoneFunc func(*v1alpha1.ManagedZone) error
}

var _ provider.Provider = &Provider{}

func (p Provider) Ensure(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return p.EnsureFunc(record, managedZone)
}

func (p Provider) Delete(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return p.DeleteFunc(record, managedZone)
}

func (p Provider) EnsureManagedZone(managedZone *v1alpha1.ManagedZone) (provider.ManagedZoneOutput, error) {
	return p.EnsureManagedZoneFunc(managedZone)
}

func (p Provider) DeleteManagedZone(managedZone *v1alpha1.ManagedZone) error {
	return p.DeleteManagedZoneFunc(managedZone)
}
