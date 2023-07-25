//go:build unit

package policy

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestGetTargetRefValueFromPolicy(t *testing.T) {
	type args struct {
		policy common.KuadrantPolicy
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should use target namespace",
			args: args{
				policy: &v1alpha1.DNSPolicy{

					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "test-policy-ns",
					},
					Spec: v1alpha1.DNSPolicySpec{
						TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
							Group: "gateway.networking.k8s.io",
							Kind:  "Gateway",
							Name:  "test-gateway",
						},
					},
				},
			},
			want: "test-gateway,test-policy-ns",
		},
		{
			name: "should use policy namespace when no target namespace set",
			args: args{
				policy: &v1alpha1.DNSPolicy{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "test-policy-ns",
					},
					Spec: v1alpha1.DNSPolicySpec{
						TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
							Group:     "gateway.networking.k8s.io",
							Kind:      "Gateway",
							Name:      "test-gateway",
							Namespace: testutil.Pointer(gatewayv1beta1.Namespace("test-gateway-ns")),
						},
					},
				},
			},
			want: "test-gateway,test-gateway-ns",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := GetTargetRefValueFromPolicy(tt.args.policy)
			if tt.want != expected {
				t.Errorf("GetTargetRefValueFromPolicy returned %v; expected %v", tt.want, expected)
			}
		})
	}
}
