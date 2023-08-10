//go:build unit

package gateway

import (
	"context"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/placement"
	fakeplacement "github.com/Kuadrant/multicluster-gateway-controller/pkg/placement/fake"
	faketls "github.com/Kuadrant/multicluster-gateway-controller/pkg/tls/fake"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func getTestHostname(hostname string) *v1beta1.Hostname {
	hn := v1beta1.Hostname(hostname)
	return &hn
}

func TestGatewayReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		req ctrl.Request
	}
	testCases := []struct {
		name   string
		fields fields
		args   args
		verify func(res ctrl.Result, err error, t *testing.T)
	}{
		{
			name: "gateway reconciled and updated",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&v1beta1.GatewayList{
						Items: []v1beta1.Gateway{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:       testutil.DummyCRName,
									Namespace:  testutil.Namespace,
									Labels:     getTestGatewayLabels(),
									Finalizers: []string{GatewayFinalizer},
								},
								Spec: v1beta1.GatewaySpec{
									GatewayClassName: testutil.DummyCRName,
									Listeners: []v1beta1.Listener{
										{
											Name:     testutil.ValidTestHostname,
											Hostname: testutil.Pointer(v1beta1.Hostname(testutil.ValidTestHostname)),
											Protocol: v1beta1.HTTPSProtocolType,
										},
									},
								},
							},
						},
					},
					&v1beta1.GatewayClassList{
						Items: []v1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: testutil.DummyCRName,
								},
							},
						},
					},
					getValidCertificateSecret(certname(testutil.DummyCRName, testutil.ValidTestHostname)),
					buildTestMZ(),
					buildTestDNSRecord(),
				),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				req: testutil.BuildValidTestRequest(testutil.DummyCRName, testutil.Namespace),
			},
			verify: testutil.AssertNoErrorReconciliation(),
		},
		{
			name: "failed to fetch gateway",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithScheme(runtime.NewScheme()).
					Build(),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				req: ctrl.Request{},
			},
			verify: func(res ctrl.Result, err error, t *testing.T) {
				if !res.IsZero() || !strings.Contains(err.Error(), "no kind is registered") {
					t.Errorf("failed. Err: %s, res: %v", err, res)
				}
			},
		},
		{
			name: "no gateway is present",
			fields: fields{
				Client: testutil.GetValidTestClient(),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				req: ctrl.Request{},
			},
			verify: testutil.AssertNoErrorReconciliation(),
		},
		{
			name: "gateway is deleting",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&v1beta1.GatewayList{
						Items: []v1beta1.Gateway{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:              testutil.DummyCRName,
									Namespace:         testutil.Namespace,
									DeletionTimestamp: testutil.GetTime(),
								},
							},
						},
					},
				),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				req: testutil.BuildValidTestRequest(testutil.DummyCRName, testutil.Namespace),
			},
			verify: testutil.AssertNoErrorReconciliation(),
		},
		{
			name: "missing gateway class",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&v1beta1.GatewayList{
						Items: []v1beta1.Gateway{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:       testutil.DummyCRName,
									Namespace:  testutil.Namespace,
									Finalizers: []string{GatewayFinalizer},
								},
								Spec: v1beta1.GatewaySpec{
									GatewayClassName: testutil.DummyCRName,
								},
							},
						},
					},
				),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				req: testutil.BuildValidTestRequest(testutil.DummyCRName, testutil.Namespace),
			},
			verify: func(res ctrl.Result, err error, t *testing.T) {
				if !reflect.DeepEqual(res, ctrl.Result{}) &&
					!strings.Contains(err.Error(), "gatewayclasses") &&
					!strings.Contains(err.Error(), "not found") {
					t.Errorf("expected to fail finding gateway class but got err: %s ", err)
				}
			},
		},
		{
			name: "invalid params on class reference",
			fields: fields{
				Client: testutil.GetValidTestClient(
					&v1beta1.GatewayList{
						Items: []v1beta1.Gateway{
							{
								ObjectMeta: v1.ObjectMeta{
									Name:       testutil.DummyCRName,
									Namespace:  testutil.Namespace,
									Finalizers: []string{GatewayFinalizer},
								},
								Spec: v1beta1.GatewaySpec{
									GatewayClassName: testutil.DummyCRName,
								},
								Status: v1beta1.GatewayStatus{
									Conditions: []v1.Condition{
										{
											Type:   string(v1beta1.GatewayConditionProgrammed),
											Status: v1.ConditionTrue,
										},
									},
								},
							},
						},
					},
					&v1beta1.GatewayClassList{
						Items: []v1beta1.GatewayClass{
							{
								ObjectMeta: v1.ObjectMeta{
									Name: testutil.DummyCRName,
								},
								Spec: v1beta1.GatewayClassSpec{
									ParametersRef: &v1beta1.ParametersReference{
										Group: "boop",
										Kind:  "theCat",
									},
								},
							},
						},
					},
				),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				req: testutil.BuildValidTestRequest(testutil.DummyCRName, testutil.Namespace),
			},
			verify: testutil.AssertNoErrorReconciliation(),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := &GatewayReconciler{
				Client:       testCase.fields.Client,
				Scheme:       testCase.fields.Scheme,
				Certificates: faketls.NewTestCertificateService(testCase.fields.Client),
				Placement:    fakeplacement.NewTestGatewayPlacer(),
			}
			res, err := r.Reconcile(context.TODO(), testCase.args.req)
			testCase.verify(res, err, t)
		})
	}
}

func TestGatewayReconciler_reconcileDownstreamFromUpstreamGateway(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		gateway *v1beta1.Gateway
	}
	type testCase struct {
		name          string
		fields        fields
		args          args
		wantStatus    v1.ConditionStatus
		wantClusters  []string
		wantRequeue   bool
		wantErr       bool
		expectedError string
	}
	testCases := []testCase{
		{
			name: "gateway successfully reconciled",
			fields: fields{
				Client: testutil.GetValidTestClient(
					getValidCertificateSecret(certname(testutil.DummyCRName, testutil.ValidTestHostname)),
					buildTestMZ(),
					buildTestDNSRecord(),
				),
				Scheme: testutil.GetValidTestScheme(),
			},

			args: args{
				gateway: &v1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Labels:    getTestGatewayLabels(),
						Namespace: testutil.Namespace,
						Name:      testutil.DummyCRName,
					},
					Spec: buildValidTestGatewaySpec(),
				},
			},
			wantStatus:   v1.ConditionTrue,
			wantClusters: []string{testutil.Cluster},
			wantRequeue:  false,
			wantErr:      false,
		},
		{
			name: "created DNSRecord CR, HTTP protocol",
			fields: fields{
				Client: testutil.GetValidTestClient(
					getValidCertificateSecret(certname(testutil.DummyCRName, testutil.ValidTestHostname)),
					buildTestMZ(),
				),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				gateway: &v1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Labels:    getTestGatewayLabels(),
						Namespace: testutil.Namespace,
					},
					Spec: v1beta1.GatewaySpec{
						Listeners: []v1beta1.Listener{
							{
								Name:     v1beta1.SectionName(testutil.ValidTestHostname),
								Hostname: testutil.Pointer(v1beta1.Hostname(testutil.ValidTestHostname)),
								Protocol: v1beta1.HTTPProtocolType,
							},
						},
					},
				},
			},
			wantStatus:   v1.ConditionTrue,
			wantClusters: []string{testutil.Cluster},
			wantRequeue:  false,
			wantErr:      false,
		},
		{
			name: "failed get certificate secret error",
			fields: fields{
				Client: testutil.GetValidTestClient(buildTestMZ()),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				gateway: &v1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Labels:    getTestGatewayLabels(),
						Namespace: testutil.Namespace,
						Name:      "fail",
					},
					Spec: v1beta1.GatewaySpec{
						Listeners: []v1beta1.Listener{
							{
								Name:     "fail",
								Hostname: getTestHostname(testutil.FailEnsureCertHost),
								Protocol: v1beta1.HTTPSProtocolType,
							},
						},
					},
				},
			},
			wantStatus:    v1.ConditionFalse,
			wantClusters:  []string{},
			wantRequeue:   true,
			wantErr:       true,
			expectedError: ReconcileErrTLS.Error(),
		},
		{
			name: "tls secret doesn't exist yet so requeue",
			fields: fields{
				Client: testutil.GetValidTestClient(buildTestMZ()),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				gateway: &v1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Labels:    getTestGatewayLabels(),
						Namespace: testutil.Namespace,
						Name:      testutil.DummyCRName,
					},
					Spec: buildValidTestGatewaySpec(),
				},
			},
			wantStatus:    v1.ConditionFalse,
			wantClusters:  []string{},
			wantRequeue:   true,
			wantErr:       true,
			expectedError: ReconcileErrTLS.Error(),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := &GatewayReconciler{
				Client:       testCase.fields.Client,
				Scheme:       testCase.fields.Scheme,
				Certificates: faketls.NewTestCertificateService(testCase.fields.Client),
				Placement:    fakeplacement.NewTestGatewayPlacer(),
			}
			requeue, programmedStatus, clusters, err := r.reconcileDownstreamFromUpstreamGateway(context.TODO(), testCase.args.gateway, &Params{})
			if (err != nil) != testCase.wantErr || !testutil.GotExpectedError(testCase.expectedError, err) {
				t.Errorf("reconcileGateway() error = %v, wantErr %v, expectedError %v", err, testCase.wantErr, testCase.expectedError)
			}
			if programmedStatus != testCase.wantStatus {
				t.Errorf("reconcileGateway() programmedStatus = %v, want %v", programmedStatus, testCase.wantStatus)
			}
			if !reflect.DeepEqual(clusters, testCase.wantClusters) {
				t.Errorf("reconcileGateway() clusters = %v, want %v", clusters, testCase.wantClusters)
			}
			if requeue != testCase.wantRequeue {
				t.Errorf("reconcileGateway() requeue = %v, want %v", requeue, testCase.wantRequeue)
			}
		})
	}
}

func TestGatewayReconciler_reconcileTLS(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		upstreamGateway *v1beta1.Gateway
		gateway         *v1beta1.Gateway
		managedHosts    []v1alpha1.ManagedHost
	}
	type testCase struct {
		name    string
		fields  fields
		args    args
		want    []v1.Object
		wantErr bool
	}
	testCases := []testCase{
		{
			name: "secret synced downstream",
			fields: fields{
				Client: testutil.GetValidTestClient(getValidCertificateSecret(certname(testutil.DummyCRName, testutil.ValidTestHostname))),
				Scheme: testutil.GetValidTestScheme(),
			},

			args: args{
				upstreamGateway: &v1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Namespace: testutil.Namespace,
						Name:      testutil.DummyCRName,
					},
					Spec: v1beta1.GatewaySpec{
						Listeners: []v1beta1.Listener{
							{
								Name:     testutil.ValidTestHostname,
								Hostname: testutil.Pointer(v1beta1.Hostname(testutil.ValidTestHostname)),
								Protocol: v1beta1.HTTPSProtocolType,
							},
						},
					},
				},
				gateway: &v1beta1.Gateway{
					Spec: v1beta1.GatewaySpec{
						Listeners: []v1beta1.Listener{
							{
								Name:     testutil.ValidTestHostname,
								Hostname: testutil.Pointer(v1beta1.Hostname(testutil.ValidTestHostname)),
								Protocol: v1beta1.HTTPSProtocolType,
							},
						},
					},
				},
				managedHosts: []v1alpha1.ManagedHost{
					{
						Host: testutil.ValidTestHostname,
					},
				},
			},
			want:    []v1.Object{&getValidCertificateSecret(certname(testutil.DummyCRName, testutil.ValidTestHostname)).Items[0]},
			wantErr: false,
		},
		{
			name: "no cert for HTTP listener",
			fields: fields{
				Client: testutil.GetValidTestClient(),
				Scheme: testutil.GetValidTestScheme(),
			},
			args: args{
				upstreamGateway: &v1beta1.Gateway{},
				gateway: &v1beta1.Gateway{
					Spec: v1beta1.GatewaySpec{
						Listeners: []v1beta1.Listener{
							{
								Name:     testutil.ValidTestHostname,
								Hostname: testutil.Pointer(v1beta1.Hostname(testutil.ValidTestHostname)),
								Protocol: v1beta1.HTTPProtocolType,
							},
						},
					},
				},
				managedHosts: []v1alpha1.ManagedHost{
					{
						Host: testutil.ValidTestHostname,
					},
				},
			},
			want:    []v1.Object{},
			wantErr: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := &GatewayReconciler{
				Client:       testCase.fields.Client,
				Scheme:       testCase.fields.Scheme,
				Certificates: faketls.NewTestCertificateService(testCase.fields.Client),
				Placement:    fakeplacement.NewTestGatewayPlacer(),
			}
			got, err := r.reconcileTLS(context.TODO(), testCase.args.upstreamGateway, testCase.args.gateway)
			if (err != nil) != testCase.wantErr {
				t.Errorf("reconcileTLS() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !verifyTLSSecretTestResultsAsExpected(got, testCase.want, testCase.args.upstreamGateway) {
				t.Errorf("reconcileTLS() \ngot: \n%v \nwant: \n%v", got, testCase.want)
			}
		})
	}
}

func Test_buildProgrammedStatus(t *testing.T) {
	type args struct {
		gatewayStatus    v1beta1.GatewayStatus
		generation       int64
		clusters         []string
		programmedStatus v1.ConditionStatus
	}
	testCases := []struct {
		name string
		args args
		want []v1.Condition
	}{
		{
			name: "State has not changed",
			args: args{
				gatewayStatus: v1beta1.GatewayStatus{
					Conditions: []v1.Condition{
						testutil.BuildTestCondition(v1beta1.GatewayConditionAccepted, 1, ""),
						testutil.BuildTestCondition(v1beta1.GatewayConditionProgrammed, 1, ""),
					},
				},
				generation:       1,
				programmedStatus: v1.ConditionTrue,
			},
			want: []v1.Condition{
				testutil.BuildTestCondition(v1beta1.GatewayConditionAccepted, 1, ""),
				testutil.BuildTestCondition(v1beta1.GatewayConditionProgrammed, 1, "gateway placed on clusters"),
			},
		},
		{
			name: "Generation changed",
			args: args{
				gatewayStatus: v1beta1.GatewayStatus{
					Conditions: []v1.Condition{
						testutil.BuildTestCondition(v1beta1.GatewayConditionAccepted, 2, ""),
						testutil.BuildTestCondition(v1beta1.GatewayConditionProgrammed, 2, ""),
					},
				},
				generation:       1,
				programmedStatus: v1.ConditionTrue,
			},
			want: []v1.Condition{
				testutil.BuildTestCondition(v1beta1.GatewayConditionAccepted, 2, ""),
				testutil.BuildTestCondition(v1beta1.GatewayConditionProgrammed, 1, "gateway placed on clusters"),
			},
		},
		{
			name: "Placement failed",
			args: args{
				gatewayStatus: v1beta1.GatewayStatus{
					Conditions: []v1.Condition{
						testutil.BuildTestCondition(v1beta1.GatewayConditionProgrammed, 1, ""),
					},
				},
				generation:       1,
				programmedStatus: v1.ConditionFalse,
			},
			want: []v1.Condition{
				{
					Type:               string(v1beta1.GatewayConditionProgrammed),
					Status:             v1.ConditionFalse,
					ObservedGeneration: 1,
					Reason:             string(v1beta1.GatewayReasonProgrammed),
					Message:            "gateway failed to be placed",
				},
			},
		},
		{
			name: "Waiting for controller",
			args: args{
				gatewayStatus: v1beta1.GatewayStatus{
					Conditions: []v1.Condition{
						testutil.BuildTestCondition(v1beta1.GatewayConditionProgrammed, 1, ""),
					},
				},
				generation:       1,
				programmedStatus: v1.ConditionUnknown,
			},
			want: []v1.Condition{
				{
					Type:               string(v1beta1.GatewayConditionProgrammed),
					Status:             v1.ConditionUnknown,
					ObservedGeneration: 1,
					Reason:             string(v1beta1.ListenerReasonProgrammed),
					Message:            "current state of the gateway is unknown error",
				},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := buildProgrammedCondition(testCase.args.generation, testCase.args.clusters, testCase.args.programmedStatus, nil); !testutil.ConditionsEqual(got, testCase.want) {
				t.Errorf("buildStatusConditions() = \ngot:\n%v, \nwant: \n%v", got, testCase.want)
			}
		})
	}
}

// helper functions
func verifyTLSSecretTestResultsAsExpected(got []v1.Object, want []v1.Object, gateway *v1beta1.Gateway) bool {
	for _, wantSecret := range want {
		match := false
		for _, gotSecret := range got {
			if wantSecret.GetName() == gotSecret.GetName() &&
				reflect.DeepEqual(wantSecret.GetLabels(), gotSecret.GetLabels()) &&
				reflect.DeepEqual(wantSecret.GetAnnotations(), gotSecret.GetAnnotations()) &&
				wantSecret.GetNamespace() == gateway.GetNamespace() {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func getValidCertificateSecret(hostname string) *corev1.SecretList {
	return &corev1.SecretList{
		Items: []corev1.Secret{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      hostname,
					Namespace: testutil.Namespace,
					Labels: map[string]string{
						"argocd.argoproj.io/secret-type": "cluster",
					},
				},
				Data: map[string][]byte{
					testutil.Cluster: []byte("boop"),
				},
			},
		},
	}
}

func buildRequeueStatus() v1beta1.GatewayStatus {
	status := v1beta1.GatewayStatus{
		Conditions: []v1.Condition{},
	}
	_ = append(status.Conditions, buildAcceptedCondition(0, v1.ConditionTrue))
	_ = append(status.Conditions, buildProgrammedCondition(0, []string{}, v1.ConditionUnknown, nil))
	return status
}

func getTestGatewayLabels() map[string]string {
	return map[string]string{
		placement.OCMPlacementLabel: testutil.Placement,
	}
}

func buildValidTestGatewaySpec() v1beta1.GatewaySpec {
	return v1beta1.GatewaySpec{
		GatewayClassName: testutil.DummyCRName,
		Listeners: []v1beta1.Listener{
			{
				Name:     v1beta1.SectionName(testutil.ValidTestHostname),
				Hostname: testutil.Pointer(v1beta1.Hostname(testutil.ValidTestHostname)),
				Protocol: v1beta1.HTTPSProtocolType,
			},
		},
	}
}

func buildTestMZ() *v1alpha1.ManagedZoneList {
	return &v1alpha1.ManagedZoneList{
		Items: []v1alpha1.ManagedZone{
			{
				ObjectMeta: v1.ObjectMeta{
					Namespace: testutil.Namespace,
				},
				Spec: v1alpha1.ManagedZoneSpec{
					DomainName: testutil.Domain,
				},
			},
		},
	}
}

func buildTestDNSRecord() *v1alpha1.DNSRecordList {
	return &v1alpha1.DNSRecordList{
		Items: []v1alpha1.DNSRecord{
			{
				ObjectMeta: v1.ObjectMeta{
					Name:      testutil.Domain,
					Namespace: testutil.Namespace,
				},
			},
		},
	}
}
