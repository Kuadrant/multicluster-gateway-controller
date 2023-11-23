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
	"errors"
	"fmt"
	"reflect"

	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/events"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	DNSPolicyFinalizer                                    = "kuadrant.io/dns-policy"
	DNSPoliciesBackRefAnnotation                          = "kuadrant.io/dnspolicies"
	DNSPolicyBackRefAnnotation                            = "kuadrant.io/dnspolicy"
	DNSPolicyAffected            conditions.ConditionType = "kuadrant.io/DNSPolicyAffected"
)

type DNSPolicyRefsConfig struct{}

func (c *DNSPolicyRefsConfig) PolicyRefsAnnotation() string {
	return DNSPoliciesBackRefAnnotation
}

// DNSPolicyReconciler reconciles a DNSPolicy object
type DNSPolicyReconciler struct {
	reconcilers.TargetRefReconciler
	DNSProvider dns.DNSProviderFactory
	dnsHelper   dnsHelper
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies,verbs=get;list;watch;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.open-cluster-management.io,resources=managedclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/finalizers,verbs=update

func (r *DNSPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger().WithValues("DNSPolicy", req.NamespacedName)
	log.Info("Reconciling DNSPolicy")
	ctx = crlog.IntoContext(ctx, log)

	previous := &v1alpha1.DNSPolicy{}
	if err := r.Client().Get(ctx, req.NamespacedName, previous); err != nil {
		log.Info("error getting dns policy", "error", err)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	dnsPolicy := previous.DeepCopy()
	log.V(3).Info("DNSPolicyReconciler Reconcile", "dnsPolicy", dnsPolicy)

	// add finalizer to the dnsPolicy
	if !controllerutil.ContainsFinalizer(dnsPolicy, DNSPolicyFinalizer) {
		if err := r.AddFinalizer(ctx, dnsPolicy, DNSPolicyFinalizer); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{Requeue: true}, err
		} else if apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	markedForDeletion := dnsPolicy.GetDeletionTimestamp() != nil

	targetNetworkObject, err := r.FetchValidTargetRef(ctx, dnsPolicy.GetTargetRef(), dnsPolicy.Namespace)

	if err != nil {
		targetNetworkObject = nil
	}
	if markedForDeletion {
		log.V(3).Info("cleaning up dns policy")
		if controllerutil.ContainsFinalizer(dnsPolicy, DNSPolicyFinalizer) {
			if err = r.deleteResources(ctx, dnsPolicy, targetNetworkObject); err != nil {
				return ctrl.Result{}, err
			}
			if err = r.RemoveFinalizer(ctx, dnsPolicy, DNSPolicyFinalizer); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	validateErr, acceptErr := r.validate(ctx, dnsPolicy)

	statusResult, statusErr := r.reconcileStatus(ctx, dnsPolicy, acceptErr)

	if validateErr != nil || targetNetworkObject == nil {
		return ctrl.Result{Requeue: true}, validateErr
	}
	if statusErr != nil {
		return statusResult, statusErr
	}

	enforceErr := r.enforce(ctx, dnsPolicy, targetNetworkObject)

	statusResult, statusErr = r.reconcileStatus(ctx, dnsPolicy, enforceErr)

	if enforceErr != nil {
		return ctrl.Result{Requeue: true}, enforceErr
	}
	return statusResult, statusErr
}

func (r *DNSPolicyReconciler) validate(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy) (error, error) {
	_, targetErr := r.FetchValidTargetRef(ctx, dnsPolicy.GetTargetRef(), dnsPolicy.Namespace)

	if targetErr != nil {
		if !apierrors.IsNotFound(targetErr) {
			return nil, targetErr
		}
		r.Logger().V(3).Info("Network object not found. Cleaning up")
		err := r.deleteResources(ctx, dnsPolicy, nil)
		return err, conditions.ErrTargetNotFound
	}
	return nil, nil

}

func (r *DNSPolicyReconciler) enforce(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, targetNetworkObject client.Object) error {
	gatewayCondition := conditions.BuildPolicyAffectedCondition(DNSPolicyAffected, dnsPolicy, targetNetworkObject, conditions.PolicyReasonAccepted, nil)
	// validate
	err := dnsPolicy.Validate()
	if err != nil {
		return err
	}

	dnsPolicy.Default()

	// reconcile based on gateway diffs
	gatewayDiffObj, err := r.ComputeGatewayDiffs(ctx, dnsPolicy, targetNetworkObject, &DNSPolicyRefsConfig{})
	if err != nil {
		return err
	}

	if err = r.reconcileDNSRecords(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		gatewayCondition = conditions.BuildPolicyAffectedCondition(DNSPolicyAffected, dnsPolicy, targetNetworkObject, conditions.PolicyReasonInvalid, err)
		updateErr := r.updateGatewayCondition(ctx, gatewayCondition, gatewayDiffObj)
		return errors.Join(fmt.Errorf("reconcile DNSRecords error %w", err), updateErr)
	}

	if err = r.reconcileHealthCheckProbes(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		gatewayCondition = conditions.BuildPolicyAffectedCondition(DNSPolicyAffected, dnsPolicy, targetNetworkObject, conditions.PolicyReasonInvalid, err)
		updateErr := r.updateGatewayCondition(ctx, gatewayCondition, gatewayDiffObj)
		return errors.Join(fmt.Errorf("reconcile HealthChecks error %w", err), updateErr)
	}

	// set direct back ref - i.e. claim the target network object as taken asap
	if err = r.ReconcileTargetBackReference(ctx, client.ObjectKeyFromObject(dnsPolicy), targetNetworkObject, DNSPolicyBackRefAnnotation); err != nil {
		gatewayCondition = conditions.BuildPolicyAffectedCondition(DNSPolicyAffected, dnsPolicy, targetNetworkObject, conditions.PolicyReasonConflicted, err)
		updateErr := r.updateGatewayCondition(ctx, gatewayCondition, gatewayDiffObj)
		return errors.Join(fmt.Errorf("reconcile TargetBackReference error %w", err), updateErr)
	}

	// set annotation of policies affecting the gateway
	if err := r.ReconcileGatewayPolicyReferences(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		gatewayCondition = conditions.BuildPolicyAffectedCondition(DNSPolicyAffected, dnsPolicy, targetNetworkObject, conditions.PolicyReasonUnknown, err)
		updateErr := r.updateGatewayCondition(ctx, gatewayCondition, gatewayDiffObj)
		return errors.Join(fmt.Errorf("ReconcileGatewayPolicyReferences error %w", err), updateErr)
	}

	// set gateway policy affected condition status - should be the last step, only when all the reconciliation steps succeed
	updateErr := r.updateGatewayCondition(ctx, gatewayCondition, gatewayDiffObj)
	if updateErr != nil {
		return fmt.Errorf("failed to update gateway conditions %w ", updateErr)
	}

	return nil
}

func (r *DNSPolicyReconciler) deleteResources(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, targetNetworkObject client.Object) error {
	// delete based on gateway diffs

	if err := r.deleteDNSRecords(ctx, dnsPolicy); err != nil {
		return err
	}

	if err := r.deleteHealthCheckProbes(ctx, dnsPolicy); err != nil {
		return err
	}

	// if the target ref object is nil don't try to delete any refs or update gateway references
	if targetNetworkObject == nil {
		return nil
	}
	// remove direct back ref
	if err := r.DeleteTargetBackReference(ctx, targetNetworkObject, DNSPolicyBackRefAnnotation); err != nil {
		return err
	}

	gatewayDiffObj, err := r.ComputeGatewayDiffs(ctx, dnsPolicy, targetNetworkObject, &DNSPolicyRefsConfig{})
	if err != nil {
		return err
	}

	// update annotation of policies affecting the gateway
	if err = r.ReconcileGatewayPolicyReferences(ctx, dnsPolicy, gatewayDiffObj); err != nil {
		return err
	}

	// remove gateway policy affected condition status
	return r.updateGatewayCondition(ctx, metav1.Condition{Type: string(DNSPolicyAffected)}, gatewayDiffObj)
}

func (r *DNSPolicyReconciler) reconcileStatus(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, statusErr error) (ctrl.Result, error) {
	newStatus, err := r.calculateStatus(ctx, dnsPolicy, statusErr)
	// TODO: Ensure whether the best approach is to fail the reconciliation
	// in case of an error here, or attempt to set the status with the
	// error
	if err != nil {
		return ctrl.Result{}, err
	}

	if !equality.Semantic.DeepEqual(newStatus, dnsPolicy.Status) {
		dnsPolicy.Status = *newStatus
		updateErr := r.Client().Status().Update(ctx, dnsPolicy)
		if updateErr != nil {
			// Ignore conflicts, resource might just be outdated.
			if apierrors.IsConflict(updateErr) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, updateErr
		}
	}
	return ctrl.Result{}, nil
}

func (r *DNSPolicyReconciler) calculateStatus(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, statusErr error) (*v1alpha1.DNSPolicyStatus, error) {
	newStatus := dnsPolicy.Status.DeepCopy()
	if statusErr != nil {
		newStatus.ObservedGeneration = dnsPolicy.Generation
		newStatus.Conditions = []metav1.Condition{}
	}

	acceptedCondition := dnsPolicy.BuildPolicyAcceptedCondition(statusErr)
	meta.SetStatusCondition(&newStatus.Conditions, *acceptedCondition)

	// Only calculate the Enforced status condition if Ready is True
	if !meta.IsStatusConditionTrue(newStatus.Conditions, string(conditions.ConditionTypeAccepted)) {
		return newStatus, nil
	}

	enforcedCond, err := r.enforcedCondition(ctx, dnsPolicy)
	if err != nil {
		return nil, err
	}

	meta.SetStatusCondition(&newStatus.Conditions, *enforcedCond)

	return newStatus, nil
}

func (r *DNSPolicyReconciler) enforcedCondition(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy) (*metav1.Condition, error) {
	dnsRecords, err := r.dnsHelper.getDNSRecordsForDNSPolicy(ctx, dnsPolicy)
	if err != nil {
		return nil, err
	}

	// If no records have been created at all, set the Enforced condition to
	// unknown
	if len(dnsRecords) == 0 {
		return &metav1.Condition{
			Type:    string(conditions.ConditionTypeEnforced),
			Status:  metav1.ConditionFalse,
			Message: "No DNSRecords created",
			Reason:  "Unknown",
		}, nil
	}

	notReadyRecords := slice.Filter(dnsRecords, slice.Not(v1alpha1.DNSRecord.IsReady))
	allRecordsReady := len(notReadyRecords) == 0

	if allRecordsReady {
		return &metav1.Condition{
			Type:    string(conditions.ConditionTypeEnforced),
			Status:  metav1.ConditionTrue,
			Message: "All DNSRecords ready",
			Reason:  "Enforced",
		}, nil
	}

	notReadyRecordNames := slice.Map(notReadyRecords, func(dnsRecord v1alpha1.DNSRecord) string {
		return fmt.Sprintf("%s/%s", dnsRecord.Namespace, dnsRecord.Namespace)
	})

	return &metav1.Condition{
		Type:    string(conditions.ConditionTypeEnforced),
		Status:  metav1.ConditionFalse,
		Message: fmt.Sprintf("DNSRecords %v not Ready", notReadyRecordNames),
		Reason:  "Unknown",
	}, nil
}

func (r *DNSPolicyReconciler) updateGatewayCondition(ctx context.Context, condition metav1.Condition, gatewayDiff *reconcilers.GatewayDiff) error {

	// update condition if needed
	for _, gw := range append(gatewayDiff.GatewaysWithValidPolicyRef, gatewayDiff.GatewaysMissingPolicyRef...) {
		previous := gw.DeepCopy()
		meta.SetStatusCondition(&gw.Status.Conditions, condition)
		if !reflect.DeepEqual(previous.Status.Conditions, gw.Status.Conditions) {
			if err := r.Client().Status().Update(ctx, gw.Gateway); err != nil {
				return err
			}
		}
	}

	// remove condition from gateway that is no longer referenced
	for _, gw := range gatewayDiff.GatewaysWithInvalidPolicyRef {
		previous := gw.DeepCopy()
		meta.RemoveStatusCondition(&gw.Status.Conditions, condition.Type)
		if !reflect.DeepEqual(previous.Status.Conditions, gw.Status.Conditions) {
			if err := r.Client().Status().Update(ctx, gw.Gateway); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *DNSPolicyReconciler) SetupWithManager(mgr ctrl.Manager, ocmHub bool) error {
	gatewayEventMapper := events.NewGatewayEventMapper(r.Logger(), &DNSPolicyRefsConfig{}, "dnspolicy")
	clusterEventMapper := events.NewClusterEventMapper(r.Logger(), r.Client(), &DNSPolicyRefsConfig{}, "dnspolicy")
	probeEventMapper := events.NewPolicyRefEventMapper(r.Logger(), DNSPolicyBackRefAnnotation, "dnspolicy")
	dnsRecordEventMapper := events.NewPolicyRefEventMapper(r.Logger(), DNSPolicyBackRefAnnotation, "dnspolicy")
	r.dnsHelper = dnsHelper{Client: r.Client()}
	ctrlr := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DNSPolicy{}).
		Watches(
			&gatewayapiv1.Gateway{},
			handler.EnqueueRequestsFromMapFunc(gatewayEventMapper.MapToPolicy),
		).
		Watches(
			&v1alpha1.DNSHealthCheckProbe{},
			handler.EnqueueRequestsFromMapFunc(probeEventMapper.MapToPolicy),
		).
		Watches(
			&v1alpha1.DNSRecord{},
			handler.EnqueueRequestsFromMapFunc(dnsRecordEventMapper.MapToPolicy),
		)
	if ocmHub {
		r.Logger().Info("ocm enabled turning on managed cluster watch")
		ctrlr.Watches(
			&clusterv1.ManagedCluster{},
			handler.EnqueueRequestsFromMapFunc(clusterEventMapper.MapToPolicy),
		)
	}
	return ctrlr.Complete(r)
}
