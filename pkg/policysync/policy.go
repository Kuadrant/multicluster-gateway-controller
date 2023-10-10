package policysync

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

type Policy interface {
	metav1.Object

	// GetTargetRef returns a copy of the TargetRef field of the policy.
	//
	// Mutating the return value of this function doesn't change the original
	// policy. Use SetTargetRef or UpdateTargetRef for that
	GetTargetRef() *gatewayapiv1alpha2.PolicyTargetReference

	// SetTargetRef replaces the TargetRef field of the policy with targetRef
	SetTargetRef(targetRef *gatewayapiv1alpha2.PolicyTargetReference)

	// UpdateTargetRef mutates the TargetRef field of the policy by applying
	// update() to it
	UpdateTargetRef(update func(*gatewayapiv1alpha2.PolicyTargetReference))

	// IsValidPolicy validates that the object is a valid Gateway policy
	IsValidPolicy() error
}

// NewPolicyFor attempts to create a Policy instance for obj, or returns an
// error if the object is not a valid Gateway Policy
func NewPolicyFor(obj interface{}) (Policy, error) {
	if _, ok := obj.(metav1.Object); !ok {
		return nil, errors.New("object doesn't implement metav1.Object interface")
	}

	var policy Policy

	switch typedObj := obj.(type) {
	case *unstructured.Unstructured:
		policy = &UnstructuredPolicy{Unstructured: typedObj}
	default:
		policy = &ReflectPolicy{Object: obj.(metav1.Object)}
	}

	if err := policy.IsValidPolicy(); err != nil {
		return nil, err
	}

	return policy, nil
}
