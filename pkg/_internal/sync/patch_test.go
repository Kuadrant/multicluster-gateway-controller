//go:build unit

package sync

import (
	"fmt"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestPatchFor(t *testing.T) {
	patch, err := PatchForType(func(gateway *gatewayv1beta1.Gateway) {
		gateway.Spec.GatewayClassName = "my-class"
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(patch)

	obj := &gatewayv1beta1.Gateway{}
	SetPatchAnnotation(func(g *gatewayv1beta1.Gateway) {
		g.Spec.GatewayClassName = "test"
	}, "all", obj)

	t.Log(obj)
}

func TestSetPatchAnnotation(t *testing.T) {
	cases := []struct {
		name string

		object           client.Object
		mutation         func(client.Object)
		downstreamTarget string
		assertion        func(client.Object, error) error
	}{
		{
			name: "No annotation set",

			object:           &gatewayv1beta1.Gateway{},
			mutation:         func(o client.Object) {},
			downstreamTarget: "test-cluster",
			assertion: and(
				noError,
				assertAnnotation("mgc-syncer-patch/test-cluster", doesNotExist),
			),
		},
		{
			name: "Annotation set correctly",

			object: &gatewayv1beta1.Gateway{},
			mutation: func(o client.Object) {
				o.(*gatewayv1beta1.Gateway).Spec.GatewayClassName = "test"
			},
			downstreamTarget: "test-cluster",
			assertion: and(
				noError,
				assertAnnotation("mgc-syncer-patch/test-cluster",
					annotationEquals(`[{"op":"replace","path":"/spec/gatewayClassName","value":"test"}]`),
				),
			),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := SetPatchAnnotation(
				testCase.mutation,
				testCase.downstreamTarget,
				testCase.object,
			)

			if testErr := testCase.assertion(testCase.object, err); testErr != nil {
				t.Error(testErr)
			}
		})
	}
}

func noError(_ client.Object, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("unexpected error: %v", err)
}

func and(assertions ...func(client.Object, error) error) func(client.Object, error) error {
	return func(o client.Object, err error) error {
		for _, assertion := range assertions {
			if err := assertion(o, err); err != nil {
				return err
			}
		}

		return nil
	}
}

func assertAnnotation(key string, assert func(string, bool) error) func(client.Object, error) error {
	return func(o client.Object, err error) error {
		annotations := o.GetAnnotations()
		if annotations == nil {
			return assert("", false)
		}

		value, ok := annotations[key]
		return assert(value, ok)
	}
}

func doesNotExist(_ string, ok bool) error {
	if !ok {
		return nil
	}

	return fmt.Errorf("expected annotation to not exist")
}

func annotationEquals(expected string) func(string, bool) error {
	return func(s string, b bool) error {
		if s == expected {
			return nil
		}

		return fmt.Errorf("expected annotation value to be %s, but got %s", expected, s)
	}
}
