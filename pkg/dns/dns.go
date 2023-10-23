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
	"context"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
)

const (
	DefaultTTL              = 60
	DefaultCnameTTL         = 300
	ProviderSpecificWeight  = "weight"
	ProviderSpecificGeoCode = "geo-code"
)

type DNSProviderFactory func(ctx context.Context, pa v1alpha2.ProviderAccessor) (Provider, error)

// Provider knows how to manage DNS zones only as pertains to routing.
type Provider interface {

	// Ensure will create or update record.
	Ensure(record *v1alpha2.DNSRecord) error

	// Delete will delete record.
	Delete(record *v1alpha2.DNSRecord) error

	// List all zones
	ListZones() (ZoneList, error)

	// Ensure will create or update a managed zone, returns an array of NameServers for that zone.
	EnsureManagedZone(managedZone *v1alpha2.ManagedZone) (ManagedZoneOutput, error)

	// Delete will delete a managed zone.
	DeleteManagedZone(managedZone *v1alpha2.ManagedZone) error

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

type Zone struct {
	ID      *string
	DNSName *string
}

type ZoneList struct {
	Items []*Zone
}

type ProviderConfig struct {
	ZoneIDFilter []string
	DomainFilter []string
}

func ConfigFromJSON(jsonKey []byte) (*ProviderConfig, error) {
	var pConfig struct {
		ZoneIDFilter []string `json:"zoneIDFilter"`
		DomainFilter []string `json:"domainFilter"`
	}
	if len(jsonKey) > 0 {
		if err := json.Unmarshal(jsonKey, &pConfig); err != nil {
			return nil, err
		}
	}
	return &ProviderConfig{
		ZoneIDFilter: pConfig.ZoneIDFilter,
	}, nil
}

var _ Provider = &FakeProvider{}

type FakeProvider struct{}

func (*FakeProvider) Ensure(_ *v1alpha2.DNSRecord) error {
	return nil
}
func (*FakeProvider) Delete(_ *v1alpha2.DNSRecord) error {
	return nil
}
func (*FakeProvider) ListZones() (ZoneList, error) {
	return ZoneList{}, nil
}
func (*FakeProvider) EnsureManagedZone(mz *v1alpha2.ManagedZone) (ManagedZoneOutput, error) {
	return ManagedZoneOutput{
		ID:          *mz.Spec.ID,
		NameServers: nil,
		RecordCount: 0,
	}, nil
}
func (*FakeProvider) DeleteManagedZone(_ *v1alpha2.ManagedZone) error { return nil }

func (*FakeProvider) HealthCheckReconciler() HealthCheckReconciler {
	return &FakeHealthCheckReconciler{}
}
func (*FakeProvider) ProviderSpecific() ProviderSpecificLabels {
	return ProviderSpecificLabels{
		Weight:        "weight",
		HealthCheckID: "fake/health-check-id",
	}
}

// SanitizeError removes request specific data from error messages in order to make them consistent across multiple similar requests to the provider.  e.g AWS SDK Request ids `request id: 051c860b-9b30-4c19-be1a-1280c3e9fdc4`
func SanitizeError(err error) error {
	re := regexp.MustCompile(`request id: [^\s]+`)
	sanitizedErr := re.ReplaceAllString(err.Error(), "")
	return errors.New(sanitizedErr)
}
