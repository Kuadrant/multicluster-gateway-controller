/*
Copyright 2023.

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

package tlspolicy

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
)

// TLSPolicyReconciler reconciles a TLSPolicy object
type TLSPolicyReconciler struct {
	reconcilers.TargetRefReconciler
	Placement gateway.GatewayPlacer
	Scheme    *runtime.Scheme
}

//+kubebuilder:rbac:groups=kuadrant.io.kuadrant.io,resources=tlspolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io.kuadrant.io,resources=tlspolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io.kuadrant.io,resources=tlspolicies/finalizers,verbs=update

func (r *TLSPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger().WithValues("TLSPolicy", req.NamespacedName)
	log.Info("Reconciling TLSPolicy")
	ctx = crlog.IntoContext(ctx, log)

	previous := &v1alpha1.TLSPolicy{}
	if err := r.Client().Get(ctx, req.NamespacedName, previous); err != nil {
		if err := client.IgnoreNotFound(err); err == nil {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TLSPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.TLSPolicy{}).
		Complete(r)
}
