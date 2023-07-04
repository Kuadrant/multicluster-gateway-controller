/*
Copyright 2023 The MultiCluster Traffic Controller Authors.

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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// DNSPolicySpec defines the desired state of DNSPolicy
type DNSPolicySpec struct {

	// +kubebuilder:validation:Required
	// +required
	TargetRef gatewayapiv1alpha2.PolicyTargetReference `json:"targetRef"`

	// +optional
	HealthCheck *HealthCheckSpec `json:"healthCheck,omitempty"`
}

// DNSPolicyStatus defines the observed state of DNSPolicy
type DNSPolicyStatus struct {

	// conditions are any conditions associated with the policy
	//
	// If configuring the policy fails, the "Failed" condition will be set with a
	// reason and message describing the cause of the failure.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// observedGeneration is the most recently observed generation of the
	// DNSPolicy.  When the DNSPolicy is updated, the controller updates the
	// corresponding configuration. If an update fails, that failure is
	// recorded in the status condition
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	HealthCheck *HealthCheckStatus `json:"healthCheck,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="DNSPolicy ready."

// DNSPolicy is the Schema for the dnspolicies API
type DNSPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSPolicySpec   `json:"spec,omitempty"`
	Status DNSPolicyStatus `json:"status,omitempty"`
}

func (p *DNSPolicy) GetWrappedNamespace() gatewayv1beta1.Namespace {
	return gatewayv1beta1.Namespace(p.Namespace)
}

func (p *DNSPolicy) GetRulesHostnames() []string {
	//TODO implement me
	panic("implement me")
}

func (p *DNSPolicy) GetTargetRef() gatewayapiv1alpha2.PolicyTargetReference {
	return p.Spec.TargetRef
}

func (p *DNSPolicy) Validate() error {
	if p.Spec.TargetRef.Group != ("gateway.networking.k8s.io") {
		return fmt.Errorf("invalid targetRef.Group %s. The only supported group is gateway.networking.k8s.io", p.Spec.TargetRef.Group)
	}

	if p.Spec.TargetRef.Kind != ("Gateway") {
		return fmt.Errorf("invalid targetRef.Kind %s. The only supported kind is Gateway", p.Spec.TargetRef.Kind)
	}

	if p.Spec.TargetRef.Namespace != nil && string(*p.Spec.TargetRef.Namespace) != p.Namespace {
		return fmt.Errorf("invalid targetRef.Namespace %s. Currently only supporting references to the same namespace", *p.Spec.TargetRef.Namespace)
	}

	return nil
}

//+kubebuilder:object:root=true

// DNSPolicyList contains a list of DNSPolicy
type DNSPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSPolicy `json:"items"`
}

// HealthCheckSpec configures health checks in the DNS provider.
// By default this health check will be applied to each unique DNS A Record for
// the listeners assigned to the target gateway
type HealthCheckSpec struct {
	Endpoint         string          `json:"endpoint,omitempty"`
	Port             *int            `json:"port,omitempty"`
	Protocol         *HealthProtocol `json:"protocol,omitempty"`
	FailureThreshold *int            `json:"failureThreshold,omitempty"`
}

type HealthCheckStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type DNSRecordRef struct {
	// +kubebuilder:validation:Required
	// +required
	Name string `json:"name"`
	// +kubebuilder:validation:Required
	// +required
	Namespace string `json:"namespace"`
}

type HealthProtocol string

const HttpProtocol HealthProtocol = "HTTP"

func init() {
	SchemeBuilder.Register(&DNSPolicy{}, &DNSPolicyList{})
}

func NewDefaultDNSPolicy(gateway *gatewayv1beta1.Gateway) DNSPolicy {
	gatewayTypedNamespace := gatewayv1beta1.Namespace(gateway.Namespace)
	return DNSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: DNSPolicySpec{
			TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
				Group:     gatewayv1beta1.Group(gatewayv1beta1.GroupVersion.Group),
				Kind:      "Gateway",
				Name:      gatewayv1beta1.ObjectName(gateway.Name),
				Namespace: &gatewayTypedNamespace,
			},
		},
	}
}
