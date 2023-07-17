//go:build unit

package gateway

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/clusterSecret"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestClusterEventHandler(t *testing.T) {
	tlsConfig := `
				{
					"tlsClientConfig":
					  {
					    "insecure": false,
					    "caData": "test",
					    "certData": "test",
					    "keyData": "test"
					  }
				}
				`
	cases := []struct {
		name             string
		scheme           *runtime.Scheme
		gateways         []gatewayv1beta1.Gateway
		secret           corev1.Secret
		enqueuedGateways []gatewayv1beta1.Gateway
	}{
		{
			name:   "Queued one",
			scheme: testutil.GetValidTestScheme(),
			gateways: []gatewayv1beta1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testutil.GatewayClassName,
						Namespace: testutil.Namespace,
						Annotations: map[string]string{
							GatewayClusterLabelSelectorAnnotation: "type=test",
						},
					},
					Spec: gatewayv1beta1.GatewaySpec{
						Listeners: []gatewayv1beta1.Listener{
							{
								Hostname: testutil.Pointer(gatewayv1beta1.Hostname(testutil.ValidTestHostname)),
								Protocol: gatewayv1beta1.HTTPSProtocolType,
								TLS: &gatewayv1beta1.GatewayTLSConfig{
									CertificateRefs: []gatewayv1beta1.SecretObjectReference{
										{
											Name:      gatewayv1beta1.ObjectName(testutil.ValidTestHostname),
											Namespace: testutil.Pointer(gatewayv1beta1.Namespace(testutil.Namespace)),
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-another-gateway",
						Namespace: testutil.Namespace,
						Annotations: map[string]string{
							GatewayClusterLabelSelectorAnnotation: "another",
						},
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterSecret.CLUSTER_SECRET_LABEL: clusterSecret.CLUSTER_SECRET_LABEL_VALUE,
					},
					Name:      testutil.ValidTestHostname,
					Namespace: testutil.Namespace,
				},
				Data: map[string][]byte{
					"name":   []byte(testutil.Cluster),
					"config": []byte(tlsConfig),
				},
			},
			enqueuedGateways: []gatewayv1beta1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testutil.GatewayClassName,
						Namespace: testutil.Namespace,
					},
				},
			},
		},
		{
			name:     "Not enqueued. Not a cluster secret",
			scheme:   testutil.GetValidTestScheme(),
			gateways: testGateway(),
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated-secret",
					Namespace: testutil.Namespace,
				},
			},
			enqueuedGateways: make([]gatewayv1beta1.Gateway, 0),
		},
		{
			name:     "Not enqueued. Error parsing cluster config",
			scheme:   testutil.GetValidTestScheme(),
			gateways: testGateway(),
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterSecret.CLUSTER_SECRET_LABEL: clusterSecret.CLUSTER_SECRET_LABEL_VALUE,
					},
					Name:      "cluster",
					Namespace: testutil.Namespace,
				},
				Data: map[string][]byte{
					"name": []byte(testutil.Cluster),
					"config": []byte(
						`
				{
					"tlsClientConfig":
					  {					  
				}
				`),
				},
			},
			enqueuedGateways: make([]gatewayv1beta1.Gateway, 0),
		},
		{
			name:     "Not enqueued. Error listing gateways",
			scheme:   testutil.GetBasicScheme(),
			gateways: nil,
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterSecret.CLUSTER_SECRET_LABEL: clusterSecret.CLUSTER_SECRET_LABEL_VALUE,
					},
					Name:      "cluster",
					Namespace: testutil.Namespace,
				},
				Data: map[string][]byte{
					"name":   []byte(testutil.Cluster),
					"config": []byte(tlsConfig),
				},
			},
			enqueuedGateways: make([]gatewayv1beta1.Gateway, 0),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(testCase.scheme).WithLists(
				&gatewayv1beta1.GatewayList{
					Items: testCase.gateways,
				},
			).Build()

			testQ := &TestQueue{t: t}
			clusterEventHandler := &ClusterEventHandler{
				client: client,
			}

			clusterEventHandler.enqueueForObject(&testCase.secret, testQ)
			testQ.MustHaveEnqueued(testCase.enqueuedGateways)
		})
	}
}

type TestQueue struct {
	t                *testing.T
	enqueuedRequests map[types.NamespacedName]bool
}

func (q *TestQueue) Add(item interface{}) {
	req, ok := item.(ctrl.Request)
	if !ok {
		q.t.Fatalf("expected enqueued item to be of type ctrl.Request, got %v", item)
	}

	if q.enqueuedRequests == nil {
		q.enqueuedRequests = make(map[types.NamespacedName]bool)
	}

	q.enqueuedRequests[req.NamespacedName] = true
}

func (q *TestQueue) MustHaveEnqueued(gateways []gatewayv1beta1.Gateway) {
	enqueuedCopy := map[types.NamespacedName]bool{}
	for obj := range q.enqueuedRequests {
		enqueuedCopy[obj] = true
	}

	for _, gateway := range gateways {
		var nsn = types.NamespacedName{
			Namespace: gateway.Namespace,
			Name:      gateway.Name,
		}

		_, ok := q.enqueuedRequests[nsn]
		if !ok {
			q.t.Errorf("Object %s expected to be enqueued, but was not", nsn)
			continue
		}

		delete(enqueuedCopy, nsn)
	}

	for obj := range enqueuedCopy {
		q.t.Errorf("Object %s expected not to be enqueued, but was", obj)
	}
}

func testGateway() []gatewayv1beta1.Gateway {
	return []gatewayv1beta1.Gateway{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testutil.GatewayClassName,
				Namespace: testutil.Namespace,
				Annotations: map[string]string{
					GatewayClusterLabelSelectorAnnotation: "type=test",
				},
			},
		},
	}
}

// Unused methods below that must be defined for TestQueue to implement the
// workqueue.RateLimitingInterface:
//
// Done implements workqueue.RateLimitingInterface
func (*TestQueue) Done(item interface{}) {
	panic("unimplemented")
}

// Get implements workqueue.RateLimitingInterface
func (*TestQueue) Get() (item interface{}, shutdown bool) {
	panic("unimplemented")
}

// Len implements workqueue.RateLimitingInterface
func (*TestQueue) Len() int {
	panic("unimplemented")
}

// ShutDown implements workqueue.RateLimitingInterface
func (*TestQueue) ShutDown() {
	panic("unimplemented")
}

// ShutDownWithDrain implements workqueue.RateLimitingInterface
func (*TestQueue) ShutDownWithDrain() {
	panic("unimplemented")
}

// ShuttingDown implements workqueue.RateLimitingInterface
func (*TestQueue) ShuttingDown() bool {
	panic("unimplemented")
}

// AddAfter implements workqueue.RateLimitingInterface
func (*TestQueue) AddAfter(item interface{}, duration time.Duration) {
	panic("unimplemented")
}

// AddRateLimited implements workqueue.RateLimitingInterface
func (*TestQueue) AddRateLimited(item interface{}) {
	panic("unimplemented")
}

// Forget implements workqueue.RateLimitingInterface
func (*TestQueue) Forget(item interface{}) {
	panic("unimplemented")
}

// NumRequeues implements workqueue.RateLimitingInterface
func (*TestQueue) NumRequeues(item interface{}) int {
	panic("unimplemented")
}

var _ workqueue.RateLimitingInterface = &TestQueue{}
