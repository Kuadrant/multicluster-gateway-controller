package utils

import (
	"reflect"
	"testing"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestAddressTypeToMultiCluster(t *testing.T) {
	tests := []struct {
		name            string
		addressType     gatewayapiv1.AddressType
		wantAddressType gatewayapiv1.AddressType
		converted       bool
	}{
		{
			name:            "supported IP address",
			addressType:     gatewayapiv1.IPAddressType,
			wantAddressType: MultiClusterIPAddressType,
			converted:       true,
		},
		{
			name:            "supported host address",
			addressType:     gatewayapiv1.HostnameAddressType,
			wantAddressType: MultiClusterHostnameAddressType,
			converted:       true,
		},
		{
			name:            "not supported address",
			addressType:     gatewayapiv1.NamedAddressType,
			wantAddressType: gatewayapiv1.NamedAddressType,
			converted:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddressType, converted := AddressTypeToMultiCluster(gatewayapiv1.GatewayAddress{Type: &tt.addressType})
			if gotAddressType != tt.wantAddressType {
				t.Errorf("AddressTypeToMultiCluster() got = %v, want %v", gotAddressType, tt.wantAddressType)
			}
			if converted != tt.converted {
				t.Errorf("AddressTypeToMultiCluster() got1 = %v, want %v", converted, tt.converted)
			}
		})
	}
}

func TestAddressTypeToSingleCluster(t *testing.T) {
	tests := []struct {
		name            string
		addressType     gatewayapiv1.AddressType
		wantAddressType gatewayapiv1.AddressType
		converted       bool
	}{
		{
			name:            "supported IP address",
			addressType:     MultiClusterIPAddressType,
			wantAddressType: gatewayapiv1.IPAddressType,
			converted:       true,
		},
		{
			name:            "supported host address",
			addressType:     MultiClusterHostnameAddressType,
			wantAddressType: gatewayapiv1.HostnameAddressType,
			converted:       true,
		},
		{
			name:            "not supported address",
			addressType:     gatewayapiv1.NamedAddressType,
			wantAddressType: gatewayapiv1.NamedAddressType,
			converted:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddressType, converted := AddressTypeToSingleCluster(v1beta1.GatewayAddress{Type: &tt.addressType})
			if gotAddressType != tt.wantAddressType {
				t.Errorf("AddressTypeToMultiCluster() got = %v, want %v", gotAddressType, tt.wantAddressType)
			}
			if converted != tt.converted {
				t.Errorf("AddressTypeToMultiCluster() got1 = %v, want %v", converted, tt.converted)
			}
		})
	}
}

func TestGatewayWrapper_GetClusterGatewayAddresses(t *testing.T) {
	tests := []struct {
		name    string
		Gateway *gatewayapiv1.Gateway
		want    map[string][]gatewayapiv1.GatewayStatusAddress
	}{
		{
			name: "single cluster Gateway",
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name: "testgw",
				},
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(v1beta1.IPAddressType),
							Value: "1.1.1.1",
						},
					},
				},
			},
			want: map[string][]gatewayapiv1.GatewayStatusAddress{
				"testgw": {
					{
						Type:  testutil.Pointer(v1beta1.IPAddressType),
						Value: "1.1.1.1",
					},
				},
			},
		},
		{
			name: "multi cluster Gateway",
			Gateway: &gatewayapiv1.Gateway{
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-workload-1/2.2.2.2",
						},
						{
							Type:  testutil.Pointer(MultiClusterHostnameAddressType),
							Value: "kind-mgc-workload-1/boop.com",
						},
					},
				},
			},
			want: map[string][]gatewayapiv1.GatewayStatusAddress{
				"kind-mgc-control-plane": {
					{
						Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
						Value: "1.1.1.1",
					},
				},
				"kind-mgc-workload-1": {
					{
						Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
						Value: "2.2.2.2",
					},
					{
						Type:  testutil.Pointer(gatewayapiv1.HostnameAddressType),
						Value: "boop.com",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGatewayWrapper(tt.Gateway)
			if got := g.GetClusterGatewayAddresses(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClusterGatewayAddresses() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGatewayWrapper_Validate(t *testing.T) {
	tests := []struct {
		name    string
		Gateway *gatewayapiv1.Gateway
		wantErr bool
	}{
		{
			name: "Valid Gateway",
			Gateway: &gatewayapiv1.Gateway{
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-workload-1/2.2.2.2",
						},
						{
							Type:  testutil.Pointer(MultiClusterHostnameAddressType),
							Value: "kind-mgc-workload-1/boop.com",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid Gateway: inconsistent addresses",
			Gateway: &gatewayapiv1.Gateway{
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
						{
							Type:  testutil.Pointer(v1beta1.NamedAddressType),
							Value: "kind-mgc-workload-1/boop.com",
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGatewayWrapper(tt.Gateway)
			if err := g.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGatewayWrapper_GetClusterGatewayLabels(t *testing.T) {
	tests := []struct {
		name        string
		Gateway     *gatewayapiv1.Gateway
		clusterName string
		want        map[string]string
	}{
		{
			name: "single cluster gateway",
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name: "testgw",
					Labels: map[string]string{
						"kuadrant.io/lb-attribute-weight":   "TSTATTR",
						"kuadrant.io/lb-attribute-geo-code": "EU",
						"foo":                               "bar",
					},
				},
			},
			clusterName: "foo",
			want: map[string]string{
				"kuadrant.io/lb-attribute-weight":   "TSTATTR",
				"kuadrant.io/lb-attribute-geo-code": "EU",
				"foo":                               "bar",
			},
		},
		{
			name: "multi cluster gateway",
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name: "testgw",
					Labels: map[string]string{
						"kuadrant.io/lb-attribute-weight":   "TSTATTR",
						"kuadrant.io/lb-attribute-geo-code": "EU",
						"foo":                               "bar",
					},
				},
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-workload-1/2.2.2.2",
						},
					},
				},
			},
			clusterName: "kind-mgc-control-plane",
			want: map[string]string{
				"kuadrant.io/lb-attribute-weight":   "TSTATTR",
				"kuadrant.io/lb-attribute-geo-code": "EU",
				"foo":                               "bar",
			},
		},
		{
			name: "multi cluster gateway with cluster labels",
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name: "testgw",
					Labels: map[string]string{
						"kuadrant.io/kind-mgc-control-plane_lb-attribute-weight":   "TSTATTR",
						"kuadrant.io/kind-mgc-control-plane_lb-attribute-geo-code": "EU",
						"kuadrant.io/kind-mgc-workload-1_lb-attribute-weight":      "TSTATTR2",
						"kuadrant.io/kind-mgc-workload-1_lb-attribute-geo-code":    "US",
						"foo": "bar",
					},
				},
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-workload-1/2.2.2.2",
						},
					},
				},
			},
			clusterName: "kind-mgc-control-plane",
			want: map[string]string{
				"kuadrant.io/lb-attribute-weight":                       "TSTATTR",
				"kuadrant.io/lb-attribute-geo-code":                     "EU",
				"kuadrant.io/kind-mgc-workload-1_lb-attribute-weight":   "TSTATTR2",
				"kuadrant.io/kind-mgc-workload-1_lb-attribute-geo-code": "US",
				"foo": "bar",
			},
		},
		{
			name: "multi cluster gateway with mix of cluster labels",
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name: "testgw",
					Labels: map[string]string{
						"kuadrant.io/kind-mgc-control-plane_lb-attribute-weight":   "TSTATTR",
						"kuadrant.io/kind-mgc-control-plane_lb-attribute-geo-code": "EU",
						"kuadrant.io/kind-mgc-workload-1_lb-attribute-weight":      "TSTATTR2",
						"kuadrant.io/kind-mgc-workload-1_lb-attribute-geo-code":    "US",
						"kuadrant.io/lb-attribute-weight":                          "TSTATTR3",
						"kuadrant.io/lb-attribute-geo-code":                        "ES",
						"foo":                                                      "bar",
					},
				},
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-workload-1/2.2.2.2",
						},
					},
				},
			},
			clusterName: "kind-mgc-control-plane",
			want: map[string]string{
				"kuadrant.io/lb-attribute-weight":                       "TSTATTR",
				"kuadrant.io/lb-attribute-geo-code":                     "EU",
				"kuadrant.io/kind-mgc-workload-1_lb-attribute-weight":   "TSTATTR2",
				"kuadrant.io/kind-mgc-workload-1_lb-attribute-geo-code": "US",
				"foo": "bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GatewayWrapper{
				Gateway: tt.Gateway,
			}
			if got := g.GetClusterGatewayLabels(tt.clusterName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetClusterGatewayLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
