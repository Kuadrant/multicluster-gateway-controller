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
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	kuadrantapi "github.com/kuadrant/kuadrant-operator/api/v1beta1"
	kuadrantcommon "github.com/kuadrant/kuadrant-operator/pkg/common"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/clusterSecret"
	syncutils "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/sync"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/syncer"
)

const (
	RateLimitPolicyFinalizer = "kuadrant.io/rate-limit-policy"
)

// RateLimitPolicyReconciler reconciles a RateLimitPolicy object
type RateLimitPolicyReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ClusterSecrets *clusterSecret.Service
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=ratelimitpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=ratelimitpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=ratelimitpolicies/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update
//+kubebuilder:rbac:groups="cert-manager.io",resources=certificates,verbs=get;list;watch;create;update;patch;delete

func (r *RateLimitPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	previous := &kuadrantapi.RateLimitPolicy{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err == nil {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	rateLimitPolicy := previous.DeepCopy()

	log.Log.V(3).Info("RateLimitPolicyReconciler Reconcile", "rateLimitPolicy", rateLimitPolicy)

	if rateLimitPolicy.DeletionTimestamp != nil && !rateLimitPolicy.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(rateLimitPolicy, RateLimitPolicyFinalizer)

		err = r.Update(ctx, rateLimitPolicy)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	targetGateway, err := r.getTargetGateway(ctx, rateLimitPolicy)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = controllerutil.SetControllerReference(targetGateway, rateLimitPolicy, r.Scheme)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(rateLimitPolicy, RateLimitPolicyFinalizer) {
		log.Log.Info("RateLimitPolicyFinalizer Reconcile: Added finalizer", "rateLimitPolicy", rateLimitPolicy.Name)

		controllerutil.AddFinalizer(rateLimitPolicy, RateLimitPolicyFinalizer)

		err = r.Update(ctx, rateLimitPolicy)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	clusters, err := r.ClusterSecrets.GetClusterSecretsFromAnnotations(ctx, targetGateway)
	if err != nil {
		return ctrl.Result{}, err
	}

	setSyncAnnotationsForClusters(rateLimitPolicy, clusters)
	setPatchAnnotationsForClusters(rateLimitPolicy, clusters)

	if !reflect.DeepEqual(rateLimitPolicy, previous) {
		log.Log.V(3).Info("Updating RateLimitPolicy Spec", "rateLimitPolicySpec", rateLimitPolicy.Spec, "previousSpec", previous.Spec)
		err = r.Update(ctx, rateLimitPolicy)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RateLimitPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kuadrantapi.RateLimitPolicy{}).
		Watches(&source.Kind{
			Type: &corev1.Secret{},
		}, &ClusterEventHandler{client: r.Client}).
		Complete(r)
}

// getTargetGateway returns the Gateway resource that the given RateLimitPolicy is targeting
func (r *RateLimitPolicyReconciler) getTargetGateway(ctx context.Context, rateLimitPolicy *kuadrantapi.RateLimitPolicy) (*gatewayv1beta1.Gateway, error) {
	if !kuadrantcommon.IsTargetRefGateway(rateLimitPolicy.Spec.TargetRef) {
		return nil, fmt.Errorf("unsupported target ref: %v", rateLimitPolicy.Spec.TargetRef)
	}

	targetNS := rateLimitPolicy.Namespace
	if rateLimitPolicy.Spec.TargetRef.Namespace != nil {
		targetNS = string(*rateLimitPolicy.Spec.TargetRef.Namespace)
	}

	targetGateway := &gatewayv1beta1.Gateway{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: targetNS, Name: string(rateLimitPolicy.Spec.TargetRef.Name)}, targetGateway)
	if err != nil {
		return targetGateway, err
	}
	return targetGateway, nil
}

// setSyncAnnotationsForClusters adds sync annotations to the given resource for each cluster.
// Note: A sync annotation is added for each individual cluster even if the wildcard `all` annotation was used on the target Gateway.
// We need to add patch annotations for each cluster individually, so for consistency sync annotations are the same.
func setSyncAnnotationsForClusters(obj metav1.Object, clusters []clusterSecret.ClusterSecret) {
	annotations := map[string]string{}
	// keep all non sync related annotations
	for annKey, annValue := range obj.GetAnnotations() {
		if !strings.HasPrefix(annKey, syncer.MCTC_SYNC_ANNOTATION_PREFIX) {
			annotations[annKey] = annValue
		}
	}
	// add all sync annotations
	for _, cluster := range clusters {
		annotations[syncer.MCTC_SYNC_ANNOTATION_PREFIX+cluster.Config.Name] = "true"
	}

	obj.SetAnnotations(annotations)
}

// setPatchAnnotationsForClusters adds patch annotations to the given RateLimitPolicy for each cluster.
// Adds a patch annotation to the RLP that injects an Action with a generic key (key: "kuadrant_gateway_cluster", value: <cluster name>) which
// can then be used in limit conditions.
// Also adds an Action with generic key for any attributes added to the cluster using the MCTC_ATTRIBUTE_ANNOTATION_PREFIX
// i.e. kuadrant.io/attribute-cloud=aws = (key: "cloud", value: "aws").
func setPatchAnnotationsForClusters(rlp *kuadrantapi.RateLimitPolicy, clusters []clusterSecret.ClusterSecret) {
	// add all patch annotations
	for _, cluster := range clusters {
		clusterAttrs := cluster.GetAttributes()
		clusterAttrs["kuadrant_gateway_cluster"] = cluster.Config.Name

		actions := []kuadrantapi.ActionSpecifier{}
		for key, value := range clusterAttrs {
			k := key
			action := &kuadrantapi.ActionSpecifier{
				GenericKey: &kuadrantapi.GenericKeySpec{
					DescriptorKey:   &k,
					DescriptorValue: value,
				}}
			actions = append(actions, *action)
		}

		rl := &kuadrantapi.RateLimit{
			Configurations: []kuadrantapi.Configuration{
				{Actions: actions},
			},
		}

		err := syncutils.SetPatchAnnotation(func(rlp *kuadrantapi.RateLimitPolicy) {
			rlp.Spec.RateLimits = append(rlp.Spec.RateLimits, *rl)
		}, cluster.Config.Name, rlp)
		if err != nil {
			log.Log.Error(err, "Unable to add patch annotation for cluster")
		}
	}
}
