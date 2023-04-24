package gateway

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestGetParams(t *testing.T) {
	cases := []struct {
		name string

		gatewayClass *gatewayv1beta1.GatewayClass
		paramsObj    client.Object
		assertParams func(*Params, error) error
	}{
		{
			name: "ConfigMap found",
			gatewayClass: &gatewayv1beta1.GatewayClass{
				Spec: gatewayv1beta1.GatewayClassSpec{
					ParametersRef: &gatewayv1beta1.ParametersReference{
						Group:     "",
						Kind:      "ConfigMap",
						Name:      "test-params",
						Namespace: addr(gatewayv1beta1.Namespace("test-ns")),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-params",
					Namespace: "test-ns",
				},
				Data: map[string]string{
					"params": `{"downstreamClass": "istio"}`,
				},
			},
			assertParams: and(
				noError,
				paramsEqual(Params{
					DownstreamClass: "istio",
				}),
			),
		},
		{
			name: "ConfigMap not found",

			gatewayClass: &gatewayv1beta1.GatewayClass{
				Spec: gatewayv1beta1.GatewayClassSpec{
					ParametersRef: &gatewayv1beta1.ParametersReference{
						Group:     "",
						Kind:      "ConfigMap",
						Name:      "test-params-no-exist",
						Namespace: addr(gatewayv1beta1.Namespace("test-ns")),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-params",
					Namespace: "test-ns",
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
		{
			name: "Unsupported GroupKind",

			gatewayClass: &gatewayv1beta1.GatewayClass{
				Spec: gatewayv1beta1.GatewayClassSpec{
					ParametersRef: &gatewayv1beta1.ParametersReference{
						Group:     "foo",
						Kind:      "Unsupported",
						Name:      "test-params",
						Namespace: addr(gatewayv1beta1.Namespace("test-ns")),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-params",
					Namespace: "test-ns",
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
	}

	scheme := runtime.NewScheme()
	if err := gatewayv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("unexpected error building scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("unexpected error building scheme: %v", err)
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(testCase.gatewayClass, testCase.paramsObj).
				Build()

			params, err := getParams(context.TODO(), client, testCase.gatewayClass)

			if err := testCase.assertParams(params, err); err != nil {
				t.Error(err)
			}
		})
	}
}

func addr[T any](value T) *T {
	return &value
}

// Assertion utils

func and(assertions ...func(*Params, error) error) func(*Params, error) error {
	return func(p *Params, err error) error {
		for _, assertion := range assertions {
			if err := assertion(p, err); err != nil {
				return err
			}
		}

		return nil
	}
}

func noError(_ *Params, err error) error {
	if err != nil {
		return fmt.Errorf("unexpected error: %v", err)
	}

	return nil
}

func assertError(assertion func(error) bool) func(*Params, error) error {
	return func(p *Params, err error) error {
		if !assertion(err) {
			return fmt.Errorf("failed to assert error %v", err)
		}

		return nil
	}
}

func paramsEqual(expected Params) func(*Params, error) error {
	return func(p *Params, _ error) error {
		got := *p
		if reflect.DeepEqual(got, expected) {
			return nil
		}

		return fmt.Errorf("unexpected params. Expected %v, got %v", expected, got)
	}
}
