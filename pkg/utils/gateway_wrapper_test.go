package utils

import (
	"reflect"
	"testing"

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
		want    map[string][]gatewayapiv1.GatewayAddress
	}{
		{
			name: "single cluster Gateway",
			Gateway: &gatewayapiv1.Gateway{
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(v1beta1.IPAddressType),
							Value: "1.1.1.1",
						},
					},
				},
			},
			want: map[string][]gatewayapiv1.GatewayAddress{
				SingleClusterNameValue: {
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
			want: map[string][]gatewayapiv1.GatewayAddress{
				"kind-mgc-control-plane": {
					{
						Type:  testutil.Pointer(MultiClusterIPAddressType),
						Value: "1.1.1.1",
					},
				},
				"kind-mgc-workload-1": {
					{
						Type:  testutil.Pointer(MultiClusterIPAddressType),
						Value: "2.2.2.2",
					},
					{
						Type:  testutil.Pointer(MultiClusterHostnameAddressType),
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

func TestGatewayWrapper_ListenerTotalAttachedRoutes(t *testing.T) {

	tests := []struct {
		name                  string
		Gateway               *gatewayapiv1.Gateway
		downstreamClusterName string
		want                  int
	}{
		{
			name: "single cluster gateway",
			Gateway: &gatewayapiv1.Gateway{
				Spec: gatewayapiv1.GatewaySpec{
					Listeners: []gatewayapiv1.Listener{
						{
							Name: "api",
						},
					},
				},
				Status: gatewayapiv1.GatewayStatus{
					Listeners: []gatewayapiv1.ListenerStatus{
						{
							Name:           "api",
							AttachedRoutes: 1,
						},
					},
				},
			},
			downstreamClusterName: SingleClusterNameValue,
			want:                  1,
		},
		{
			name: "multi cluster gateway",
			Gateway: &gatewayapiv1.Gateway{
				Spec: gatewayapiv1.GatewaySpec{
					Listeners: []gatewayapiv1.Listener{
						{
							Name: "api",
						},
					},
				},
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
					},
					Listeners: []gatewayapiv1.ListenerStatus{
						{
							Name:           "kind-mgc-control-plane.api",
							AttachedRoutes: 1,
						},
					},
				},
			},
			downstreamClusterName: "kind-mgc-control-plane",
			want:                  1,
		},
		{
			name: "invalid status listener name",
			Gateway: &gatewayapiv1.Gateway{
				Spec: gatewayapiv1.GatewaySpec{
					Listeners: []gatewayapiv1.Listener{
						{
							Name: "api",
						},
					},
				},
				Status: gatewayapiv1.GatewayStatus{
					Addresses: []gatewayapiv1.GatewayStatusAddress{
						{
							Type:  testutil.Pointer(MultiClusterIPAddressType),
							Value: "kind-mgc-control-plane/1.1.1.1",
						},
					},
					Listeners: []gatewayapiv1.ListenerStatus{
						{
							Name:           "kind-mgc-control-plane-api",
							AttachedRoutes: 1,
						},
					},
				},
			},
			want: 0,
		},
		{
			name: "no status",
			Gateway: &gatewayapiv1.Gateway{
				Spec: gatewayapiv1.GatewaySpec{
					Listeners: []gatewayapiv1.Listener{
						{
							Name: "api",
						},
					},
				},
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGatewayWrapper(tt.Gateway)
			if got := g.ListenerTotalAttachedRoutes(tt.downstreamClusterName, tt.Gateway.Spec.Listeners[0]); got != tt.want {
				t.Errorf("ListenerTotalAttachedRoutes() = %v, want %v", got, tt.want)
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
