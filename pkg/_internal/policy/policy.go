package policy

import (
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"
)

const (
	TARGET_REF_KEY = "policyTargetRef"
)

func GetTargetRefValueFromPolicy(policy common.KuadrantPolicy) string {
	targetRef := policy.GetTargetRef()
	ns := targetRef.Namespace
	if ns == nil {
		policyTypedNamespace := gatewayapiv1.Namespace(policy.GetNamespace())
		ns = &policyTypedNamespace
	}
	return string(targetRef.Name) + "," + string(*ns)
}
