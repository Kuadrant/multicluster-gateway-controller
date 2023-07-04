/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dns

import (
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

const (
	DefaultTTL = 60
)

// Provider knows how to manage DNS zones only as pertains to routing.
type Provider interface {

	// Ensure will create or update record.
	Ensure(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error

	// Delete will delete record.
	Delete(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error

	// Ensure will create or update a managed zone, returns an array of NameServers for that zone.
	EnsureManagedZone(managedZone *v1alpha1.ManagedZone) (ManagedZoneOutput, error)

	// Delete will delete a managed zone.
	DeleteManagedZone(managedZone *v1alpha1.ManagedZone) error

	// Get an instance of HealthCheckReconciler for this provider
	HealthCheckReconciler() HealthCheckReconciler

	ProviderSpecific() ProviderSpecificLabels
}

type ProviderSpecificLabels struct {
	Weight        string
	HealthCheckID string
}

type ManagedZoneOutput struct {
	ID          string
	NameServers []*string
	RecordCount int64
}

var _ Provider = &FakeProvider{}

type FakeProvider struct{}

func (*FakeProvider) Ensure(dnsRecord *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return nil
}
func (*FakeProvider) Delete(dnsRecord *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return nil
}
func (*FakeProvider) EnsureManagedZone(managedZone *v1alpha1.ManagedZone) (ManagedZoneOutput, error) {
	return ManagedZoneOutput{}, nil
}
func (*FakeProvider) DeleteManagedZone(managedZone *v1alpha1.ManagedZone) error { return nil }
func (*FakeProvider) HealthCheckReconciler() HealthCheckReconciler {
	return &FakeHealthCheckReconciler{}
}
func (*FakeProvider) ProviderSpecific() ProviderSpecificLabels {
	return ProviderSpecificLabels{
		Weight:        "weight",
		HealthCheckID: "fake/health-check-id",
	}
}
