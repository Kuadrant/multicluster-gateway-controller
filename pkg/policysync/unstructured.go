package policysync

import (
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
	spec, err := ensureMapContains[map[string]interface{}]("spec", p.Object)
	if err != nil {
		return err
	}

	targetRef, err := ensureMapContains[map[string]interface{}]("targetRef", spec)
	if err != nil {
		return err
	}

	if _, err := ensureMapContains[string]("name", targetRef); err != nil {
		return err
	}
	if _, err := ensureMapContains[string]("group", targetRef); err != nil {
		return err
	}
	if _, err := ensureMapContains[string]("kind", targetRef); err != nil {
		return err
	}

	return nil
}

func ensureMapContains[T any](k string, m map[string]interface{}) (T, error) {
	var result T

	value, ok := m[k]
	if !ok {
		return result, fmt.Errorf("field %s is missing", k)
	}

	result, ok = value.(T)
	if !ok {
		return result, fmt.Errorf("invalid type of field %s %v", k, value)
	}

	return result, nil
}
