/*
Copyright 2023 The MultiCluster Traffic Controller Authors.

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

package dnspolicy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	gatewayapiv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	DNSPolicyFinalizer           = "kuadrant.io/dns-policy"
	DNSPoliciesBackRefAnnotation = "kuadrant.io/dnspolicies"
	DNSPolicyBackRefAnnotation   = "kuadrant.io/dnspolicy"
)

type DNSPolicyRefsConfig struct{}

func (c *DNSPolicyRefsConfig) PolicyRefsAnnotation() string {
	return DNSPoliciesBackRefAnnotation
}

// DNSPolicyReconciler reconciles a DNSPolicy object
type DNSPolicyReconciler struct {
	reconcilers.TargetRefReconciler
	DNSProvider dns.Provider
	HostService gateway.HostService
	Placement   gateway.GatewayPlacer
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies/finalizers,verbs=update

func (r *DNSPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger().WithValues("DNSPolicy", req.NamespacedName)
	log.Info("Reconciling DNSPolicy")
	ctx = crlog.IntoContext(ctx, log)

	previous := &v1alpha1.DNSPolicy{}
	if err := r.Client().Get(ctx, req.NamespacedName, previous); err != nil {
		if err := client.IgnoreNotFound(err); err == nil {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	dnsPolicy := previous.DeepCopy()
	log.V(3).Info("DNSPolicyReconciler Reconcile", "dnsPolicy", dnsPolicy)

	markedForDeletion := dnsPolicy.GetDeletionTimestamp() != nil

	targetNetworkObject, err := r.FetchValidTargetRef(ctx, dnsPolicy.GetTargetRef(), dnsPolicy.Namespace)
	if err != nil {
		if !markedForDeletion {
			if apierrors.IsNotFound(err) {
				log.V(3).Info("Network object not found. Cleaning up")
				err := r.deleteResources(ctx, dnsPolicy, nil)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
			return ctrl.Result{}, err
		}
		targetNetworkObject = nil // we need the object set to nil when there's an error, otherwise deleting the resources (when marked for deletion) will panic
	}

	if markedForDeletion {
		if controllerutil.ContainsFinalizer(dnsPolicy, DNSPolicyFinalizer) {
			if err := r.deleteResources(ctx, dnsPolicy, targetNetworkObject); err != nil {
				return ctrl.Result{}, err
			}
			if err := r.RemoveFinalizer(ctx, dnsPolicy, DNSPolicyFinalizer); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// add finalizer to the dnsPolicy
	if !controllerutil.ContainsFinalizer(dnsPolicy, DNSPolicyFinalizer) {
		if err := r.AddFinalizer(ctx, dnsPolicy, DNSPolicyFinalizer); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		}
	}

	specErr := r.reconcileResources(ctx, dnsPolicy, targetNetworkObject)

	newStatus := r.calculateStatus(dnsPolicy, specErr)
	dnsPolicy.Status = *newStatus

	if !equality.Semantic.DeepEqual(previous.Status, dnsPolicy.Status) {
		updateErr := r.Client().Status().Update(ctx, dnsPolicy)
		if updateErr != nil {
			// Ignore conflicts, resource might just be outdated.
			if apierrors.IsConflict(updateErr) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, updateErr
		}
	}

	if specErr != nil {
		return ctrl.Result{}, specErr
	}

	return ctrl.Result{}, nil
}

func (r *DNSPolicyReconciler) reconcileResources(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, targetNetworkObject client.Object) error {
	// validate
	err := dnsPolicy.Validate()
	if err != nil {
		return err
	}

	// reconcile based on gateway diffs
	gatewayDiffObj, err := r.ComputeGatewayDiffs(ctx, dnsPolicy, targetNetworkObject, &DNSPolicyRefsConfig{})
	if err != nil {
		return err
	}

	if err := r.reconcileDNSRecords(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		return err
	}

	if err := r.reconcileHealthChecks(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		return err
	}

	// set direct back ref - i.e. claim the target network object as taken asap
	if err := r.ReconcileTargetBackReference(ctx, client.ObjectKeyFromObject(dnsPolicy), targetNetworkObject, DNSPolicyBackRefAnnotation); err != nil {
		return err
	}

	// set annotation of policies affecting the gateway - should be the last step, only when all the reconciliation steps succeed
	return r.ReconcileGatewayPolicyReferences(ctx, dnsPolicy, gatewayDiffObj)
}

func (r *DNSPolicyReconciler) deleteResources(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, targetNetworkObject client.Object) error {
	// delete based on gateway diffs

	gatewayDiffObj, err := r.ComputeGatewayDiffs(ctx, dnsPolicy, targetNetworkObject, &DNSPolicyRefsConfig{})
	if err != nil {
		return err
	}

	if err := r.reconcileDNSRecords(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		return err
	}

	if err := r.reconcileHealthChecks(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		return err
	}

	// remove direct back ref
	if targetNetworkObject != nil {
		if err := r.DeleteTargetBackReference(ctx, client.ObjectKeyFromObject(dnsPolicy), targetNetworkObject, DNSPolicyBackRefAnnotation); err != nil {
			return err
		}
	}

	// update annotation of policies affecting the gateway
	return r.ReconcileGatewayPolicyReferences(ctx, dnsPolicy, gatewayDiffObj)
}

func (r *DNSPolicyReconciler) calculateStatus(dnsPolicy *v1alpha1.DNSPolicy, specErr error) *v1alpha1.DNSPolicyStatus {
	newStatus := dnsPolicy.Status.DeepCopy()
	if specErr != nil {
		newStatus.ObservedGeneration = dnsPolicy.Generation
	}
	readyCond := r.readyCondition(string(dnsPolicy.Spec.TargetRef.Kind), specErr)
	meta.SetStatusCondition(&newStatus.Conditions, *readyCond)
	return newStatus
}

func (r *DNSPolicyReconciler) readyCondition(targetNetworkObjectectKind string, specErr error) *metav1.Condition {
	cond := &metav1.Condition{
		Type:    conditions.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  fmt.Sprintf("%sDNSEnabled", targetNetworkObjectectKind),
		Message: fmt.Sprintf("%s is DNS Enabled", targetNetworkObjectectKind),
	}

	if specErr != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "ReconciliationError"
		cond.Message = specErr.Error()
	}

	return cond
}

func (r *DNSPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gatewayEventMapper := &GatewayEventMapper{
		Logger: r.Logger().WithName("gatewayEventMapper"),
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DNSPolicy{}).
		Watches(
			&source.Kind{Type: &gatewayapiv1beta1.Gateway{}},
			handler.EnqueueRequestsFromMapFunc(gatewayEventMapper.MapToDNSPolicy),
		).
		Complete(r)
}
