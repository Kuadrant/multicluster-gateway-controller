//go:build unit

package mutator

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer"
)

func Test_JSONPatch(t *testing.T) {
	unit := &JSONPatch{}
	config := syncer.MutatorConfig{
		ClusterID: "test_cluster_id",
		Logger:    testr.New(t),
	}
	scenarios := []struct {
		name   string //for name of test
		obj    interface{}
		verify func(objBefore, objAfter *unstructured.Unstructured, err error, t *testing.T) //what we want to verify
	}{
		{
			name: "no patches",
			obj: &gatewayapi.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec:   gatewayapi.GatewaySpec{},
				Status: gatewayapi.GatewayStatus{},
			},
			verify: func(objBefore, objAfter *unstructured.Unstructured, err error, t *testing.T) {
				if err != nil {
					t.Fatalf("unexpected error from mutator: %v", err.Error())
				}

				gwBefore := &gatewayapi.Gateway{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(objBefore.Object, gwBefore)
				if err != nil {
					t.Fatalf("unexpected error converting unstructured to gateway: %v", err.Error())
				}

				gwAfter := &gatewayapi.Gateway{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(objAfter.Object, gwBefore)
				if err != nil {
					t.Fatalf("unexpected error converting unstructured to gateway: %v", err.Error())
				}
				if !equality.Semantic.DeepEqual(gwBefore, gwAfter) {
					t.Fatalf("expected gateways to match, before: %+v, after: %+v", gwBefore, gwAfter)
				}
			},
		},
		{
			name: "spec patch",
			obj: &gatewayapi.Gateway{
				TypeMeta: metav1.TypeMeta{
					Kind:       "gateway",
					APIVersion: "v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						JSONPatchAnnotationPrefix + config.ClusterID: `[
							{"op": "replace", "path": "/spec/gatewayClassName", "value": "boo"},
							{"op": "replace", "path": "/spec/listeners/0/name", "value": "test"}
						]`,
					},
				},
				Spec: gatewayapi.GatewaySpec{
					GatewayClassName: "test-class-name",
					Listeners: []gatewayapi.Listener{
						{
							Name:     "test-listener-1",
							Port:     8443,
							Protocol: gatewayapi.HTTPSProtocolType,
						},
					},
				},
				Status: gatewayapi.GatewayStatus{},
			},
			verify: func(objBefore, objAfter *unstructured.Unstructured, err error, t *testing.T) {
				if err != nil {
					t.Fatalf("unexpected error from mutator: %v", err.Error())
				}

				gwBefore := &gatewayapi.Gateway{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(objBefore.Object, gwBefore)
				if err != nil {
					t.Fatalf("unexpected error converting unstructured to gateway: %v", err.Error())
				}

				gwAfter := &gatewayapi.Gateway{}
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(objAfter.Object, gwAfter)
				if err != nil {
					t.Fatalf("unexpected error converting unstructured to gateway: %v", err.Error())
				}

				if equality.Semantic.DeepEqual(gwBefore, gwAfter) {
					t.Fatalf("expected gateways to not match, before: %+v, after: %+v", gwBefore, gwAfter)
				}

				if gwAfter.Spec.GatewayClassName != "boo" {
					t.Fatalf("expected modified gateway to have className 'boo' found: %v", gwAfter.Spec.GatewayClassName)
				}

				if gwAfter.Spec.Listeners == nil {
					t.Fatalf("expected modified gateway to have listeners")
				}

				if len(gwAfter.Spec.Listeners) != 1 {
					t.Fatalf("expected modified gateway to have exactly 1 listener, found: %v", len(gwAfter.Spec.Listeners))
				}

				if gwAfter.Spec.Listeners[0].Name != "test" {
					t.Fatalf("expected modified listener to have name 'test' found: %v", gwAfter.Spec.Listeners[0].Name)
				}
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			//convert scenario object to an unstructured
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(scenario.obj)
			unstrBefore := &unstructured.Unstructured{
				Object: unstrObj,
			}
			//store the pre-mutate state
			unstrAfter := unstrBefore.DeepCopy()
			if err != nil {
				t.Errorf("unexpected error creating unstructured object: '%v'", err)
			}

			//mutate into after state
			err = unit.Mutate(config, unstrAfter)
			//very before/after changes
			scenario.verify(unstrBefore, unstrAfter, err, t)
		})
	}
}
