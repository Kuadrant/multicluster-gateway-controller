//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
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

func (f FakeOCMPlacer) Place(ctx context.Context, upstream *gatewayv1beta1.Gateway, downstream *gatewayv1beta1.Gateway, children ...metav1.Object) (sets.Set[string], error) {
	return nil, nil
}

func (f FakeOCMPlacer) GetPlacedClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	clusters := sets.Set[string](sets.NewString())
	for _, cluster := range f.placedClusters {
		if gateway.Name == f.placedGatewayName {
			clusters.Insert(cluster.name)
		}
	}
	return clusters, nil
}

func (f FakeOCMPlacer) GetClusters(ctx context.Context, gateway *gatewayv1beta1.Gateway) (sets.Set[string], error) {
	return f.GetPlacedClusters(ctx, gateway)
}

func (f FakeOCMPlacer) ListenerTotalAttachedRoutes(ctx context.Context, gateway *gatewayv1beta1.Gateway, listenerName string, downstream string) (int, error) {
	count := 0
	for _, placedCluster := range f.placedClusters {
		if gateway.Name == f.placedGatewayName && (listenerName == f.attachedRouteName || listenerName == TestWildCardListenerName) && downstream == placedCluster.name {
			count = 1
		}
	}
	return count, nil
}

func (f FakeOCMPlacer) GetAddresses(ctx context.Context, gateway *gatewayv1beta1.Gateway, downstream string) ([]gatewayv1beta1.GatewayAddress, error) {
	gwAddresses := []gatewayv1beta1.GatewayAddress{}
	t := gatewayv1beta1.IPAddressType
	for _, cluster := range f.placedClusters {
		if gateway.Name == f.placedGatewayName && downstream == cluster.name {
			gwAddresses = append(gwAddresses, gatewayv1beta1.GatewayAddress{
				Type:  &t,
				Value: cluster.attachedRouteAddress,
			})
		}
	}
	return gwAddresses, nil
}

func (f FakeOCMPlacer) GetClusterGateway(ctx context.Context, gateway *gatewayv1beta1.Gateway, clusterName string) (dns.ClusterGateway, error) {
	gwAddresses, _ := f.GetAddresses(ctx, gateway, clusterName)
	cgw := dns.ClusterGateway{
		Cluster: &testutil.TestResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		},
		GatewayAddresses: gwAddresses,
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
	Eventually(func() error {
		return testClient().Get(context.Background(), types.NamespacedName{Name: generatedTestNamespace}, existingNamespace)
	}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

	*namespace = existingNamespace.Name
}

func DeleteNamespace(namespace *string) {
	desiredTestNamespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: *namespace}}
	err := testClient().Delete(context.Background(), desiredTestNamespace, client.PropagationPolicy(metav1.DeletePropagationForeground))

	Expect(err).ToNot(HaveOccurred())

	existingNamespace := &v1.Namespace{}
	Eventually(func() error {
		err := testClient().Get(context.Background(), types.NamespacedName{Name: *namespace}, existingNamespace)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, 3*time.Minute, 2*time.Second).Should(BeNil())
}

type testHealthServer struct {
	Port int
}

func (s *testHealthServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	endpoint := func(expectedCode int) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(expectedCode)
		}
	}

	mux.HandleFunc("/healthy", endpoint(200))
	mux.HandleFunc("/unhealthy", endpoint(500))

	errCh := make(chan error)

	go func() {
		errCh <- http.ListenAndServe(fmt.Sprintf(":%d", s.Port), mux)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
