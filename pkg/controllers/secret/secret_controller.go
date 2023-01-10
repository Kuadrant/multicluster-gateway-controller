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

package secret

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/multiClusterWatch"
)

// SecretReconciler reconciles a Secret object
type SecretReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	MCWatch multiClusterWatch.Interface
}

const (
	ARGO_CLUSTER_LABEL       = "argocd.argoproj.io/secret-type"
	ARGO_CLUSTER_LABEL_VALUE = "cluster"
)

//+kubebuilder:rbac:groups=,resources=secret,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=,resources=secret/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=,resources=secret/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Secret object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *SecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	previous := &corev1.Secret{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		return ctrl.Result{}, err
	}
	secret := previous.DeepCopy()
	restConfig, err := multiClusterWatch.RestConfigFromArgoSecret(secret)
	if err != nil {
		return ctrl.Result{}, err
	}

	_, err = r.MCWatch.WatchCluster(restConfig)

	if err != nil {
		log.Log.Info("error occurred", "error", err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return metadata.HasLabel(e.Object, ARGO_CLUSTER_LABEL) && e.Object.GetLabels()[ARGO_CLUSTER_LABEL] == ARGO_CLUSTER_LABEL_VALUE
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return metadata.HasLabel(e.Object, ARGO_CLUSTER_LABEL) && e.Object.GetLabels()[ARGO_CLUSTER_LABEL] == ARGO_CLUSTER_LABEL_VALUE
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return metadata.HasLabel(e.ObjectNew, ARGO_CLUSTER_LABEL) && e.ObjectNew.GetLabels()[ARGO_CLUSTER_LABEL] == ARGO_CLUSTER_LABEL_VALUE
			},
		}).
		Complete(r)
}
