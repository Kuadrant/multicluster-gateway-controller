package dnspolicy

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayapiv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"
	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

const (
	Domain            = "thecat.com"
	ValidTestHostname = "boop." + Domain
	ValidTestWildcard = "*." + Domain
)

func TestDNSPolicyReconciler_expectedProbesForGateway(t *testing.T) {

	type fields struct {
		TargetRefReconciler reconcilers.TargetRefReconciler
		DNSProvider         dns.DNSProviderFactory
		dnsHelper           dnsHelper
		Placer              gateway.GatewayPlacer
	}
	type args struct {
		ctx       context.Context
		gw        common.GatewayWrapper
		dnsPolicy *v1alpha1.DNSPolicy
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*v1alpha1.DNSHealthCheckProbe
		wantErr bool
	}{
		{
			name: "expected probes not nil and no error when all values specified in the check",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				DNSProvider:         nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: common.GatewayWrapper{
					Gateway: &gatewayapiv1beta1.Gateway{
						ObjectMeta: controllerruntime.ObjectMeta{
							Name:      "testgateway",
							Namespace: "testnamespace",
						},
						Spec: v1alpha2.GatewaySpec{
							Listeners: []v1alpha2.Listener{
								{
									Name:     "testlistener",
									Hostname: (*gatewayapiv1beta1.Hostname)(testutil.Pointer(ValidTestHostname)),
								},
							},
						},
						Status: v1alpha2.GatewayStatus{
							Addresses: []v1alpha2.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1beta1.IPAddressType),
									Value: "clusterName/172.31.200.0",
								},
							},
						},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: controllerruntime.ObjectMeta{
						Name:      "testdnspolicy",
						Namespace: "testnamespace",
					},
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{
							Endpoint: "/",
							Port:     testutil.Pointer(8443),
							Protocol: testutil.Pointer(v1alpha1.HttpsProtocol),
							AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
								Name: "probe-headers",
							},
							FailureThreshold: testutil.Pointer(1),
							ExpectedResponses: []int{
								200, 201,
							},
							AllowInsecureCertificates: true,
						},
					},
				},
			},
			want: []*v1alpha1.DNSHealthCheckProbe{
				{
					ObjectMeta: controllerruntime.ObjectMeta{
						Name:      "172.31.200.0-testdnspolicy-testlistener",
						Namespace: "testnamespace",
						Labels: map[string]string{
							DNSPolicyBackRefAnnotation:                              "testdnspolicy",
							fmt.Sprintf("%s-namespace", DNSPolicyBackRefAnnotation): "testnamespace",
							LabelGatewayNSRef:                                       "testnamespace",
							LabelGatewayReference:                                   "testgateway",
						},
						Annotations: map[string]string{
							"dnsrecord-name":      "testgateway-testlistener",
							"dnsrecord-namespace": "testnamespace",
						},
					},
					Spec: v1alpha1.DNSHealthCheckProbeSpec{
						Port:     8443,
						Host:     ValidTestHostname,
						Address:  "172.31.200.0",
						Path:     "/",
						Protocol: v1alpha1.HttpsProtocol,
						Interval: metav1.Duration{Duration: 60 * time.Second},
						AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
							Name: "probe-headers",
						},
						AllowInsecureCertificate: true,
						ExpectedResponses:        []int{200, 201},
						FailureThreshold:         testutil.Pointer(1),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "expected probes not nil and no error when some values not specified in the check",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				DNSProvider:         nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: common.GatewayWrapper{
					Gateway: &gatewayapiv1beta1.Gateway{
						ObjectMeta: controllerruntime.ObjectMeta{
							Name:      "testgateway",
							Namespace: "testnamespace",
						},
						Spec: v1alpha2.GatewaySpec{
							Listeners: []v1alpha2.Listener{
								{
									Name:     "testlistener",
									Hostname: (*gatewayapiv1beta1.Hostname)(testutil.Pointer(ValidTestHostname)),
								},
							},
						},
						Status: v1alpha2.GatewayStatus{
							Addresses: []v1alpha2.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1beta1.IPAddressType),
									Value: "clusterName/172.31.200.0",
								},
							},
						},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: controllerruntime.ObjectMeta{
						Name:      "testdnspolicy",
						Namespace: "testnamespace",
					},
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{},
					},
				},
			},
			want: []*v1alpha1.DNSHealthCheckProbe{
				{
					ObjectMeta: controllerruntime.ObjectMeta{
						Name:      "172.31.200.0-testdnspolicy-testlistener",
						Namespace: "testnamespace",
						Labels: map[string]string{
							DNSPolicyBackRefAnnotation:                              "testdnspolicy",
							fmt.Sprintf("%s-namespace", DNSPolicyBackRefAnnotation): "testnamespace",
							LabelGatewayNSRef:                                       "testnamespace",
							LabelGatewayReference:                                   "testgateway",
						},
						Annotations: map[string]string{
							"dnsrecord-name":      "testgateway-testlistener",
							"dnsrecord-namespace": "testnamespace",
						},
					},
					Spec: v1alpha1.DNSHealthCheckProbeSpec{
						Port:     443,
						Host:     ValidTestHostname,
						Address:  "172.31.200.0",
						Protocol: v1alpha1.HttpsProtocol,
						Interval: metav1.Duration{Duration: 60 * time.Second},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no probes when listener has a wildcard domain",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				DNSProvider:         nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: common.GatewayWrapper{
					Gateway: &gatewayapiv1beta1.Gateway{
						Spec: v1alpha2.GatewaySpec{
							Listeners: []v1alpha2.Listener{
								{
									Hostname: (*gatewayapiv1beta1.Hostname)(testutil.Pointer(ValidTestWildcard)),
								},
							},
						},
						Status: v1alpha2.GatewayStatus{
							Addresses: []v1alpha2.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1beta1.IPAddressType),
									Value: "clusterName/172.31.200.0",
								},
							},
						},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "no probes and error produced when address.Value doesn't contain /",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				DNSProvider:         nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: common.GatewayWrapper{
					Gateway: &gatewayapiv1beta1.Gateway{
						Status: v1alpha2.GatewayStatus{
							Addresses: []v1alpha2.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1beta1.IPAddressType),
									Value: "clusterName:172.31.200.0",
								},
							},
						},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "no probes and no error when no listeners defined",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				DNSProvider:         nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: common.GatewayWrapper{
					Gateway: &gatewayapiv1beta1.Gateway{
						Status: v1alpha2.GatewayStatus{
							Addresses: []v1alpha2.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1beta1.IPAddressType),
									Value: "clusterName/172.31.200.0",
								},
							},
						},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "no probes and no error when no address defined",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				DNSProvider:         nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: common.GatewayWrapper{
					Gateway: &gatewayapiv1beta1.Gateway{
						Status: v1alpha2.GatewayStatus{},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name:   "no probes and no error when no healthcheck spec defined",
			fields: fields{},
			args: args{
				dnsPolicy: &v1alpha1.DNSPolicy{},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &DNSPolicyReconciler{
				TargetRefReconciler: tt.fields.TargetRefReconciler,
				DNSProvider:         tt.fields.DNSProvider,
				dnsHelper:           tt.fields.dnsHelper,
				Placer:              tt.fields.Placer,
			}
			got, err := r.expectedProbesForGateway(tt.args.ctx, tt.args.gw, tt.args.dnsPolicy)
			if (err != nil) != tt.wantErr {
				t.Errorf("expectedProbesForGateway() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expectedProbesForGateway() got = %v, want %v", got, tt.want)
			}
		})
	}
}
