//go:build unit

package test

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

type FakeCertificateService struct {
	controlClient client.Client
}

func (s *FakeCertificateService) CleanupCertificates(_ context.Context, _ traffic.Interface) error {
	return nil
}

func (s *FakeCertificateService) EnsureCertificate(_ context.Context, host string, _ v1.Object) error {
	if host == testutil.FailEnsureCertHost {
		return fmt.Errorf(testutil.FailEnsureCertHost)
	}
	return nil
}

func (s *FakeCertificateService) GetCertificateSecret(ctx context.Context, host string, namespace string) (*corev1.Secret, error) {
	if host == testutil.FailGetCertSecretHost {
		return &corev1.Secret{}, fmt.Errorf(testutil.FailGetCertSecretHost)
	}
	tlsSecret := &corev1.Secret{ObjectMeta: v1.ObjectMeta{
		Name:      host,
		Namespace: namespace,
	}}
	if err := s.controlClient.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
		return nil, err
	}
	return tlsSecret, nil
}

func NewTestCertificateService(client client.Client) *FakeCertificateService {
	return &FakeCertificateService{controlClient: client}
}

type FakeGatewayPlacer struct{}

func (p *FakeGatewayPlacer) GetClusterGateway(_ context.Context, _ *v1beta1.Gateway, _ string) (dns.ClusterGateway, error) {
	return dns.ClusterGateway{}, nil
}

func (p *FakeGatewayPlacer) Place(_ context.Context, upstream *v1beta1.Gateway, _ *v1beta1.Gateway, _ ...v1.Object) (sets.Set[string], error) {
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

func (p *FakeGatewayPlacer) GetPlacedClusters(_ context.Context, gateway *v1beta1.Gateway) (sets.Set[string], error) {
	if gateway.Labels == nil {
		return nil, nil
	}
	placedClusters := sets.Set[string](sets.NewString())
	placedClusters.Insert(testutil.Cluster)
	return placedClusters, nil
}

func (p *FakeGatewayPlacer) GetClusters(_ context.Context, _ *v1beta1.Gateway) (sets.Set[string], error) {
	return nil, nil
}

func (p *FakeGatewayPlacer) ListenerTotalAttachedRoutes(_ context.Context, _ *v1beta1.Gateway, listenerName string, _ string) (int, error) {
	if listenerName == testutil.ValidTestHostname {
		return 1, nil
	}
	return 0, nil
}

func (p *FakeGatewayPlacer) GetAddresses(_ context.Context, _ *v1beta1.Gateway, _ string) ([]v1beta1.GatewayAddress, error) {
	t := v1beta1.IPAddressType
	return []v1beta1.GatewayAddress{
		{
			Type:  &t,
			Value: "1.1.1.1",
		},
	}, nil
}

func NewTestGatewayPlacer() *FakeGatewayPlacer {
	return &FakeGatewayPlacer{}
}
