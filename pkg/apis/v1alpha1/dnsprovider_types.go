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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

type DNSProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec DnsProviderSpec `json:"spec,omitempty"`
}
type DnsProviderSpec struct {
	// +required
	Credentials *DNSProviderCredentials `json:"credentials"`
}

type DNSProviderCredentials struct {
	// +required
	Namespace string `json:"namespace"`

	Name string `json:"name,omitempty"`
}

type DNSProviderConfig struct {
	// Route53 configures this config to communicate with AWS Route 53
	// +optional
	Route53 *DNSProviderConfigRoute53 `json:"route53,omitempty"`
}

// DNSProviderConfigRoute53 is a structure containing the Route 53 configuration for AWS
type DNSProviderConfigRoute53 struct {
	AccessKeyID string `json:"accessKeyID,omitempty"`

	SecretAccessKey string `json:"SecretAccessKey,omitempty"`

	// Always set the region when using AccessKeyID and SecretAccessKey
	Region string `json:"region,omitempty"`
}

type ProviderRef struct {
	//+required
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
	ProviderType string `json:"type"`
}

// +kubebuilder:object:root=true
// DNSProviderList contains a list of DNSProvider
type DNSProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSProvider{}, &DNSProviderList{})
}
