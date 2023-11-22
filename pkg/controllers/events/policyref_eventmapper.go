package events

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
)

// PolicyRefEventMapper is an EventHandler that maps object events to policy events through the objects PolicyRef
type PolicyRefEventMapper struct {
	Logger     logr.Logger
	PolicyKind string
	PolicyRef  string
}

func (p *PolicyRefEventMapper) MapToPolicy(_ context.Context, obj client.Object) []reconcile.Request {
	return p.mapToPolicyRequest(obj, p.PolicyRef, p.PolicyKind)
}

func NewPolicyRefEventMapper(logger logr.Logger, policyRef, policyKind string) *PolicyRefEventMapper {
	return &PolicyRefEventMapper{
		Logger:     logger.WithName("PolicyRefEventMapper"),
		PolicyKind: policyKind,
		PolicyRef:  policyRef,
	}
}

func (p *PolicyRefEventMapper) mapToPolicyRequest(o client.Object, policyRef, policyKind string) []reconcile.Request {
	logger := p.Logger.V(3).WithValues("object", client.ObjectKeyFromObject(o))

	obj, ok := o.(metav1.Object)
	if !ok {
		logger.Info("mapToPolicyRequest:", "error", fmt.Sprintf("%T is not a metav1.Object", o))
		return []reconcile.Request{}
	}

	requests := make([]reconcile.Request, 0)

	policyName := metadata.GetLabel(obj, policyRef)
	if policyName == "" {
		return requests
	}
	policyNamespace := metadata.GetLabel(obj, fmt.Sprintf("%s-namespace", policyRef))
	if policyNamespace == "" {
		return requests
	}
	logger.Info("mapToPolicyRequest", policyKind, policyName)
	requests = append(requests, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      policyName,
			Namespace: policyNamespace,
		}})

	return requests
}
