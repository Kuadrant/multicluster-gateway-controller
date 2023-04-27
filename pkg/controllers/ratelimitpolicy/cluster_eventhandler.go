/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ratelimitpolicy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kuadrantapi "github.com/kuadrant/kuadrant-operator/api/v1beta1"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer"
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

	rlps, err := eh.getRateLimitPoliciesFor(obj.(*corev1.Secret))
	if err != nil {
		log.Log.Error(err, "failed to get rlps when enqueueing from cluster secret")
		return
	}

	for _, rlp := range rlps {
		log.Log.Info(fmt.Sprintf("Enqueing reconciliation from secret update to ratelimitpolicy/%s", rlp.Name))
		q.Add(ctrl.Request{
			NamespacedName: client.ObjectKeyFromObject(&rlp),
		})
	}
}

func (eh *ClusterEventHandler) getRateLimitPoliciesFor(secret *corev1.Secret) ([]kuadrantapi.RateLimitPolicy, error) {
	clusterConfig, err := clusterSecret.ClusterConfigFromSecret(secret)
	if err != nil {
		return nil, err
	}

	rlps := &kuadrantapi.RateLimitPolicyList{}
	if err := eh.client.List(context.TODO(), rlps); err != nil {
		return nil, err
	}

	return slice.Filter(rlps.Items, func(rlp kuadrantapi.RateLimitPolicy) bool {
		return metadata.HasAnnotation(&rlp, syncer.MCTC_SYNC_ANNOTATION_PREFIX+syncer.MCTC_SYNC_ANNOTATION_WILDCARD) ||
			metadata.HasAnnotation(&rlp, syncer.MCTC_SYNC_ANNOTATION_PREFIX+clusterConfig.Name)
	}), nil
}
