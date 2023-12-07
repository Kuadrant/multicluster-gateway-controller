//go:build integration

package gateway_integration

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	TestTimeoutMedium            = time.Second * 10
	TestTimeoutLong              = time.Second * 30
	ConsistentlyTimeoutMedium    = time.Second * 60
	TestRetryIntervalMedium      = time.Millisecond * 250
	TestPlacedGatewayName        = "test-placed-gateway"
	TestPlacedClusterControlName = "test-placed-control"
	TestPlaceClusterWorkloadName = "test-placed-workload-1"
	TestAttachedRouteName        = "test.example.com"
	OtherAttachedRouteName       = "other.example.com"
	TestWildCardListenerName     = "wildcard"
	TestWildCardListenerHost     = "*.example.com"
	TestAttachedRouteAddressOne  = "172.0.0.1"
	TestAttachedRouteAddressTwo  = "172.0.0.2"
	nsSpoke1Name                 = "test-spoke-cluster-1"
	nsSpoke2Name                 = "test-spoke-cluster-2"
	defaultNS                    = "default"
	gatewayFinalizer             = "kuadrant.io/gateway"
	providerCredential           = "secretname"
)

// FakeOCMPlacer has one gateway called "test-placed-gateway"
// placed on two clusters called
// "test-placed-control" with address value of "172.0.0.3" and
// "test-placed-workload-1" with address value of "172.0.0.4" with one
// attached route "test.example.com"

type placedClusters struct {
	name                 string
	attachedRouteAddress string
}

type FakeOCMPlacer struct {
	placedGatewayName string
	placedClusters    []placedClusters
	attachedRouteName string
}

func NewFakeOCMPlacer(placedGatewayName, attachedRouteName string) *FakeOCMPlacer {
	return &FakeOCMPlacer{
		placedGatewayName: placedGatewayName,
		placedClusters: []placedClusters{
			{
				name:                 TestPlacedClusterControlName,
				attachedRouteAddress: TestAttachedRouteAddressOne,
			},
			{
				name:                 TestPlaceClusterWorkloadName,
				attachedRouteAddress: TestAttachedRouteAddressTwo,
			},
		},
		attachedRouteName: attachedRouteName,
	}
}

func NewTestOCMPlacer() *FakeOCMPlacer {
	return NewFakeOCMPlacer(TestPlacedGatewayName, TestAttachedRouteName)
}

func (f FakeOCMPlacer) Place(_ context.Context, _ *gatewayapiv1.Gateway, _ *gatewayapiv1.Gateway, _ ...metav1.Object) (sets.Set[string], error) {
	return nil, nil
}

func (f FakeOCMPlacer) GetPlacedClusters(_ context.Context, gateway *gatewayapiv1.Gateway) (sets.Set[string], error) {
	clusters := sets.Set[string](sets.NewString())
	for _, cluster := range f.placedClusters {
		if gateway.Name == f.placedGatewayName {
			clusters.Insert(cluster.name)
		}
	}
	return clusters, nil
}

func (f FakeOCMPlacer) GetClusters(ctx context.Context, gateway *gatewayapiv1.Gateway) (sets.Set[string], error) {
	return f.GetPlacedClusters(ctx, gateway)
}

func (f FakeOCMPlacer) ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayapiv1.Gateway, listenerName string, downstream string) (int, error) {
	count := 0
	for _, placedCluster := range f.placedClusters {
		if gateway.Name == f.placedGatewayName && (listenerName == f.attachedRouteName || listenerName == TestWildCardListenerName) && downstream == placedCluster.name {
			count = 1
		}
	}
	return count, nil
}

func (f FakeOCMPlacer) GetAddresses(ctx context.Context, gateway *gatewayapiv1.Gateway, downstream string) ([]gatewayapiv1.GatewayAddress, error) {
	gwAddresses := []gatewayapiv1.GatewayAddress{}
	t := gatewayapiv1.IPAddressType
	for _, cluster := range f.placedClusters {
		if gateway.Name == f.placedGatewayName && downstream == cluster.name {
			gwAddresses = append(gwAddresses, gatewayapiv1.GatewayAddress{
				Type:  &t,
				Value: cluster.attachedRouteAddress,
			})
		}
	}
	return gwAddresses, nil
}
