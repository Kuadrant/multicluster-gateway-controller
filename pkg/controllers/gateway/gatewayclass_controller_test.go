//go:build unit

package gateway

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestGatewayClassReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		req ctrl.Request
	}
	testCases := []struct {
		name   string
		fields fields
		args   args
		verify func(client client.Client, res ctrl.Result, err error, t *testing.T)
	}{
		{
			name: "Gateway class already accepted",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&gatewayv1beta1.GatewayClassList{
						Items: []gatewayv1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: testutil.DummyCRName,
								},
								Status: gatewayv1beta1.GatewayClassStatus{
									Conditions: []v1.Condition{
										{
											Type:   string(gatewayv1beta1.GatewayConditionAccepted),
											Status: v1.ConditionTrue,
										},
									},
								},
							},
						},
					},
				),
			},
			args: args{
				req: buildGCTestRequest(),
			},
			verify: verifyGatewayClassAcceptance(testutil.DummyCRName, true),
		},
		{
			name: "Gateway class being accepted",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&gatewayv1beta1.GatewayClassList{
						Items: []gatewayv1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: getSupportedClasses()[0],
								},
								Spec: gatewayv1beta1.GatewayClassSpec{
									ParametersRef: &gatewayv1beta1.ParametersReference{
										Group:     "",
										Kind:      "ConfigMap",
										Name:      "test-params",
										Namespace: testutil.Pointer(gatewayv1beta1.Namespace(testutil.Namespace)),
									},
								},
							},
						},
					},
					&corev1.ConfigMapList{
						Items: []corev1.ConfigMap{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "test-params",
									Namespace: testutil.Namespace,
								},
								Data: map[string]string{
									"params": `{"downstreamClass": "istio"}`,
								},
							},
						},
					},
				),
			},
			args: args{
				req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name: getSupportedClasses()[0],
					},
				},
			},
			verify: verifyGatewayClassAcceptance(getSupportedClasses()[0], true),
		},
		{
			name: "Unsupported class name",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&gatewayv1beta1.GatewayClassList{
						Items: []gatewayv1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: testutil.DummyCRName,
								},
							},
						},
					},
				),
			},
			args: args{
				req: buildGCTestRequest(),
			},
			verify: verifyGatewayClassAcceptance(testutil.DummyCRName, false),
		},
		{
			name: "Invalid Parameters in config map",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&gatewayv1beta1.GatewayClassList{
						Items: []gatewayv1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: getSupportedClasses()[0],
								},
								Spec: gatewayv1beta1.GatewayClassSpec{
									ParametersRef: &gatewayv1beta1.ParametersReference{
										Group:     "",
										Kind:      "ConfigMap",
										Name:      "test-params",
										Namespace: testutil.Pointer(gatewayv1beta1.Namespace("boop-namespace")),
									},
								},
							},
						},
					},
					&corev1.ConfigMapList{
						Items: []corev1.ConfigMap{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:      "test-params",
									Namespace: testutil.Namespace,
								},
								Data: map[string]string{
									"params": `{"downstreamClass": "istio" boop`,
								},
							},
						},
					},
				),
			},
			args: args{
				req: ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name: getSupportedClasses()[0],
					},
				},
			},
			verify: verifyGatewayClassAcceptance(getSupportedClasses()[0], false),
		},
		{
			name: "Gateway class not found",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&gatewayv1beta1.GatewayClassList{
						Items: []gatewayv1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: getSupportedClasses()[0],
								},
							},
						},
					},
				),
			},
			args: args{
				req: buildGCTestRequest(),
			},
			verify: func(_ client.Client, _ ctrl.Result, err error, t *testing.T) {
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := &GatewayClassReconciler{
				Client: testCase.fields.Client,
				Scheme: testutil.GetValidTestScheme(),
			}
			res, err := r.Reconcile(context.TODO(), testCase.args.req)
			testCase.verify(testCase.fields.Client, res, err, t)
		})
	}
}

func buildGCTestRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: testutil.DummyCRName,
		},
	}
}

func verifyGatewayClassAcceptance(name string, want bool) func(c client.Client, res ctrl.Result, err error, t *testing.T) {
	return func(c client.Client, res ctrl.Result, err error, t *testing.T) {
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		class := &gatewayv1beta1.GatewayClass{}
		err = c.Get(context.TODO(), client.ObjectKey{Name: name}, class)
		if err != nil {
			t.Fatalf("error getting gateway class from client: %s", err)
		}
		if want != gatewayClassIsAccepted(class) {
			t.Fatalf("controller ignored or not accepted gateway class")
		}
	}
}
