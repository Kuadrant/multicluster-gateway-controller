package placement

import (
	"testing"
	"time"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterEventHandler(t *testing.T) {
	cases := []struct {
		name string

		placements         []v1alpha1.Placement
		secret             corev1.Secret
		enqueuedPlacements []v1alpha1.Placement
	}{
		{
			name: "Queued two",
			placements: []v1alpha1.Placement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-placement",
						Namespace: "test-ns",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-another-placement",
						Namespace: "test-ns",
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterSecret.CLUSTER_SECRET_LABEL: clusterSecret.CLUSTER_SECRET_LABEL_VALUE,
					},
					Name:      "cluster",
					Namespace: "test-ns",
				},
				Data: map[string][]byte{
					"name": []byte("cluster"),
					"config": []byte(`
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
			enqueuedPlacements: []v1alpha1.Placement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-placement",
						Namespace: "test-ns",
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-another-placement",
						Namespace: "test-ns",
					},
				},
			},
		},
		{
			name: "Not enqueued. Not a cluster secret",
			placements: []v1alpha1.Placement{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-placement",
						Namespace: "test-ns",
					},
				},
			},
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated-secret",
					Namespace: "test-ns",
				},
			},
			enqueuedPlacements: make([]v1alpha1.Placement, 0),
		},
	}

	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal("unexpected error building scheme", err)
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithLists(
				&v1alpha1.PlacementList{
					Items: testCase.placements,
				},
			).Build()

			testQ := &testQueue{t: t}
			clusterEventHandler := &ClusterEventHandler{
				client: client,
			}

			clusterEventHandler.enqueueForObject(&testCase.secret, testQ)
			testQ.MustHaveEnqueued(testCase.enqueuedPlacements)
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

func (q *testQueue) MustHaveEnqueued(placements []v1alpha1.Placement) {
	enqueuedCopy := map[types.NamespacedName]bool{}
	for obj := range q.enqueuedRequests {
		enqueuedCopy[obj] = true
	}

	for _, placement := range placements {
		var nsn types.NamespacedName = types.NamespacedName{
			Namespace: placement.Namespace,
			Name:      placement.Name,
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
