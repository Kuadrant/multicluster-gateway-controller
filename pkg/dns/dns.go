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
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns/aws"
)

// Provider knows how to manage DNS zones only as pertains to routing.
type Provider interface {
	// Ensure will create or update record.
	Ensure(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error

	// Delete will delete record.
	Delete(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error

	// Ensure will create or update a managed zone, returns an array of NameServers for that zone.
	EnsureManagedZone(managedZone *v1alpha1.ManagedZone) (aws.ManagedZoneOutput, error)

	// Delete will delete a managed zone.
	DeleteManagedZone(managedZone *v1alpha1.ManagedZone) error
}

var _ Provider = &FakeProvider{}

type FakeProvider struct{}

func (_ *FakeProvider) Ensure(dnsRecord *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return nil
}
func (_ *FakeProvider) Delete(dnsRecord *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return nil
}
func (_ *FakeProvider) EnsureManagedZone(managedZone *v1alpha1.ManagedZone) (aws.ManagedZoneOutput, error) {
	return aws.ManagedZoneOutput{}, nil
}
func (_ *FakeProvider) DeleteManagedZone(managedZone *v1alpha1.ManagedZone) error { return nil }
