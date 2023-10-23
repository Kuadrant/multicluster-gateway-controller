//go:build unit

package policy

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/kuadrant/kuadrant-operator/pkg/common"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestGetTargetRefValueFromPolicy(t *testing.T) {
	type args struct {
		policy common.KuadrantPolicy
	}
	testCases := []struct {
		name string
		args args
		want string
	}{
		{
			name: "should use target namespace",
			args: args{
				policy: &v1alpha2.DNSPolicy{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "test-policy-ns",
					},
					Spec: v1alpha2.DNSPolicySpec{
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
				policy: &v1alpha2.DNSPolicy{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-policy",
						Namespace: "test-policy-ns",
					},
					Spec: v1alpha2.DNSPolicySpec{
						TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
							Group:     "gateway.networking.k8s.io",
							Kind:      "Gateway",
							Name:      "test-gateway",
							Namespace: (*gatewayapiv1.Namespace)(testutil.Pointer("test-gateway-ns")),
						},
					},
				},
			},
			want: "test-gateway,test-gateway-ns",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			expected := GetTargetRefValueFromPolicy(testCase.args.policy)
			if testCase.want != expected {
				t.Errorf("GetTargetRefValueFromPolicy returned %v; expected %v", testCase.want, expected)
			}
		})
	}
}
