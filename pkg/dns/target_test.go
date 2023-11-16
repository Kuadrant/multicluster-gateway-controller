//go:build unit

package dns

import (
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

const (
	testAddress1 = "127.0.0.1"
	testAddress2 = "127.0.0.2"
	clusterName1 = "tst-cluster"
	clusterName2 = "tst-cluster2"
)

func TestNewClusterGatewayTarget(t *testing.T) {

	type args struct {
		clusterGateway ClusterGateway
		defaultGeoCode GeoCode
		defaultWeight  int
		customWeights  []*v1alpha1.CustomWeight
	}
	testCases := []struct {
		name    string
		args    args
		want    ClusterGatewayTarget
		wantErr bool
	}{
		{
			name: "set geo and weight from defaults",
			args: args{
				clusterGateway: ClusterGateway{
					Cluster: &testutil.TestResource{
						ObjectMeta: v1.ObjectMeta{
							Name: clusterName1,
						},
					},
					GatewayAddresses: buildGatewayAddress(testAddress1),
				},
				defaultWeight:  100,
				defaultGeoCode: GeoCode("IE"),
				customWeights:  []*v1alpha1.CustomWeight{},
			},
			want: ClusterGatewayTarget{
				ClusterGateway: &ClusterGateway{
					Cluster: &testutil.TestResource{
						ObjectMeta: v1.ObjectMeta{
							Name: clusterName1,
						},
					},
					GatewayAddresses: buildGatewayAddress(testAddress1),
				},
				Geo:    testutil.Pointer(GeoCode("IE")),
				Weight: testutil.Pointer(100),
			},
			wantErr: false,
		},
		{
			name: "set geo and weight from cluster labels",
			args: args{
				clusterGateway: ClusterGateway{
					Cluster: &testutil.TestResource{
						ObjectMeta: v1.ObjectMeta{
							Name: clusterName1,
							Labels: map[string]string{
								"kuadrant.io/lb-attribute-weight":   "TSTATTR",
								"kuadrant.io/lb-attribute-geo-code": "EU",
							},
						},
					},
					GatewayAddresses: buildGatewayAddress(testAddress1),
				},
				defaultWeight:  100,
				defaultGeoCode: GeoCode("IE"),
				customWeights: []*v1alpha1.CustomWeight{
					{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kuadrant.io/lb-attribute-weight": "TSTATTR",
							},
						},
						Weight: 60,
					},
				},
			},
			want: ClusterGatewayTarget{
				ClusterGateway: &ClusterGateway{
					Cluster: &testutil.TestResource{
						ObjectMeta: v1.ObjectMeta{
							Name: clusterName1,
							Labels: map[string]string{
								"kuadrant.io/lb-attribute-weight":   "TSTATTR",
								"kuadrant.io/lb-attribute-geo-code": "EU",
							},
						},
					},
					GatewayAddresses: buildGatewayAddress(testAddress1),
				},
				Geo:    testutil.Pointer(GeoCode("EU")),
				Weight: testutil.Pointer(60),
			},
			wantErr: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Run(testCase.name, func(t *testing.T) {
				got, err := NewClusterGatewayTarget(testCase.args.clusterGateway, testCase.args.defaultGeoCode, testCase.args.defaultWeight, testCase.args.customWeights)
				if (err != nil) != testCase.wantErr {
					t.Errorf("NewClusterGatewayTarget() error = %v, wantErr %v", err, testCase.wantErr)
					return
				}
				if !reflect.DeepEqual(got, testCase.want) {
					t.Errorf("NewClusterGatewayTarget() = %v, want %v", got, testCase.want)
				}
			})

		})
	}
}

func TestNewMultiClusterGatewayTarget(t *testing.T) {
	type args struct {
		gateway         *gatewayapiv1.Gateway
		clusterGateways []ClusterGateway
		loadBalancing   *v1alpha1.LoadBalancingSpec
	}
	gateway := &gatewayapiv1.Gateway{
		ObjectMeta: v1.ObjectMeta{
			Name:      "testgw",
			Namespace: "testns",
		},
	}
	testCases := []struct {
		name    string
		args    args
		want    *MultiClusterGatewayTarget
		wantErr bool
	}{
		{
			name: "set cluster gateway targets with default geo and weight values",
			args: args{
				gateway: gateway,
				clusterGateways: []ClusterGateway{
					{
						Cluster: &testutil.TestResource{
							ObjectMeta: v1.ObjectMeta{
								Name: clusterName1,
							},
						},
						GatewayAddresses: buildGatewayAddress(testAddress1),
					},
					{
						Cluster: &testutil.TestResource{
							ObjectMeta: v1.ObjectMeta{
								Name: clusterName2,
							},
						},
						GatewayAddresses: buildGatewayAddress(testAddress2),
					},
				},
				loadBalancing: nil,
			},
			want: &MultiClusterGatewayTarget{
				Gateway: gateway,
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: clusterName1,
								},
							},
							GatewayAddresses: buildGatewayAddress(testAddress1),
						},
						Geo:    testutil.Pointer(DefaultGeo),
						Weight: testutil.Pointer(DefaultWeight),
					},
					{
						ClusterGateway: &ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: clusterName2,
								},
							},
							GatewayAddresses: buildGatewayAddress(testAddress2),
						},
						Geo:    testutil.Pointer(DefaultGeo),
						Weight: testutil.Pointer(DefaultWeight),
					},
				},
				LoadBalancing: nil,
			},
			wantErr: false,
		},
		{
			name: "set cluster gateway targets with default geo and weight from load balancing config",
			args: args{
				gateway: gateway,
				clusterGateways: []ClusterGateway{
					{
						Cluster: &testutil.TestResource{
							ObjectMeta: v1.ObjectMeta{
								Name: clusterName1,
							},
						},
						GatewayAddresses: buildGatewayAddress(testAddress1),
					},
					{
						Cluster: &testutil.TestResource{
							ObjectMeta: v1.ObjectMeta{
								Name: clusterName2,
							},
						},
						GatewayAddresses: buildGatewayAddress(testAddress2),
					},
				},
				loadBalancing: &v1alpha1.LoadBalancingSpec{
					Weighted: &v1alpha1.LoadBalancingWeighted{
						DefaultWeight: 255,
					},
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "IE",
					},
				},
			},
			want: &MultiClusterGatewayTarget{
				Gateway: gateway,
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: clusterName1,
								},
							},
							GatewayAddresses: buildGatewayAddress(testAddress1),
						},
						Geo:    testutil.Pointer(GeoCode("IE")),
						Weight: testutil.Pointer(255),
					},
					{
						ClusterGateway: &ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: clusterName2,
								},
							},
							GatewayAddresses: buildGatewayAddress(testAddress2),
						},
						Geo:    testutil.Pointer(GeoCode("IE")),
						Weight: testutil.Pointer(255),
					},
				},
				LoadBalancing: &v1alpha1.LoadBalancingSpec{
					Weighted: &v1alpha1.LoadBalancingWeighted{
						DefaultWeight: 255,
					},
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "IE",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "set cluster gateway targets with default geo and weight from cluster labels",
			args: args{
				gateway: gateway,
				clusterGateways: []ClusterGateway{
					{
						Cluster: &testutil.TestResource{
							ObjectMeta: v1.ObjectMeta{
								Name: clusterName1,
								Labels: map[string]string{
									"kuadrant.io/lb-attribute-weight":   "TSTATTR",
									"kuadrant.io/lb-attribute-geo-code": "EU",
								},
							},
						},
						GatewayAddresses: buildGatewayAddress(testAddress1),
					},
					{
						Cluster: &testutil.TestResource{
							ObjectMeta: v1.ObjectMeta{
								Name: clusterName2,
							},
						},
						GatewayAddresses: buildGatewayAddress(testAddress2),
					},
				},
				loadBalancing: &v1alpha1.LoadBalancingSpec{
					Weighted: &v1alpha1.LoadBalancingWeighted{
						DefaultWeight: 255,
						Custom: []*v1alpha1.CustomWeight{
							{
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kuadrant.io/lb-attribute-weight": "TSTATTR",
									},
								},
								Weight: 60,
							},
						},
					},
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "IE",
					},
				},
			},
			want: &MultiClusterGatewayTarget{
				Gateway: gateway,
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: clusterName1,
									Labels: map[string]string{
										"kuadrant.io/lb-attribute-weight":   "TSTATTR",
										"kuadrant.io/lb-attribute-geo-code": "EU",
									},
								},
							},
							GatewayAddresses: buildGatewayAddress(testAddress1),
						},
						Geo:    testutil.Pointer(GeoCode("EU")),
						Weight: testutil.Pointer(60),
					},
					{
						ClusterGateway: &ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: clusterName2,
								},
							},
							GatewayAddresses: buildGatewayAddress(testAddress2),
						},
						Geo:    testutil.Pointer(GeoCode("IE")),
						Weight: testutil.Pointer(255),
					},
				},
				LoadBalancing: &v1alpha1.LoadBalancingSpec{
					Weighted: &v1alpha1.LoadBalancingWeighted{
						DefaultWeight: 255,
						Custom: []*v1alpha1.CustomWeight{
							{
								Selector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kuadrant.io/lb-attribute-weight": "TSTATTR",
									},
								},
								Weight: 60,
							},
						},
					},
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "IE",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NewMultiClusterGatewayTarget(testCase.args.gateway, testCase.args.clusterGateways, testCase.args.loadBalancing)
			if (err != nil) != testCase.wantErr {
				t.Errorf("NewMultiClusterGatewayTarget() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("NewMultiClusterGatewayTarget() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestToBase36hash(t *testing.T) {
	testCases := []struct {
		in   string
		want string
	}{
		{"c1", "2piivc"},
		{"c2", "2pcjv8"},
		{"g1", "egzg90"},
		{"g2", "28bp8h"},
		{"cluster1", "20st0r"},
		{"cluster2", "1c80l6"},
		{"gateway1", "2hyvk7"},
		{"gateway2", "5c23wh"},
		{"prod-web-multi-cluster-gateways", "4ej5le"},
		{"kind-mgc-control-plane", "2c71gf"},
		{"test-cluster-1", "20qri0"},
		{"test-cluster-2", "2pj3we"},
		{"testgw-testns", "0ecjaw"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.in, func(t *testing.T) {
			if got := ToBase36hash(testCase.in); got != testCase.want {
				t.Errorf("ToBase36hash() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestClusterGatewayTarget_setGeo(t *testing.T) {
	testCases := []struct {
		name          string
		defaultGeo    GeoCode
		clusterLabels map[string]string
		want          GeoCode
	}{
		{
			name:          "sets geo from default",
			defaultGeo:    "IE",
			clusterLabels: nil,
			want:          "IE",
		},
		{
			name:       "sets geo from label",
			defaultGeo: "IE",
			clusterLabels: map[string]string{
				"kuadrant.io/lb-attribute-geo-code": "EU",
			},
			want: "EU",
		},
		{
			name:       "sets geo to default for default geo value",
			defaultGeo: "default",
			clusterLabels: map[string]string{
				"kuadrant.io/lb-attribute-geo-code": "EU",
			},
			want: "default",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t1 *testing.T) {
			cgt := &ClusterGatewayTarget{
				ClusterGateway: &ClusterGateway{
					Cluster: &testutil.TestResource{
						ObjectMeta: v1.ObjectMeta{
							Name:   clusterName1,
							Labels: testCase.clusterLabels,
						},
					},
					GatewayAddresses: buildGatewayAddress(testAddress1),
				},
			}
			cgt.setGeo(testCase.defaultGeo)
			if got := *cgt.Geo; got != testCase.want {
				t.Errorf("setGeo() got = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestClusterGatewayTarget_setWeight(t *testing.T) {
	testCases := []struct {
		name          string
		defaultWeight int
		customWeights []*v1alpha1.CustomWeight
		clusterLabels map[string]string
		want          int
		wantErr       bool
	}{
		{
			name:          "sets geo from default",
			defaultWeight: 255,
			clusterLabels: nil,
			customWeights: []*v1alpha1.CustomWeight{},
			want:          255,
			wantErr:       false,
		},
		{
			name:          "sets geo from custom weight",
			defaultWeight: 255,
			clusterLabels: map[string]string{
				"tstlabel1": "TSTATTR",
			},
			customWeights: []*v1alpha1.CustomWeight{
				{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"tstlabel1": "TSTATTR",
						},
					},
					Weight: 100,
				},
			},
			want:    100,
			wantErr: false,
		},
		{
			name:          "sets geo from from custom weight with selector with multiple matches",
			defaultWeight: 255,
			clusterLabels: map[string]string{
				"tstlabel1": "TSTATTR",
				"tstlabel2": "TSTATTR2",
			},
			customWeights: []*v1alpha1.CustomWeight{
				{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"tstlabel1": "TSTATTR",
							"tstlabel2": "TSTATTR2",
						},
					},
					Weight: 100,
				},
			},
			want:    100,
			wantErr: false,
		},
		{
			name:          "sets geo from default when not all custom weight selectors match",
			defaultWeight: 255,
			clusterLabels: map[string]string{
				"tstlabel1": "TSTATTR",
			},
			customWeights: []*v1alpha1.CustomWeight{
				{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"tstlabel1": "TSTATTR",
							"tstlabel2": "TSTATTR2",
						},
					},
					Weight: 100,
				},
			},
			want:    255,
			wantErr: false,
		},
		{
			name:          "returns error when label selector invalid",
			defaultWeight: 255,
			clusterLabels: map[string]string{
				"/tstlabel1": "TSTATTR",
			},
			customWeights: []*v1alpha1.CustomWeight{
				{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"/tstlabel1": "TSTATTR",
						},
					},
					Weight: 100,
				},
			},
			want:    255,
			wantErr: true,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t1 *testing.T) {
			cgt := &ClusterGatewayTarget{
				ClusterGateway: &ClusterGateway{
					Cluster: &testutil.TestResource{
						ObjectMeta: v1.ObjectMeta{
							Name:   clusterName1,
							Labels: testCase.clusterLabels,
						},
					},
					GatewayAddresses: buildGatewayAddress(testAddress1),
				},
				Weight: &testCase.defaultWeight,
			}
			err := cgt.setWeight(testCase.defaultWeight, testCase.customWeights)
			if (err != nil) != testCase.wantErr {
				t.Errorf("setWeight() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			got := *cgt.Weight
			if got != testCase.want {
				t.Errorf("setWeight() got = %v, want %v", got, testCase.want)
			}
		})
	}
}

func buildGatewayAddress(value string) []gatewayapiv1.GatewayAddress {
	return []gatewayapiv1.GatewayAddress{
		{
			Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
			Value: value,
		},
	}
}

func TestMultiClusterGatewayTarget_RemoveUnhealthyGatewayAddresses(t *testing.T) {
	type fields struct {
		Gateway               *gatewayapiv1.Gateway
		ClusterGatewayTargets []ClusterGatewayTarget
		LoadBalancing         *v1alpha1.LoadBalancingSpec
	}
	type args struct {
		probes   []*v1alpha1.DNSHealthCheckProbe
		listener gatewayapiv1.Listener
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []ClusterGatewayTarget
	}{
		{
			name: "healthy probes or no probes results in all gateways being kept",
			fields: fields{
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							GatewayAddresses: []gatewayapiv1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "1.1.1.1",
								},
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "2.2.2.2",
								},
							},
						},
					},
				},
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
			},
			args: args{
				probes: []*v1alpha1.DNSHealthCheckProbe{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsHealthCheckProbeName("1.1.1.1", "testgw", "test"),
							Namespace: "namespace",
						},
						Status: v1alpha1.DNSHealthCheckProbeStatus{
							ConsecutiveFailures: 0,
							Healthy:             testutil.Pointer(true),
						},
						Spec: v1alpha1.DNSHealthCheckProbeSpec{
							FailureThreshold: testutil.Pointer(5),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsHealthCheckProbeName("2.2.2.2", "testgw", "test"),
							Namespace: "namespace",
						},
						Status: v1alpha1.DNSHealthCheckProbeStatus{
							ConsecutiveFailures: 0,
							Healthy:             testutil.Pointer(true),
						},
						Spec: v1alpha1.DNSHealthCheckProbeSpec{
							FailureThreshold: testutil.Pointer(5),
						},
					},
				},
				listener: gatewayapiv1.Listener{Name: "test"},
			},
			want: []ClusterGatewayTarget{
				{
					ClusterGateway: &ClusterGateway{
						GatewayAddresses: []gatewayapiv1.GatewayAddress{
							{
								Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
								Value: "1.1.1.1",
							},
							{
								Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
								Value: "2.2.2.2",
							},
						},
					},
				},
			},
		},
		{
			name: "some unhealthy probes results in the removal of a gateway",
			fields: fields{
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							GatewayAddresses: []gatewayapiv1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "1.1.1.1",
								},
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "2.2.2.2",
								},
							},
						},
					},
				},
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
			},
			args: args{
				probes: []*v1alpha1.DNSHealthCheckProbe{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsHealthCheckProbeName("1.1.1.1", "testgw", "test"),
							Namespace: "namespace",
						},
						Status: v1alpha1.DNSHealthCheckProbeStatus{
							ConsecutiveFailures: 6,
							Healthy:             testutil.Pointer(false),
						},
						Spec: v1alpha1.DNSHealthCheckProbeSpec{
							FailureThreshold: testutil.Pointer(5),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsHealthCheckProbeName("2.2.2.2", "testgw", "test"),
							Namespace: "namespace",
						},
						Status: v1alpha1.DNSHealthCheckProbeStatus{
							ConsecutiveFailures: 0,
							Healthy:             testutil.Pointer(true),
						},
						Spec: v1alpha1.DNSHealthCheckProbeSpec{
							FailureThreshold: testutil.Pointer(5),
						},
					},
				},
				listener: gatewayapiv1.Listener{Name: "test"},
			},
			want: []ClusterGatewayTarget{
				{
					ClusterGateway: &ClusterGateway{
						GatewayAddresses: []gatewayapiv1.GatewayAddress{
							{
								Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
								Value: "2.2.2.2",
							},
						},
					},
				},
			},
		},
		{
			name: "all unhealthy probes results in all gateways being kept",
			fields: fields{
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							GatewayAddresses: []gatewayapiv1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "1.1.1.1",
								},
								{
									Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
									Value: "2.2.2.2",
								},
							},
						},
					},
				},
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
			},
			args: args{
				probes: []*v1alpha1.DNSHealthCheckProbe{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsHealthCheckProbeName("1.1.1.1", "testgw", "test"),
							Namespace: "namespace",
						},
						Status: v1alpha1.DNSHealthCheckProbeStatus{
							ConsecutiveFailures: 6,
							Healthy:             testutil.Pointer(false),
						},
						Spec: v1alpha1.DNSHealthCheckProbeSpec{
							FailureThreshold: testutil.Pointer(5),
						},
					},
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsHealthCheckProbeName("2.2.2.2", "testgw", "test"),
							Namespace: "namespace",
						},
						Status: v1alpha1.DNSHealthCheckProbeStatus{
							ConsecutiveFailures: 6,
							Healthy:             testutil.Pointer(false),
						},
						Spec: v1alpha1.DNSHealthCheckProbeSpec{
							FailureThreshold: testutil.Pointer(5),
						},
					},
				},
				listener: gatewayapiv1.Listener{Name: "test"},
			},
			want: []ClusterGatewayTarget{
				{
					ClusterGateway: &ClusterGateway{
						GatewayAddresses: []gatewayapiv1.GatewayAddress{
							{
								Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
								Value: "1.1.1.1",
							},
							{
								Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
								Value: "2.2.2.2",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t1 *testing.T) {
			mgt := &MultiClusterGatewayTarget{
				Gateway:               tt.fields.Gateway,
				ClusterGatewayTargets: tt.fields.ClusterGatewayTargets,
				LoadBalancing:         tt.fields.LoadBalancing,
			}
			mgt.RemoveUnhealthyGatewayAddresses(tt.args.probes, tt.args.listener)
			if !reflect.DeepEqual(mgt.ClusterGatewayTargets, tt.want) {
				for _, target := range mgt.ClusterGatewayTargets {
					fmt.Println(target)
				}
				t.Errorf("got = %v, want %v", mgt.ClusterGatewayTargets, tt.want)
			}
		})
	}
}
