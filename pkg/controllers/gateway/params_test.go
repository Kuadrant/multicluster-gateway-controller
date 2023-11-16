//go:build unit

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
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestGetParams(t *testing.T) {
	cases := []struct {
		name         string
		gatewayClass *gatewayapiv1.GatewayClass
		paramsObj    client.Object
		assertParams func(*Params, error) error
	}{
		{
			name: "ConfigMap found",
			gatewayClass: &gatewayapiv1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: &gatewayapiv1.ParametersReference{
						Group:     "",
						Kind:      "ConfigMap",
						Name:      testutil.DummyCRName,
						Namespace: testutil.Pointer(gatewayapiv1.Namespace(testutil.Namespace)),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
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
			name: "ConfigMap found. Uses default params on missing ref",
			gatewayClass: &gatewayapiv1.GatewayClass{
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: nil,
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
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
			name: "Misconfigured ConfigMap",
			gatewayClass: &gatewayapiv1.GatewayClass{
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: &gatewayapiv1.ParametersReference{
						Group:     "",
						Kind:      "ConfigMap",
						Name:      testutil.DummyCRName,
						Namespace: testutil.Pointer(gatewayapiv1.Namespace(testutil.Namespace)),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
				},
				Data: map[string]string{
					"parameters": `{"downstreamClass": "istio"}`,
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
		{
			name: "Corrupted data field ConfigMap",
			gatewayClass: &gatewayapiv1.GatewayClass{
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: &gatewayapiv1.ParametersReference{
						Group:     "",
						Kind:      "ConfigMap",
						Name:      testutil.DummyCRName,
						Namespace: testutil.Pointer(gatewayapiv1.Namespace(testutil.Namespace)),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
				},
				Data: map[string]string{
					"params": `{"downstreamClass": "istio" boop`,
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
		{
			name: "Missing namespace",
			gatewayClass: &gatewayapiv1.GatewayClass{
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: &gatewayapiv1.ParametersReference{
						Group: "",
						Kind:  "ConfigMap",
						Name:  testutil.DummyCRName,
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
				},
				Data: map[string]string{
					"params": `{"downstreamClass": "istio"}`,
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
		{
			name: "ConfigMap not found",

			gatewayClass: &gatewayapiv1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: &gatewayapiv1.ParametersReference{
						Group:     "",
						Kind:      "ConfigMap",
						Name:      "test-params-no-exist",
						Namespace: testutil.Pointer(gatewayapiv1.Namespace(testutil.Namespace)),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
		{
			name: "Unsupported GroupKind",

			gatewayClass: &gatewayapiv1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: gatewayapiv1.GatewayClassSpec{
					ParametersRef: &gatewayapiv1.ParametersReference{
						Group:     "foo",
						Kind:      "Unsupported",
						Name:      testutil.DummyCRName,
						Namespace: testutil.Pointer(gatewayapiv1.Namespace(testutil.Namespace)),
					},
				},
			},
			paramsObj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.DummyCRName,
					Namespace: testutil.Namespace,
				},
			},
			assertParams: assertError(IsInvalidParamsError),
		},
	}

	scheme := runtime.NewScheme()
	if err := gatewayapiv1.AddToScheme(scheme); err != nil {
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

			params, err := getParams(context.TODO(), client, testCase.gatewayClass.Name)

			if err := testCase.assertParams(params, err); err != nil {
				t.Error(err)
			}
		})
	}
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
