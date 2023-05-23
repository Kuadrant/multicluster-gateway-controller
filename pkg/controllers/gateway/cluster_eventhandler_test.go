package gateway

import (
	"testing"
	"time"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

func TestClusterEventHandler(t *testing.T) {
	testHostName := gatewayv1beta1.Hostname("test.listener.com")
	testNS := gatewayv1beta1.Namespace("test-ns")
	cases := []struct {
		name string

		gateways         []gatewayv1beta1.Gateway
		secret           corev1.Secret
		enqueuedGateways []gatewayv1beta1.Gateway
	}{
		{
			name: "Queued one",
			gateways: []gatewayv1beta1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gateway",
						Namespace: "test-ns",
						Annotations: map[string]string{
							GatewayClusterLabelSelectorAnnotation: "type=test",
						},
					},
					Spec: gatewayv1beta1.GatewaySpec{
						Listeners: []gatewayv1beta1.Listener{
							{
								Hostname: &testHostName,
								Protocol: gatewayv1beta1.HTTPSProtocolType,
								TLS: &gatewayv1beta1.GatewayTLSConfig{
									CertificateRefs: []gatewayv1beta1.SecretObjectReference{
										{
											Name:      gatewayv1beta1.ObjectName("test.listener.com"),
											Namespace: &testNS,
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
						Namespace: "test-ns",
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
					Name:      "test.listener.com",
					Namespace: "test-ns",
				},
				Data: map[string][]byte{
					"name": []byte("test_cluster_one"),
					"config": []byte(
						`
				{
					"tlsClientConfig":
					  {
					    "insecure": false,
					    "caData": "test",
					    "certData": "test",
					    "keyData": "test"
					  }
				}
				`),
				},
			},
			enqueuedGateways: []gatewayv1beta1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gateway",
						Namespace: "test-ns",
					},
				},
			},
		},
		{
			name: "Not enqueued. Not a cluster secret",
			gateways: []gatewayv1beta1.Gateway{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-gateway",
						Namespace: "test-ns",
						Annotations: map[string]string{
							GatewayClusterLabelSelectorAnnotation: "type=test",
						},
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated-secret",
					Namespace: "test-ns",
				},
			},
			enqueuedGateways: make([]gatewayv1beta1.Gateway, 0),
		},
	}

	scheme := runtime.NewScheme()
	if err := gatewayv1beta1.AddToScheme(scheme); err != nil {
		t.Fatal("unexpected error building scheme", err)
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithLists(
				&gatewayv1beta1.GatewayList{
					Items: testCase.gateways,
				},
			).Build()

			testQ := &testQueue{t: t}
			clusterEventHandler := &ClusterEventHandler{
				client: client,
			}

			clusterEventHandler.enqueueForObject(&testCase.secret, testQ)
			testQ.MustHaveEnqueued(testCase.enqueuedGateways)
		})
	}
}

type testQueue struct {
	t                *testing.T
	enqueuedRequests map[types.NamespacedName]bool
}

func (q *testQueue) Add(item interface{}) {
	req, ok := item.(ctrl.Request)
	if !ok {
		q.t.Fatalf("expected enqueued item to be of type ctrl.Request, got %v", item)
	}

	if q.enqueuedRequests == nil {
		q.enqueuedRequests = make(map[types.NamespacedName]bool)
	}

	q.enqueuedRequests[req.NamespacedName] = true
}

func (q *testQueue) MustHaveEnqueued(gateways []gatewayv1beta1.Gateway) {
	enqueuedCopy := map[types.NamespacedName]bool{}
	for obj := range q.enqueuedRequests {
		enqueuedCopy[obj] = true
	}

	for _, gateway := range gateways {
		var nsn types.NamespacedName = types.NamespacedName{
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

// Unused methods below that must be defined for testQueue to implement the
// workqueue.RateLimitingInterface:
//
// Done implements workqueue.RateLimitingInterface
func (*testQueue) Done(item interface{}) {
	panic("unimplemented")
}

// Get implements workqueue.RateLimitingInterface
func (*testQueue) Get() (item interface{}, shutdown bool) {
	panic("unimplemented")
}

// Len implements workqueue.RateLimitingInterface
func (*testQueue) Len() int {
	panic("unimplemented")
}

// ShutDown implements workqueue.RateLimitingInterface
func (*testQueue) ShutDown() {
	panic("unimplemented")
}

// ShutDownWithDrain implements workqueue.RateLimitingInterface
func (*testQueue) ShutDownWithDrain() {
	panic("unimplemented")
}

// ShuttingDown implements workqueue.RateLimitingInterface
func (*testQueue) ShuttingDown() bool {
	panic("unimplemented")
}

// AddAfter implements workqueue.RateLimitingInterface
func (*testQueue) AddAfter(item interface{}, duration time.Duration) {
	panic("unimplemented")
}

// AddRateLimited implements workqueue.RateLimitingInterface
func (*testQueue) AddRateLimited(item interface{}) {
	panic("unimplemented")
}

// Forget implements workqueue.RateLimitingInterface
func (*testQueue) Forget(item interface{}) {
	panic("unimplemented")
}

// NumRequeues implements workqueue.RateLimitingInterface
func (*testQueue) NumRequeues(item interface{}) int {
	panic("unimplemented")
}

var _ workqueue.RateLimitingInterface = &testQueue{}
