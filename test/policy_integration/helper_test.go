//go:build integration

package policy_integration

import (
	"context"
	"fmt"
	"net/http"
	"time"
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
