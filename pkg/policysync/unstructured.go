package policysync

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayapiv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

type UnstructuredPolicy struct {
	*unstructured.Unstructured
}

var _ Policy = &UnstructuredPolicy{}

func (p *UnstructuredPolicy) GetTargetRef() *gatewayapiv1alpha2.PolicyTargetReference {
	targetRef := p.Object["spec"].(map[string]interface{})["targetRef"].(map[string]interface{})
	if targetRef == nil {
		return nil
	}

	var namespace *gatewayapiv1beta1.Namespace
	if targetRef["namespace"] != nil {
		ns := gatewayapiv1beta1.Namespace(targetRef["namespace"].(string))
		namespace = &ns
	}

	return &gatewayapiv1alpha2.PolicyTargetReference{
		Group:     gatewayapiv1beta1.Group(targetRef["group"].(string)),
		Kind:      gatewayapiv1beta1.Kind(targetRef["kind"].(string)),
		Name:      gatewayapiv1beta1.ObjectName(targetRef["name"].(string)),
		Namespace: namespace,
	}
}

func (p *UnstructuredPolicy) SetTargetRef(targetRef *gatewayapiv1alpha2.PolicyTargetReference) {
	var namespace *string
	if targetRef.Namespace != nil {
		ns := string(*targetRef.Namespace)
		namespace = &ns
	}

	asObject := map[string]interface{}{
		"group":     string(targetRef.Group),
		"kind":      string(targetRef.Kind),
		"name":      string(targetRef.Name),
		"namespace": namespace,
	}

	spec := p.Object["spec"].(map[string]interface{})
	spec["targetRef"] = asObject
}

func (p *UnstructuredPolicy) UpdateTargetRef(update func(*gatewayapiv1alpha2.PolicyTargetReference)) {
	targetRef := p.GetTargetRef()
	if targetRef == nil {
		return
	}

	update(targetRef)

	p.SetTargetRef(targetRef)
}

func (p *UnstructuredPolicy) IsValidPolicy() error {
	spec, ok := p.Object["spec"]
	if !ok {
		return errors.New("object missing .spec field")
	}

	specMap, ok := spec.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected .spec to be map[string]interface{} but got %v", spec)
	}

	targetRef, ok := specMap["targetRef"]
	if !ok {
		return errors.New("object missing .spec.targetRef field")
	}

	targetRefMap, ok := targetRef.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected .spec.targetRef to be map[string]interface{} but got %v", targetRef)
	}

	if err := validateMapContains[string]("name", targetRefMap); err != nil {
		return err
	}
	if err := validateMapContains[string]("group", targetRefMap); err != nil {
		return err
	}
	if err := validateMapContains[string]("kind", targetRefMap); err != nil {
		return err
	}

	return nil
}

func validateMapContains[T any](k string, m map[string]interface{}) error {
	value, ok := m[k]
	if !ok {
		return fmt.Errorf("field %s missing", k)
	}

	_, ok = value.(T)
	if !ok {
		return fmt.Errorf("invalid type of field %s %v", k, value)
	}

	return nil
}
