package policy

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

const (
	POLICY_TARGET_REF_KEY = "policyTargetRef"
)

func GetTargetRefValueFromPolicy(policy common.KuadrantPolicy) string {
	targetRef := policy.GetTargetRef()
	ns := targetRef.Namespace
	if ns == nil {
		policyTypedNamespace := gatewayv1beta1.Namespace(policy.GetNamespace())
		ns = &policyTypedNamespace
	}
	return string(targetRef.Name) + "," + string(*ns)
}

func NewDefaultDNSPolicy(gateway *gatewayv1beta1.Gateway) v1alpha1.DNSPolicy {
	gatewayTypedNamespace := gatewayv1beta1.Namespace(gateway.Namespace)
	return v1alpha1.DNSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: v1alpha1.DNSPolicySpec{
			TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
				Group:     gatewayv1beta1.Group(gatewayv1beta1.GroupVersion.Group),
				Kind:      "Gateway",
				Name:      gatewayv1beta1.ObjectName(gateway.Name),
				Namespace: &gatewayTypedNamespace,
			},
		},
	}
}
