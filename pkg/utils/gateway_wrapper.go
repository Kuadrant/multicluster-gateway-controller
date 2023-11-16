package utils

import (
	"fmt"
	"strings"

	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	SingleClusterAddressValue = "kudarant.io/single"

	MultiClusterIPAddressType       gatewayapiv1.AddressType = "kuadrant.io/MultiClusterIPAddress"
	MultiClusterHostnameAddressType gatewayapiv1.AddressType = "kuadrant.io/MultiClusterHostnameAddress"
)

type GatewayWrapper struct {
	*gatewayapiv1.Gateway
}

func NewGatewayWrapper(g *gatewayapiv1.Gateway) *GatewayWrapper {
	return &GatewayWrapper{Gateway: g}
}

func isMultiClusterAddressType(addressType gatewayapiv1.AddressType) bool {
	return addressType == MultiClusterIPAddressType || addressType == MultiClusterHostnameAddressType
}

// IsMultiCluster reports a type of the first address in the Status block
// returns false if no addresses present
func (g *GatewayWrapper) IsMultiCluster() bool {
	if len(g.Status.Addresses) > 0 {
		return isMultiClusterAddressType(*g.Status.Addresses[0].Type)
	}
	return false
}

// Validate ensures correctly configured underlying Gateway object
// Returns nil if validation passed
func (g *GatewayWrapper) Validate() error {

	// Status.Addresses validation
	// Compares all addresses against the first address to ensure the same type
	for _, address := range g.Status.Addresses {
		if g.IsMultiCluster() == isMultiClusterAddressType(*address.Type) {
			continue
		}
		return fmt.Errorf("gateway is invalid: inconsistent status addresses")
	}
	return nil
}

// GetClusterGatewayAddresses constructs a map from Status.Addresses of underlying Gateway
// with key being a cluster and value being an address in the cluster.
// In case of a single-cluster Gateway the key is SingleClusterAddressValue
func (g *GatewayWrapper) GetClusterGatewayAddresses() map[string][]gatewayapiv1.GatewayAddress {
	clusterAddrs := make(map[string][]gatewayapiv1.GatewayAddress, len(g.Status.Addresses))

	for _, address := range g.Status.Addresses {
		//Default to Single Cluster (Normal Gateway Status)
		cluster := SingleClusterAddressValue
		addressValue := address.Value

		//Check for Multi Cluster (MGC Gateway Status)
		if g.IsMultiCluster() {
			tmpCluster, tmpAddress, found := strings.Cut(address.Value, "/")
			//If this fails something is wrong and the value hasn't been set correctly
			if found {
				cluster = tmpCluster
				addressValue = tmpAddress
			}
		}

		if _, ok := clusterAddrs[cluster]; !ok {
			clusterAddrs[cluster] = []gatewayapiv1.GatewayAddress{}
		}

		clusterAddrs[cluster] = append(clusterAddrs[cluster], gatewayapiv1.GatewayAddress{
			Type:  address.Type,
			Value: addressValue,
		})
	}

	return clusterAddrs
}

// ListenerTotalAttachedRoutes returns a count of attached routes from the Status.Listeners for a specified
// combination of downstreamClusterName and specListener.Name
func (g *GatewayWrapper) ListenerTotalAttachedRoutes(downstreamClusterName string, specListener gatewayapiv1.Listener) int {
	for _, statusListener := range g.Status.Listeners {
		// for Multi Cluster (MGC Gateway Status)
		if g.IsMultiCluster() {
			clusterName, listenerName, found := strings.Cut(string(statusListener.Name), ".")
			if !found {
				return 0
			}
			if clusterName == downstreamClusterName && listenerName == string(specListener.Name) {
				return int(statusListener.AttachedRoutes)
			}
		}
		// Single Cluster (Normal Gateway Status)
		if string(statusListener.Name) == string(specListener.Name) {
			return int(statusListener.AttachedRoutes)
		}
	}

	return 0
}

// AddressTypeToMultiCluster returns a multi cluster version of the address type
// and a bool to indicate that provided address type was converted. If not - original type is returned
func AddressTypeToMultiCluster(address gatewayapiv1.GatewayAddress) (gatewayapiv1.AddressType, bool) {
	if *address.Type == gatewayapiv1.IPAddressType {
		return MultiClusterIPAddressType, true
	} else if *address.Type == gatewayapiv1.HostnameAddressType {
		return MultiClusterHostnameAddressType, true
	}
	return *address.Type, false
}

// AddressTypeToSingleCluster converts provided multicluster address to single cluster version
// the bool indicates a successful conversion
func AddressTypeToSingleCluster(address gatewayapiv1.GatewayAddress) (gatewayapiv1.AddressType, bool) {
	if *address.Type == MultiClusterIPAddressType {
		return gatewayapiv1.IPAddressType, true
	} else if *address.Type == MultiClusterHostnameAddressType {
		return gatewayapiv1.HostnameAddressType, true
	}
	return *address.Type, false
}
