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

package v1alpha2

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetID returns an id that should be unique across a set of endpoints
func (e *Endpoint) SetID() string {
	return e.DNSName + e.SetIdentifier
}

// ProviderSpecificProperty holds the name and value of a configuration which is specific to individual DNS providers
type ProviderSpecificProperty struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// Targets is a representation of a list of targets for an endpoint.
type Targets []string

// TTL is a structure defining the TTL of a DNS record
type TTL int64

// Labels store metadata related to the endpoint
// it is then stored in a persistent storage via serialization
type Labels map[string]string

// ProviderSpecific holds configuration which is specific to individual DNS providers
type ProviderSpecific []ProviderSpecificProperty

// Endpoint is a high-level way of a connection between a service and an IP
type Endpoint struct {
	// The hostname of the DNS record
	DNSName string `json:"dnsName,omitempty"`
	// The targets the DNS record points to
	Targets Targets `json:"targets,omitempty"`
	// RecordType type of record, e.g. CNAME, A, SRV, TXT etc
	RecordType string `json:"recordType,omitempty"`
	// Identifier to distinguish multiple records with the same name and type (e.g. Route53 records with routing policies other than 'simple')
	SetIdentifier string `json:"setIdentifier,omitempty"`
	// TTL for the record
	RecordTTL TTL `json:"recordTTL,omitempty"`
	// Labels stores labels defined for the Endpoint
	// +optional
	Labels Labels `json:"labels,omitempty"`
	// ProviderSpecific stores provider specific config
	// +optional
	ProviderSpecific ProviderSpecific `json:"providerSpecific,omitempty"`
}

// WithSetIdentifier applies the given set identifier to the endpoint.
func (e *Endpoint) WithSetIdentifier(setIdentifier string) *Endpoint {
	e.SetIdentifier = setIdentifier
	return e
}

// WithProviderSpecific attaches a key/value pair to the Endpoint and returns the Endpoint.
// This can be used to pass additional data through the stages of ExternalDNS's Endpoint processing.
// The assumption is that most of the time this will be provider specific metadata that doesn't
// warrant its own field on the Endpoint object itself. It differs from Labels in the fact that it's
// not persisted in the Registry but only kept in memory during a single record synchronization.
func (e *Endpoint) WithProviderSpecific(key, value string) *Endpoint {
	if e.ProviderSpecific == nil {
		e.ProviderSpecific = ProviderSpecific{}
	}

	e.ProviderSpecific = append(e.ProviderSpecific, ProviderSpecificProperty{Name: key, Value: value})
	return e
}

// GetProviderSpecificProperty returns a ProviderSpecificProperty if the property exists.
func (e *Endpoint) GetProviderSpecificProperty(key string) (ProviderSpecificProperty, bool) {
	for _, providerSpecific := range e.ProviderSpecific {
		if providerSpecific.Name == key {
			return providerSpecific, true
		}
	}
	return ProviderSpecificProperty{}, false
}

func (e *Endpoint) String() string {
	return fmt.Sprintf("%s %d IN %s %s %s %s", e.DNSName, e.RecordTTL, e.RecordType, e.SetIdentifier, e.Targets, e.ProviderSpecific)
}

// DNSRecordSpec defines the desired state of DNSRecord
type DNSRecordSpec struct {
	// +kubebuilder:validation:MinItems=1
	// +optional
	Endpoints []*Endpoint `json:"endpoints,omitempty"`

	// +kubebuilder:validation:Required
	// +required
	ProviderRef ProviderRef `json:"providerRef"`

	// +optional
	ZoneID *string `json:"zoneID,omitempty"`
}

// DNSRecordStatus defines the observed state of DNSRecord
type DNSRecordStatus struct {

	// conditions are any conditions associated with the record in the managed zone.
	//
	// If publishing the record fails, the "Failed" condition will be set with a
	// reason and message describing the cause of the failure.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recently observed generation of the
	// DNSRecord.  When the DNSRecord is updated, the controller updates the
	// corresponding record in each managed zone.  If an update for a
	// particular zone fails, that failure is recorded in the status
	// condition for the zone so that the controller can determine that it
	// needs to retry the update for that specific zone.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// endpoints are the last endpoints that were successfully published by the provider
	//
	// Provides a simple mechanism to store the current provider records in order to
	// delete any that are no longer present in DNSRecordSpec.Endpoints
	//
	// Note: This will not be required if/when we switch to using external-dns since when
	// running with a "sync" policy it will clean up unused records automatically.
	Endpoints []*Endpoint `json:"endpoints,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="DNSRecord ready."

// DNSRecord is the Schema for the dnsrecords API
type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSRecordSpec   `json:"spec,omitempty"`
	Status DNSRecordStatus `json:"status,omitempty"`
}

func (p *DNSRecord) GetProviderRef() ProviderRef {
	return p.Spec.ProviderRef
}

//+kubebuilder:object:root=true

// DNSRecordList contains a list of DNSRecord
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSRecord `json:"items"`
}

// DNSRecordType is a DNS resource record type.
// +kubebuilder:validation:Enum=CNAME;A
type DNSRecordType string

const (
	// CNAMERecordType is an RFC 1035 CNAME record.
	CNAMERecordType DNSRecordType = "CNAME"

	// ARecordType is an RFC 1035 A record.
	ARecordType DNSRecordType = "A"

	// NSRecordType is a name server record.
	NSRecordType DNSRecordType = "NS"
)

const (
	TargetTypeHost = "HOST"
	TargetTypeIP   = "IP"
)

type Target struct {
	Cluster    string
	TargetType string
	Value      string
}

func (endpoint *Endpoint) GetAddress() (string, bool) {
	if endpoint.SetIdentifier == "" || len(endpoint.Targets) == 0 {
		return "", false
	}

	return string(endpoint.Targets[0]), true
}

func (endpoint *Endpoint) SetProviderSpecific(name, value string) {
	if endpoint.ProviderSpecific == nil {
		endpoint.ProviderSpecific = ProviderSpecific{}
	}

	for i, pair := range endpoint.ProviderSpecific {
		if pair.Name == name {
			endpoint.ProviderSpecific[i].Value = value
			return
		}
	}

	endpoint.ProviderSpecific = append(endpoint.ProviderSpecific, ProviderSpecificProperty{
		Name:  name,
		Value: value,
	})
}

func (endpoint *Endpoint) GetProviderSpecific(name string) (string, bool) {
	for _, property := range endpoint.ProviderSpecific {
		if property.Name == name {
			return property.Value, true
		}
	}

	return "", false
}

func (endpoint *Endpoint) DeleteProviderSpecific(name string) bool {
	if endpoint.ProviderSpecific == nil {
		return false
	}

	deleted := false
	providerSpecific := make(ProviderSpecific, 0, len(endpoint.ProviderSpecific))
	for _, pair := range endpoint.ProviderSpecific {
		if pair.Name == name {
			deleted = true
		} else {
			providerSpecific = append(providerSpecific, pair)
		}
	}

	endpoint.ProviderSpecific = providerSpecific
	return deleted
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}
