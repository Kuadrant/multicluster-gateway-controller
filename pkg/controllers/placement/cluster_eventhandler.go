package placement

import (
	"context"
	"fmt"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterEventHandler struct {
	client client.Client
}

var _ handler.EventHandler = &ClusterEventHandler{}

// Create implements handler.EventHandler
func (eh *ClusterEventHandler) Create(e event.CreateEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.Object, q)
}

// Delete implements handler.EventHandler
func (eh *ClusterEventHandler) Delete(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.Object, q)
}

// Generic implements handler.EventHandler
func (eh *ClusterEventHandler) Generic(e event.GenericEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.Object, q)
}

// Update implements handler.EventHandler
func (eh *ClusterEventHandler) Update(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	eh.enqueueForObject(e.ObjectNew, q)
}

func (eh *ClusterEventHandler) enqueueForObject(obj v1.Object, q workqueue.RateLimitingInterface) {
	if !clusterSecret.IsClusterSecret(obj) {
		return
	}

	placements, err := eh.getPlacementsFor(obj.(*corev1.Secret))
	if err != nil {
		log.Log.Error(err, "failed to get placements when enqueueing from cluster secret")
		return
	}

	for _, placement := range placements {
		log.Log.Info(fmt.Sprintf("Enqueing reconciliation from secret update to placement/%s", placement.Name))
		q.Add(ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(&placement),
		})
	}
}

func (eh *ClusterEventHandler) getPlacementsFor(secret *corev1.Secret) ([]v1alpha1.Placement, error) {
	// Return all placements as we don't know which ones are affected by cluster secrets until they are individually reconciled
	placements := &v1alpha1.PlacementList{}
	err := eh.client.List(context.TODO(), placements)
	return placements.Items, err
}
