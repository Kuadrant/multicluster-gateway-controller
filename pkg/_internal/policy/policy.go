package policy

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// GetRelatedObjects populates resultList with a list of objects that reference
// object Name and Namespace through backRefLabel.
func GetRelatedObjects(ctx context.Context, apiClient client.Client, object metav1.Object, resultList client.ObjectList, backRefLabel string) error {
	nameReq, err := labels.NewRequirement(backRefLabel, selection.Equals, []string{object.GetName()})
	if err != nil {
		return err
	}

	namespaceReq, err := labels.NewRequirement(fmt.Sprintf("%s-namespace", backRefLabel), selection.Equals, []string{object.GetNamespace()})
	if err != nil {
		return err
	}

	selector := labels.NewSelector().Add(*nameReq, *namespaceReq)

	if err := apiClient.List(ctx, resultList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return err
	}

	return nil
}
