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

package placement

import (
	"context"
	"reflect"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PlacementReconciler reconciles a Placement object
type PlacementReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=placements,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=placements/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=placements/finalizers,verbs=update

// Reconciles Placements by doing a few things:
// - find any matching clusters, given the 'predicates' set in the spec
// - update the status with these cluster 'decisions'
// - update the targetRef resource to set syncer annotations for each cluster decision
//
// Any change to a Placement triggers a reconcile
// Also, any change to a cluster secret (create/update/delete) will trigger a reconcile of *all* Placements
func (r *PlacementReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	previous := &v1alpha1.Placement{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Unable to fetch Placement")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if previous.GetDeletionTimestamp() != nil && !previous.GetDeletionTimestamp().IsZero() {
		log.Info("Placement is deleting", "placement", previous.Name, "namespace", previous.Namespace)
		return ctrl.Result{}, nil
	}

	predicates := previous.Spec.Predicates
	selectedClusters := []corev1.Secret{}
	for _, predicate := range predicates {
		clusterList := &corev1.SecretList{}
		// filter list by predicate label selector
		selector, err := metav1.LabelSelectorAsSelector(&predicate.RequiredClusterSelector.LabelSelector)
		if err != nil {
			log.Error(err, "Unable to convert label selector")
			return ctrl.Result{}, err
		}
		// only consider cluster secrets
		listOptions := client.MatchingLabels{
			"argocd.argoproj.io/secret-type": "cluster",
		}

		err = r.Client.List(ctx, clusterList, listOptions, client.MatchingLabelsSelector{Selector: selector})
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Unable to fetch cluster Secrets")
			return ctrl.Result{}, err
		}
		selectedClusters = append(selectedClusters, clusterList.Items...)
	}

	// Update placement decisions
	placement := previous.DeepCopy()
	placement.Status.Decisions = []v1alpha1.ClusterDecision{}
	for _, cluster := range selectedClusters {
		placement.Status.Decisions = append(placement.Status.Decisions, v1alpha1.ClusterDecision{ClusterName: cluster.Name})
	}
	placement.Status.NumberOfSelectedClusters = int32(len(placement.Status.Decisions))

	// Update status
	if !reflect.DeepEqual(placement.Status, previous.Status) {
		log.Info("Updating Placement status", "placementstatus", placement.Status, "previousstatus", previous.Status)
		err = r.Status().Update(ctx, placement)
		if err != nil {
			log.Error(err, "Error updating Placement status")
		}
	}

	// TODO: status conditions

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PlacementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Placement{}).
		Complete(r)
}
