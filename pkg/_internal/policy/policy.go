package policy

import (
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"
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
