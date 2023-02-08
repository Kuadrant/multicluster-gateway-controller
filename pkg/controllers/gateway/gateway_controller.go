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

package gateway

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	ClusterSyncerAnnotation               = "clustersync.kuadrant.io"
	GatewayClusterLabelSelectorAnnotation = "kuadrant.io/gateway-cluster-label-selector"
)

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Gateway object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	previous := &gatewayv1beta1.Gateway{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Unable to fetch Gateway")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if the class name is one of ours
	gatewayClass := &gatewayv1beta1.GatewayClass{}
	err = r.Client.Get(ctx, client.ObjectKey{Namespace: previous.Namespace, Name: string(previous.Spec.GatewayClassName)}, gatewayClass)
	if err != nil {
		if err := client.IgnoreNotFound(err); err != nil {
			log.Error(err, "Unable to fetch GatewayClass")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if !slice.ContainsString(getSupportedClasses(), string(previous.Spec.GatewayClassName)) {
		// ignore as it may be for a different gateway controller
		log.Info("Not a Gateway for this controller", "GatewayClassName", previous.Spec.GatewayClassName)
		return ctrl.Result{}, nil
	}

	gateway := previous.DeepCopy()
	statusConditions := []metav1.Condition{}
	statusConditions = append(statusConditions, metav1.Condition{
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Handled by %s", ControllerName),
		Reason:             string(gatewayv1beta1.GatewayConditionAccepted),
		Status:             metav1.ConditionTrue,
		Type:               string(gatewayv1beta1.GatewayConditionAccepted),
		ObservedGeneration: previous.Generation,
	})

	clusters := selectClusters(*gateway)
	if len(clusters) == 0 {
		statusConditions = append(statusConditions, metav1.Condition{
			LastTransitionTime: metav1.Now(),
			Message:            "No clusters match selection",
			Reason:             string(gatewayv1beta1.GatewayReasonPending),
			Status:             metav1.ConditionFalse,
			Type:               string(gatewayv1beta1.GatewayConditionProgrammed),
			ObservedGeneration: previous.Generation,
		})
	} else {
		applyClusterSyncerAnnotations(gateway, clusters)
		statusConditions = append(statusConditions, metav1.Condition{
			LastTransitionTime: metav1.Now(),
			Message:            fmt.Sprintf("Gateways configured in data plane clusters - [%v]", strings.Join(clusters, ",")),
			Reason:             string(gatewayv1beta1.GatewayConditionProgrammed),
			Status:             metav1.ConditionTrue,
			Type:               string(gatewayv1beta1.GatewayConditionProgrammed),
			ObservedGeneration: previous.Generation,
		})
	}

	if !reflect.DeepEqual(gateway.Annotations, previous.Annotations) {
		log.Info("Updating Gateway annotations", "gateway", gateway)
		err = r.Update(ctx, gateway)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	gateway.Status.Conditions = statusConditions
	if !reflect.DeepEqual(gateway.Status, previous.Status) {
		log.Info("Updating Gateway status", "gatewayStatus", gateway.Status)
		err = r.Status().Update(ctx, gateway)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func applyClusterSyncerAnnotations(gateway *gatewayv1beta1.Gateway, clusters []string) {
	for _, cluster := range clusters {
		gateway.Annotations[fmt.Sprintf("%s/%s", ClusterSyncerAnnotation, cluster)] = "True"
	}
}

func findConditionByType(conditions []metav1.Condition, conditionType gatewayv1beta1.GatewayConditionType) *metav1.Condition {
	for _, condition := range conditions {
		if condition.Type == string(conditionType) {
			return &condition
		}
	}
	return nil
}

func selectClusters(gateway gatewayv1beta1.Gateway) []string {
	if gateway.Annotations == nil {
		return []string{}
	}

	selector := gateway.Annotations[GatewayClusterLabelSelectorAnnotation]
	log.Log.Info("selectClusters", "selector", selector)

	// TODO: Lookup clusters and select based on gateway cluster label selector annotation
	// HARDCODED IMPLEMENTATION
	// Issue: https://github.com/Kuadrant/multi-cluster-traffic-controller/issues/52
	if selector == "type=test" {
		return []string{"test_cluster_one"}
	}
	return []string{}
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1beta1.Gateway{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			gateway := object.(*gatewayv1beta1.Gateway)
			return slice.ContainsString(getSupportedClasses(), string(gateway.Spec.GatewayClassName))
		})).
		Complete(r)
}
