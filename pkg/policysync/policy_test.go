package policysync

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kuadrantv1beta2 "github.com/kuadrant/kuadrant-operator/api/v1beta2"
)

func TestReflectPolicy(t *testing.T) {
	policy := &kuadrantv1beta2.AuthPolicy{
		Spec: kuadrantv1beta2.AuthPolicySpec{
			TargetRef: gatewayapiv1alpha2.PolicyTargetReference{
				Group: gatewayapiv1.Group("test.io"),
				Kind:  gatewayapiv1.Kind("Test"),
				Name:  gatewayapiv1.ObjectName("test"),
			},
		},
	}

	reflectPolicy := &ReflectPolicy{
		Object: policy,
	}

	if err := reflectPolicy.IsValidPolicy(); err != nil {
		t.Fatalf("expected policy to be valid, but failed with %v", err)
	}

	targetRef := reflectPolicy.GetTargetRef()
	if string(targetRef.Group) != "test.io" {
		t.Fatalf("expected targetRef.Group to be test.io, got %s", targetRef.Group)
	}
	if string(targetRef.Kind) != "Test" {
		t.Fatalf("expected targetRef.Kind to be Test, got %s", targetRef.Kind)
	}
	if string(targetRef.Name) != "test" {
		t.Fatalf("expected targetRef.Kind to be test, got %s", targetRef.Name)
	}

	reflectPolicy.UpdateTargetRef(func(targetRef *gatewayapiv1alpha2.PolicyTargetReference) {
		namespace := gatewayapiv1.Namespace("default")
		name := "changed-name"

		targetRef.Name = gatewayapiv1.ObjectName(name)
		targetRef.Namespace = &namespace
	})

	if string(policy.Spec.TargetRef.Name) != "changed-name" {
		t.Errorf("expected targetRef.Name to be changed-name, got %s", policy.Spec.TargetRef.Name)
	}
	if string(*policy.Spec.TargetRef.Namespace) != "default" {
		t.Errorf("expected targetRef.Namespace to be default, got %s", *policy.Spec.TargetRef.Namespace)
	}
}

func TestUnstructuredPolicy(t *testing.T) {
	policy := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": map[string]interface{}{
				"targetRef": map[string]interface{}{
					"name":  "test",
					"kind":  "Test",
					"group": "test.io",
				},
			},
		},
	}

	unstructuredPolicy := &UnstructuredPolicy{
		Unstructured: policy,
	}

	if err := unstructuredPolicy.IsValidPolicy(); err != nil {
		t.Fatalf("expected policy to be valid, but failed with %v", err)
	}

	targetRef := unstructuredPolicy.GetTargetRef()
	if string(targetRef.Group) != "test.io" {
		t.Fatalf("expected targetRef.Group to be test.io, got %s", targetRef.Group)
	}
	if string(targetRef.Kind) != "Test" {
		t.Fatalf("expected targetRef.Kind to be Test, got %s", targetRef.Kind)
	}
	if string(targetRef.Name) != "test" {
		t.Fatalf("expected targetRef.Kind to be test, got %s", targetRef.Name)
	}

	unstructuredPolicy.UpdateTargetRef(func(targetRef *gatewayapiv1alpha2.PolicyTargetReference) {
		namespace := gatewayapiv1.Namespace("default")
		name := "changed-name"

		targetRef.Name = gatewayapiv1.ObjectName(name)
		targetRef.Namespace = &namespace
	})

	actualName := policy.Object["spec"].(map[string]interface{})["targetRef"].(map[string]interface{})["name"].(string)
	if actualName != "changed-name" {
		t.Errorf("expected targetRef.Name to be changed-name, got %s", actualName)
	}
	actualNamespace := policy.Object["spec"].(map[string]interface{})["targetRef"].(map[string]interface{})["namespace"].(*string)
	if *actualNamespace != "default" {
		t.Errorf("expected targetRef.Namespace to be default, got %s", *actualNamespace)
	}
}
