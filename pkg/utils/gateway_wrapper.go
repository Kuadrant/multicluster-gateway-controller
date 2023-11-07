package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	MultiClusterIPAddressType       gatewayv1beta1.AddressType = "kuadrant.io/MultiClusterIPAddress"
	MultiClusterHostnameAddressType gatewayv1beta1.AddressType = "kuadrant.io/MultiClusterHostnameAddress"
)

type GatewayWrapper struct {
	*gatewayv1beta1.Gateway
	isMultiCluster bool
}

func NewGatewayWrapper(g *gatewayv1beta1.Gateway) (*GatewayWrapper, error) {
	gw := &GatewayWrapper{Gateway: g, isMultiCluster: false}

	for i, address := range gw.Status.Addresses {
		if i == 0 {
			gw.isMultiCluster = isMultiClusterAddressType(*address.Type)
			continue
		}
		if gw.isMultiCluster == isMultiClusterAddressType(*address.Type) {
			continue
		}
		return nil, fmt.Errorf("gateway is invalid: inconsistent status addresses")

	}
	return gw, nil
}

func isMultiClusterAddressType(addressType gatewayv1beta1.AddressType) bool {
	return addressType == MultiClusterIPAddressType || addressType == MultiClusterHostnameAddressType
}

func (a *GatewayWrapper) GetKind() string {
	return "GatewayWrapper"
}

func (a *GatewayWrapper) IsMultiCluster() bool {
	return a.isMultiCluster
}

func (a *GatewayWrapper) GetNamespaceName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: a.Namespace,
		Name:      a.Name,
	}
}

func (a *GatewayWrapper) String() string {
	return fmt.Sprintf("kind: %v, namespace/name: %v", a.GetKind(), a.GetNamespaceName())
}

// AddressTypeToMultiCluster returns a multi cluster version of the address type
// and a bool to indicate that provided address has supported type
func AddressTypeToMultiCluster(address gatewayv1beta1.GatewayAddress) (gatewayv1beta1.AddressType, bool) {
	if *address.Type == gatewayv1beta1.IPAddressType {
		return MultiClusterIPAddressType, true
	} else if *address.Type == gatewayv1beta1.HostnameAddressType {
		return MultiClusterHostnameAddressType, true
	}
	return "", false
}

// AddressTypeToSingleCluster returns a single cluster version of the address type
// and a bool to indicate that provided address was of the multi cluster type
func AddressTypeToSingleCluster(address gatewayv1beta1.GatewayAddress) (gatewayv1beta1.AddressType, bool) {
	if *address.Type == MultiClusterIPAddressType {
		return gatewayv1beta1.IPAddressType, true
	} else if *address.Type == MultiClusterHostnameAddressType {
		return gatewayv1beta1.HostnameAddressType, true
	}
	return "", false
}
