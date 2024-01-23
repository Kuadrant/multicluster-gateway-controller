//go:build unit

package fake

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

type FakeGatewayPlacer struct{}

func NewTestGatewayPlacer() *FakeGatewayPlacer {
	return &FakeGatewayPlacer{}
}

func (p *FakeGatewayPlacer) Place(_ context.Context, upstream *gatewayapiv1.Gateway, _ *gatewayapiv1.Gateway, _ ...metav1.Object) (sets.Set[string], error) {
	if upstream.Labels == nil {
		return nil, nil
	}
	if *upstream.Spec.Listeners[0].Hostname == testutil.FailPlacementHostname {
		return nil, fmt.Errorf(testutil.FailPlacementHostname)
	}
	targetClusters := sets.Set[string](sets.NewString())
	targetClusters.Insert(testutil.Cluster)
	return targetClusters, nil
}

func (p *FakeGatewayPlacer) GetPlacedClusters(_ context.Context, gateway *gatewayapiv1.Gateway) (sets.Set[string], error) {
	if gateway.Labels == nil {
		return nil, nil
	}
	placedClusters := sets.Set[string](sets.NewString())
	placedClusters.Insert(testutil.Cluster)
	return placedClusters, nil
}

func (p *FakeGatewayPlacer) GetClusters(_ context.Context, _ *gatewayapiv1.Gateway) (sets.Set[string], error) {
	return nil, nil
}

func (p *FakeGatewayPlacer) ListenerTotalAttachedRoutes(_ context.Context, _ *gatewayapiv1.Gateway, listenerName string, _ string) (int, error) {
	if listenerName == testutil.Cluster {
		return 1, nil
	}
	return 0, nil
}

func (p *FakeGatewayPlacer) GetAddresses(_ context.Context, _ *gatewayapiv1.Gateway, _ string) ([]gatewayapiv1.GatewayAddress, error) {
	t := gatewayapiv1.IPAddressType
	return []gatewayapiv1.GatewayAddress{
		{
			Type:  &t,
			Value: "1.1.1.1",
		},
	}, nil
}
