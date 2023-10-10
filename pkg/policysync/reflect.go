package policysync

import (
	"errors"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

const (
	PolicyTargetReferencePath = "sigs.k8s.io/gateway-api/apis/v1alpha2/PolicyTargetReference"
)

type ReflectPolicy struct {
	metav1.Object

	policy *GenericPolicy
}

var _ Policy = &ReflectPolicy{}

type GenericPolicy struct {
	TargetRef *gatewayapiv1alpha2.PolicyTargetReference
}

func (p *ReflectPolicy) GetTargetRef() *gatewayapiv1alpha2.PolicyTargetReference {
	if p.policy == nil {
		policy := p.buildPolicy()
		p.policy = &policy
	}

	return p.policy.TargetRef
}

func (p *ReflectPolicy) SetTargetRef(targetRef *gatewayapiv1alpha2.PolicyTargetReference) {
	obj := reflect.ValueOf(p.Object).Elem()

	specValue := obj.FieldByName("Spec")
	targetRefValue := specValue.FieldByName("TargetRef")

	var valueToSet reflect.Value
	if targetRefValue.Kind() == reflect.Struct {
		valueToSet = reflect.ValueOf(*targetRef)
	} else if targetRefValue.Kind() == reflect.Pointer {
		valueToSet = reflect.ValueOf(targetRef)
	}

	targetRefValue.Set(valueToSet)

	p.policy.TargetRef = targetRef
}

func (p *ReflectPolicy) UpdateTargetRef(update func(*gatewayapiv1alpha2.PolicyTargetReference)) {
	targetRef := p.GetTargetRef()
	if targetRef == nil {
		return
	}

	update(targetRef)

	p.SetTargetRef(targetRef)
}

func (p *ReflectPolicy) buildPolicy() GenericPolicy {
	obj := reflect.ValueOf(p.Object).Elem()

	specValue := obj.FieldByName("Spec")
	targetRefValue := specValue.FieldByName("TargetRef")

	if targetRefValue.Kind() == reflect.Struct {
		targetRefValue = targetRefValue.Addr()
	}

	return GenericPolicy{
		TargetRef: targetRefValue.Interface().(*gatewayapiv1alpha2.PolicyTargetReference),
	}
}

func (p *ReflectPolicy) IsValidPolicy() error {
	objType := reflect.TypeOf(p.Object)
	specType, ok := objType.Elem().FieldByName("Spec")
	if !ok {
		return errors.New("field .Spec missing from object")
	}

	targetRefType, ok := specType.Type.FieldByName("TargetRef")
	if !ok {
		return errors.New("field .Spec.TargetRef missing from object")
	}

	typeAndPkg := fmt.Sprintf("%s/%s", targetRefType.Type.PkgPath(), targetRefType.Type.Name())

	if typeAndPkg != PolicyTargetReferencePath {
		return fmt.Errorf("type of .Spec.TargetRef %s not valid. Expected %s", typeAndPkg, PolicyTargetReferencePath)
	}

	return nil
}
