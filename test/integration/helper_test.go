//go:build integration

package integration

import (
	"context"
	"time"

	"github.com/google/uuid"

	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	TestTimeoutMedium         = time.Second * 10
	ConsistentlyTimeoutMedium = time.Second * 60
	TestRetryIntervalMedium   = time.Millisecond * 250
	TestPlacedGatewayName     = "test-placed-gateway"
	TestPlacedClusterName     = "test-placed-cluster"
	TestAttachedRouteName     = "test.example.com"
	TestAttachedRouteAddress  = "172.0.0.3"
	nsSpoke1Name              = "test-spoke-cluster-1"
	nsSpoke2Name              = "test-spoke-cluster-2"
	defaultNS                 = "default"
	gatewayFinalizer          = "kuadrant.io/gateway"
	providerCredential        = "secretname"
)

// FakeOCMPlacer has one gateway called `placedGatewayName` placed on one cluster called `placedClusterName` with one
// attached route called `attachedRouteName` with an address value of `attachedRouteAddress`
type FakeOCMPlacer struct {
	placedGatewayName    string
	placedClusterName    string
	attachedRouteName    string
	attachedRouteAddress string
}

func NewFakeOCMPlacer(placedGatewayName, placedClusterName, attachedRouteName, attachedRouteAddress string) *FakeOCMPlacer {
	return &FakeOCMPlacer{
		placedGatewayName:    placedGatewayName,
		placedClusterName:    placedClusterName,
		attachedRouteName:    attachedRouteName,
		attachedRouteAddress: attachedRouteAddress,
	}
}

func NewTestOCMPlacer() *FakeOCMPlacer {
	return NewFakeOCMPlacer(TestPlacedGatewayName, TestPlacedClusterName, TestAttachedRouteName, TestAttachedRouteAddress)
}

func (f FakeOCMPlacer) Place(ctx context.Context, upstream *gatewayv1beta1.Gateway, downstream *gatewayv1beta1.Gateway, children ...metav1.Object) (sets.Set[string], error) {
	return nil, nil
}

func (f FakeOCMPlacer) GetPlacedClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	clusters := sets.Set[string](sets.NewString())
	if gateway.Name == f.placedGatewayName {
		clusters.Insert(f.placedClusterName)
	}
	return clusters, nil
}

func (f FakeOCMPlacer) GetClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	return f.GetPlacedClusters(ctx, gateway)
}

func (f FakeOCMPlacer) ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayv1beta1.Gateway, listenerName string, downstream string) (int, error) {
	count := 0
	if gateway.Name == f.placedGatewayName && listenerName == f.attachedRouteName && downstream == f.placedClusterName {
		count = 1
	}
	return count, nil
}

func (f FakeOCMPlacer) GetAddresses(ctx context.Context, gateway *gatewayv1beta1.Gateway, downstream string) ([]gatewayv1beta1.GatewayAddress, error) {
	gwAddresses := []gatewayv1beta1.GatewayAddress{}
	if gateway.Name == f.placedGatewayName && downstream == f.placedClusterName {
		t := gatewayv1beta1.IPAddressType
		gwAddresses = append(gwAddresses, gatewayv1beta1.GatewayAddress{
			Type:  &t,
			Value: f.attachedRouteAddress,
		})
	}
	return gwAddresses, nil
}

func (f FakeOCMPlacer) GetClusterGateway(ctx context.Context, gateway *gatewayv1beta1.Gateway, clusterName string) (dns.ClusterGateway, error) {
	gwAddresses, _ := f.GetAddresses(ctx, gateway, clusterName)
	cgw := dns.ClusterGateway{
		ClusterName:       clusterName,
		GatewayAddresses:  gwAddresses,
		ClusterAttributes: dns.ClusterAttributes{},
	}
	return cgw, nil
}

func CreateNamespace(namespace *string) {
	var generatedTestNamespace = "test-namespace-" + uuid.New().String()

	nsObject := &v1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: generatedTestNamespace},
	}

	err := testClient().Create(context.Background(), nsObject)
	Expect(err).ToNot(HaveOccurred())

	existingNamespace := &v1.Namespace{}
	Eventually(func() bool {
		err := testClient().Get(context.Background(), types.NamespacedName{Name: generatedTestNamespace}, existingNamespace)
		return err == nil
	}, time.Minute, 5*time.Second).Should(BeTrue())

	*namespace = existingNamespace.Name
}

func DeleteNamespace(namespace *string) {
	desiredTestNamespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: *namespace}}
	err := testClient().Delete(context.Background(), desiredTestNamespace, client.PropagationPolicy(metav1.DeletePropagationForeground))

	Expect(err).ToNot(HaveOccurred())

	existingNamespace := &v1.Namespace{}
	Eventually(func() bool {
		err := testClient().Get(context.Background(), types.NamespacedName{Name: *namespace}, existingNamespace)
		if err != nil && k8serrors.IsNotFound(err) {
			return true
		}
		return false
	}, 3*time.Minute, 2*time.Second).Should(BeTrue())
}
