//go:build unit

package dns

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestGeoCode_IsContinentCode(t *testing.T) {
	tests := []struct {
		gc   GeoCode
		want bool
	}{
		{
			gc:   "",
			want: false,
		},
		{
			gc:   "AF",
			want: false,
		},
		{
			gc:   "af",
			want: false,
		},
		{
			gc:   "C-AF",
			want: true,
		},
		{
			gc:   "C-AN",
			want: true,
		}, {
			gc:   "C-AS",
			want: true,
		}, {
			gc:   "C-OC",
			want: true,
		}, {
			gc:   "C-NA",
			want: true,
		}, {
			gc:   "C-SA",
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(string(tt.gc), func(t *testing.T) {
			if got := tt.gc.IsContinentCode(); got != tt.want {
				t.Errorf("IsContinentCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

type TestCluster struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

const (
	testAddress1 = "127.0.0.1"
	testAddress2 = "127.0.0.2"
	clusterName1 = "tst-cluster"
	clusterName2 = "tst-cluster2"
)

func TestNewClusterGateway(t *testing.T) {

	type args struct {
		cluster          v1.Object
		gatewayAddresses []gatewayv1beta1.GatewayAddress
	}
	tests := []struct {
		name string
		args args
		want *ClusterGateway
	}{
		{
			name: "no attributes",
			args: args{
				cluster: &TestCluster{
					ObjectMeta: v1.ObjectMeta{
						Name: clusterName1,
					},
				},
				gatewayAddresses: buildGatewayAddress(testAddress1),
			},
			want: &ClusterGateway{
				ClusterName:       clusterName1,
				GatewayAddresses:  buildGatewayAddress(testAddress1),
				ClusterAttributes: ClusterAttributes{},
			},
		},
		{
			name: "sets valid geo code from geo code attribute label",
			args: args{
				cluster: &TestCluster{
					ObjectMeta: v1.ObjectMeta{
						Name: clusterName1,
						Labels: map[string]string{
							"kuadrant.io/lb-attribute-geo-code": "IE",
						},
					},
				},
				gatewayAddresses: buildGatewayAddress(testAddress1),
			},
			want: &ClusterGateway{
				ClusterName:      clusterName1,
				GatewayAddresses: buildGatewayAddress(testAddress1),
				ClusterAttributes: ClusterAttributes{
					Geo: testutil.Pointer(GeoCode("IE")),
				},
			},
		},
		{
			name: "ignores invalid geo code in geo code attribute label",
			args: args{
				cluster: &TestCluster{
					ObjectMeta: v1.ObjectMeta{
						Name: clusterName1,
						Labels: map[string]string{
							"kuadrant.io/lb-attribute-geo-code": "NOTACODE",
						},
					},
				},
				gatewayAddresses: buildGatewayAddress(testAddress1),
			},
			want: &ClusterGateway{
				ClusterName:       clusterName1,
				GatewayAddresses:  buildGatewayAddress(testAddress1),
				ClusterAttributes: ClusterAttributes{},
			},
		},
		{
			name: "sets custom weight from custom weight attribute label",
			args: args{
				cluster: &TestCluster{
					ObjectMeta: v1.ObjectMeta{
						Name: clusterName1,
						Labels: map[string]string{
							"kuadrant.io/lb-attribute-custom-weight": "MYATTR",
						},
					},
				},
				gatewayAddresses: buildGatewayAddress(testAddress1),
			},
			want: &ClusterGateway{
				ClusterName:      clusterName1,
				GatewayAddresses: buildGatewayAddress(testAddress1),
				ClusterAttributes: ClusterAttributes{
					CustomWeight: testutil.Pointer("MYATTR"),
				},
			},
		},
		{
			name: "sets both custom weight and geo from attribute labels",
			args: args{
				cluster: &TestCluster{
					ObjectMeta: v1.ObjectMeta{
						Name: clusterName1,
						Labels: map[string]string{
							"label1":                                 "label1",
							"kuadrant.io/lb-attribute-geo-code":      "IE",
							"label2":                                 "label2",
							"kuadrant.io/lb-attribute-custom-weight": "MYATTR",
							"label3":                                 "label3",
						},
					},
				},
				gatewayAddresses: buildGatewayAddress(testAddress1),
			},
			want: &ClusterGateway{
				ClusterName:      clusterName1,
				GatewayAddresses: buildGatewayAddress(testAddress1),
				ClusterAttributes: ClusterAttributes{
					CustomWeight: testutil.Pointer("MYATTR"),
					Geo:          testutil.Pointer(GeoCode("IE")),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewClusterGateway(tt.args.cluster, tt.args.gatewayAddresses); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClusterGateway() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMultiClusterGatewayTarget(t *testing.T) {
	type args struct {
		gateway         *gatewayv1beta1.Gateway
		clusterGateways []ClusterGateway
		loadBalancing   *v1alpha1.LoadBalancingSpec
	}
	gateway := &gatewayv1beta1.Gateway{
		ObjectMeta: v1.ObjectMeta{
			Name:      "testgw",
			Namespace: "testns",
		},
	}
	tests := []struct {
		name string
		args args
		want *MultiClusterGatewayTarget
	}{
		{
			name: "set cluster gateway targets with default geo and weight values",
			args: args{
				gateway: gateway,
				clusterGateways: []ClusterGateway{
					{
						ClusterName:       clusterName1,
						GatewayAddresses:  buildGatewayAddress(testAddress1),
						ClusterAttributes: ClusterAttributes{},
					},
					{
						ClusterName:       clusterName2,
						GatewayAddresses:  buildGatewayAddress(testAddress2),
						ClusterAttributes: ClusterAttributes{},
					},
				},
				loadBalancing: nil,
			},
			want: &MultiClusterGatewayTarget{
				Gateway: gateway,
				ClusterGatewayTargets: []ClusterGatewayTarget{
					{
						ClusterGateway: &ClusterGateway{
							ClusterName:       clusterName1,
							GatewayAddresses:  buildGatewayAddress(testAddress1),
							ClusterAttributes: ClusterAttributes{},
						},
						Geo:    testutil.Pointer(DefaultGeo),
						Weight: testutil.Pointer(DefaultWeight),
					},
					{
						ClusterGateway: &ClusterGateway{
							ClusterName:       clusterName2,
							GatewayAddresses:  buildGatewayAddress(testAddress2),
							ClusterAttributes: ClusterAttributes{},
						},
						Geo:    testutil.Pointer(DefaultGeo),
						Weight: testutil.Pointer(DefaultWeight),
					},
				},
				LoadBalancing: nil,
			},
		},
		{
			name: "set cluster gateway targets with default geo and weight from load balancing config",
			args: args{
				gateway: gateway,
				clusterGateways: []ClusterGateway{
					{
						ClusterName:       clusterName1,
						GatewayAddresses:  buildGatewayAddress(testAddress1),
						ClusterAttributes: ClusterAttributes{},
					},
					{
						ClusterName:       clusterName2,
						GatewayAddresses:  buildGatewayAddress(testAddress2),
						ClusterAttributes: ClusterAttributes{},
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
							ClusterName:       clusterName1,
							GatewayAddresses:  buildGatewayAddress(testAddress1),
							ClusterAttributes: ClusterAttributes{},
						},
						Geo:    testutil.Pointer(GeoCode("IE")),
						Weight: testutil.Pointer(255),
					},
					{
						ClusterGateway: &ClusterGateway{
							ClusterName:       clusterName2,
							GatewayAddresses:  buildGatewayAddress(testAddress2),
							ClusterAttributes: ClusterAttributes{},
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
		},
		{
			name: "set cluster gateway targets with default geo and weight from cluster attributes",
			args: args{
				gateway: gateway,
				clusterGateways: []ClusterGateway{
					{
						ClusterName:      clusterName1,
						GatewayAddresses: buildGatewayAddress(testAddress1),
						ClusterAttributes: ClusterAttributes{
							CustomWeight: testutil.Pointer("TSTATTR"),
							Geo:          testutil.Pointer(GeoCode("EU")),
						},
					},
					{
						ClusterName:       clusterName2,
						GatewayAddresses:  buildGatewayAddress(testAddress2),
						ClusterAttributes: ClusterAttributes{},
					},
				},
				loadBalancing: &v1alpha1.LoadBalancingSpec{
					Weighted: &v1alpha1.LoadBalancingWeighted{
						DefaultWeight: 255,
						Custom: []*v1alpha1.CustomWeight{
							{
								Value:  "TSTATTR",
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
							ClusterName:      clusterName1,
							GatewayAddresses: buildGatewayAddress(testAddress1),
							ClusterAttributes: ClusterAttributes{
								CustomWeight: testutil.Pointer("TSTATTR"),
								Geo:          testutil.Pointer(GeoCode("EU")),
							},
						},
						Geo:    testutil.Pointer(GeoCode("EU")),
						Weight: testutil.Pointer(60),
					},
					{
						ClusterGateway: &ClusterGateway{
							ClusterName:       clusterName2,
							GatewayAddresses:  buildGatewayAddress(testAddress2),
							ClusterAttributes: ClusterAttributes{},
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
								Value:  "TSTATTR",
								Weight: 60,
							},
						},
					},
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "IE",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMultiClusterGatewayTarget(tt.args.gateway, tt.args.clusterGateways, tt.args.loadBalancing); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMultiClusterGatewayTarget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToBase36hash(t *testing.T) {
	tests := []struct {
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
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := ToBase36hash(tt.in); got != tt.want {
				t.Errorf("ToBase36hash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func buildGatewayAddress(value string) []gatewayv1beta1.GatewayAddress {
	return []gatewayv1beta1.GatewayAddress{
		{
			Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
			Value: value,
		},
	}
}
