//go:build unit

package policy

import (
	"testing"

	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

func TestGetTargetRefValueFromPolicy(t *testing.T) {
	t.Run("should use target namespace", func(t *testing.T) {
		policy := &v1alpha1.DNSPolicy{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "test-policy-ns",
			},
			Spec: v1alpha1.DNSPolicySpec{
				TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
					Group: "gateway.networking.k8s.io",
					Kind:  "Gateway",
					Name:  gatewayv1beta1.ObjectName("test-gateway"),
				},
			},
		}

		res := GetTargetRefValueFromPolicy(policy)
		expected := "test-gateway,test-policy-ns"
		if res != expected {
			t.Errorf("GetTargetRefValueFromPolicy returned %v; expected %v", res, expected)
		}
	})

	t.Run("should use policy namespace when no target namespace set", func(t *testing.T) {
		typedNamespace := gatewayv1beta1.Namespace("test-gateway-ns")
		policy := &v1alpha1.DNSPolicy{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "test-policy-ns",
			},
			Spec: v1alpha1.DNSPolicySpec{
				TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
					Group:     "gateway.networking.k8s.io",
					Kind:      "Gateway",
					Name:      gatewayv1beta1.ObjectName("test-gateway"),
					Namespace: &typedNamespace,
				},
			},
		}

		res := GetTargetRefValueFromPolicy(policy)
		expected := "test-gateway,test-gateway-ns"
		if res != expected {
			t.Errorf("GetTargetRefValueFromPolicy returned %v; expected %v", res, expected)
		}
	})
}
