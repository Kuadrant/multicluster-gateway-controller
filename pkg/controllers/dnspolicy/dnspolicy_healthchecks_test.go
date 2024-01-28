package dnspolicy

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	kuadrantcommon "github.com/kuadrant/kuadrant-operator/pkg/common"
	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/common"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/provider"
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
		ProviderFactory     provider.Factory
		dnsHelper           dnsHelper
		Placer              gateway.GatewayPlacer
	}
	type args struct {
		ctx       context.Context
		gw        kuadrantcommon.GatewayWrapper
		dnsPolicy *v1alpha1.DNSPolicy
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*v1alpha1.DNSHealthCheckProbe
	}{
		{
			name: "expected probes not nil when all values specified in the check",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						ObjectMeta: controllerruntime.ObjectMeta{
							Name:      "testgateway",
							Namespace: "testnamespace",
						},
						Spec: gatewayapiv1.GatewaySpec{
							Listeners: []gatewayapiv1.Listener{
								{
									Name:     "testlistener",
									Hostname: (*gatewayapiv1.Hostname)(testutil.Pointer(ValidTestHostname)),
								},
							},
						},
						Status: gatewayapiv1.GatewayStatus{
							Addresses: []gatewayapiv1.GatewayStatusAddress{
								{
									Type:  testutil.Pointer(common.MultiClusterIPAddressType),
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
						Name:      "172.31.200.0-testgateway-testlistener",
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
		},
		{
			name: "expected probes not nil when some values not specified in the check",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						ObjectMeta: controllerruntime.ObjectMeta{
							Name:      "testgateway",
							Namespace: "testnamespace",
						},
						Spec: gatewayapiv1.GatewaySpec{
							Listeners: []gatewayapiv1.Listener{
								{
									Name:     "testlistener",
									Hostname: (*gatewayapiv1.Hostname)(testutil.Pointer(ValidTestHostname)),
									Port:     443,
									Protocol: gatewayapiv1.ProtocolType(v1alpha1.HttpsProtocol),
								},
							},
						},
						Status: gatewayapiv1.GatewayStatus{
							Addresses: []gatewayapiv1.GatewayStatusAddress{
								{
									Type:  testutil.Pointer(common.MultiClusterIPAddressType),
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
						Name:      "172.31.200.0-testgateway-testlistener",
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
		},
		{
			name: "no probes when listener has a wildcard domain",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						Spec: gatewayapiv1.GatewaySpec{
							Listeners: []gatewayapiv1.Listener{
								{
									Hostname: (*gatewayapiv1.Hostname)(testutil.Pointer(ValidTestWildcard)),
								},
							},
						},
						Status: gatewayapiv1.GatewayStatus{
							Addresses: []gatewayapiv1.GatewayStatusAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
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
			want: nil,
		},
		{
			name: "expected probes when status.address.value is an IP address",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						ObjectMeta: controllerruntime.ObjectMeta{
							Name:      "testgateway",
							Namespace: "testnamespace",
						},
						Spec: gatewayapiv1.GatewaySpec{
							Listeners: []gatewayapiv1.Listener{
								{
									Name:     "testlistener",
									Hostname: (*gatewayapiv1.Hostname)(testutil.Pointer(ValidTestHostname)),
									Port:     443,
									Protocol: gatewayapiv1.ProtocolType(v1alpha1.HttpsProtocol),
								},
							},
						},
						Status: gatewayapiv1.GatewayStatus{
							Addresses: []gatewayapiv1.GatewayStatusAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "172.31.200.0",
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
						Name:      "172.31.200.0-testgateway-testlistener",
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
		},
		{
			name: "no probes when address.Value doesn't contain /",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						Status: gatewayapiv1.GatewayStatus{
							Addresses: []gatewayapiv1.GatewayStatusAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
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
			want: nil,
		},
		{
			name: "no probes when no listeners defined",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						Status: gatewayapiv1.GatewayStatus{
							Addresses: []gatewayapiv1.GatewayStatusAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
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
			want: nil,
		},
		{
			name: "no probes when no address defined",
			fields: fields{
				TargetRefReconciler: reconcilers.TargetRefReconciler{},
				ProviderFactory:     nil,
				dnsHelper:           dnsHelper{},
				Placer:              nil,
			},
			args: args{
				ctx: nil,
				gw: kuadrantcommon.GatewayWrapper{
					Gateway: &gatewayapiv1.Gateway{
						Status: gatewayapiv1.GatewayStatus{},
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					Spec: v1alpha1.DNSPolicySpec{
						HealthCheck: &v1alpha1.HealthCheckSpec{},
					},
				},
			},
			want: nil,
		},
		{
			name:   "no probes when no healthcheck spec defined",
			fields: fields{},
			args: args{
				dnsPolicy: &v1alpha1.DNSPolicy{},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &DNSPolicyReconciler{
				TargetRefReconciler: tt.fields.TargetRefReconciler,
				ProviderFactory:     tt.fields.ProviderFactory,
				dnsHelper:           tt.fields.dnsHelper,
			}
			got := r.expectedHealthCheckProbesForGateway(tt.args.ctx, tt.args.gw, tt.args.dnsPolicy)
			if !reflect.DeepEqual(got, tt.want) {
				for _, g := range got {
					t.Logf("got: %+v", g)
				}
				for _, w := range tt.want {
					t.Logf("want: %+v", w)
				}
				t.Errorf("expectedHealthCheckProbesForGateway()")
			}
		})
	}
}
