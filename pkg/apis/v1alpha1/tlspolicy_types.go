/*
Copyright 2023.

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
	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// TLSPolicySpec defines the desired state of TLSPolicy
type TLSPolicySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	TargetRef   gatewayapiv1alpha2.PolicyTargetReference `json:"targetRef"`
	Certificate certman.CertificateSpec                  `json:",inline"`
}

// TLSPolicyStatus defines the observed state of TLSPolicy
type TLSPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TLSPolicy is the Schema for the tlspolicies API
type TLSPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TLSPolicySpec   `json:"spec,omitempty"`
	Status TLSPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TLSPolicyList contains a list of TLSPolicy
type TLSPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TLSPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TLSPolicy{}, &TLSPolicyList{})
}

func NewDefaultTLSPolicy(gateway *gatewayv1beta1.Gateway) TLSPolicy {
	gatewayTypedNamespace := gatewayv1beta1.Namespace(gateway.Namespace)
	return TLSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: TLSPolicySpec{
			TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
				Group:     gatewayv1beta1.Group(gatewayv1beta1.GroupVersion.Group),
				Kind:      "Gateway",
				Name:      gatewayv1beta1.ObjectName(gateway.Name),
				Namespace: &gatewayTypedNamespace,
			},
			Certificate: certman.CertificateSpec{
				IssuerRef: cmmeta.ObjectReference{
					Group: "cert-manager.io",
					Kind:  "ClusterIssuer",
					Name:  "glbc-ca",
				},
			},
		},
	}
}
