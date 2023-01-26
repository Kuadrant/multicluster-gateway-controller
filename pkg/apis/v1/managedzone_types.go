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

package v1

import (
	//ToDo Remove the need for the cert manager import
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagedZoneSpec defines the desired state of ManagedZone
type ManagedZoneSpec struct {
	//Root Domain of this ManagedZone
	RootDomain        string `json:"rootDomain"`
	DNSProviderConfig `json:",inline"`
}

// Only one of these can be set.
type DNSProviderConfig struct {
	// Route53 configures this managed zone to communicate with AWS Route 53
	// +optional
	Route53 *DNSProviderConfigRoute53 `json:"route53,omitempty"`
}

// DNSProviderConfigRoute53 is a structure containing the Route 53 configuration for AWS
type DNSProviderConfigRoute53 struct {
	AccessKeyID string `json:"accessKeyID"`

	SecretAccessKey cmmeta.SecretKeySelector `json:"secretAccessKeySecretRef"`

	HostedZoneID string `json:"hostedZoneID"`

	// Always set the region when using AccessKeyID and SecretAccessKey
	Region string `json:"region"`
}

// ManagedZoneStatus defines the observed state of ManagedZone
type ManagedZoneStatus struct {
	// List of status conditions to indicate the status of a ManagedZone.
	// Known condition types are `Ready`.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []ManagedZoneCondition `json:"conditions,omitempty"`
}

// ManagedZoneCondition contains condition information for an ManagedZone.
type ManagedZoneCondition struct {
	// Type of the condition, known values are (`Ready`).
	Type ManagedZoneConditionType `json:"type"`

	// Status of the condition, one of (`True`, `False`, `Unknown`).
	Status ConditionStatus `json:"status"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime is the timestamp corresponding to the last status
	// change of this condition.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ManagedZoneConditionType represents a ManagedZone condition value.
type ManagedZoneConditionType string

// ConditionStatus represents a condition's status.
// +kubebuilder:validation:Enum=True;False;Unknown
type ConditionStatus string

const (
	ManagedZoneConditionReady ManagedZoneConditionType = "Ready"

	// ConditionTrue represents the fact that a given condition is true
	ConditionTrue ConditionStatus = "True"

	// ConditionFalse represents the fact that a given condition is false
	ConditionFalse ConditionStatus = "False"

	// ConditionUnknown represents the fact that a given condition is unknown
	ConditionUnknown ConditionStatus = "Unknown"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Domain",type="string",JSONPath=".spec.rootDomain",description="Root domain of this Managed Zone"

// ManagedZone is the Schema for the managedzones API
type ManagedZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedZoneSpec   `json:"spec,omitempty"`
	Status ManagedZoneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManagedZoneList contains a list of ManagedZone
type ManagedZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedZone `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedZone{}, &ManagedZoneList{})
}
