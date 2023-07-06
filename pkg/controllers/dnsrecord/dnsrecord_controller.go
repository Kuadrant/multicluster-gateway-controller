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

package dnsrecord

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	DNSRecordFinalizer = "kuadrant.io/dns-record"
)

var Clock clock.Clock = clock.RealClock{}

// DNSRecordReconciler reconciles a DNSRecord object
type DNSRecordReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	DNSProvider dns.Provider
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnsrecords/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnsrecords/finalizers,verbs=update

func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	previous := &v1alpha1.DNSRecord{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, previous)
	if err != nil {
		if err := client.IgnoreNotFound(err); err == nil {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}
	dnsRecord := previous.DeepCopy()

	log.Log.V(3).Info("DNSRecordReconciler Reconcile", "dnsRecord", dnsRecord)

	if dnsRecord.DeletionTimestamp != nil && !dnsRecord.DeletionTimestamp.IsZero() {
		if err := r.deleteRecord(ctx, dnsRecord); err != nil {
			log.Log.Error(err, "Failed to delete DNSRecord", "record", dnsRecord)
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(dnsRecord, DNSRecordFinalizer)

		err = r.Update(ctx, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(dnsRecord, DNSRecordFinalizer) {
		controllerutil.AddFinalizer(dnsRecord, DNSRecordFinalizer)
		err = r.Update(ctx, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	var reason, message string
	status := metav1.ConditionTrue
	reason = "ProviderSuccess"
	message = "Provider ensured the managed zone"

	// Publish the record
	err = r.publishRecord(ctx, dnsRecord)
	if err != nil {
		status = metav1.ConditionFalse
		reason = "ProviderError"
		message = fmt.Sprintf("The DNS provider failed to ensure the record: %v", err)
	} else {
		dnsRecord.Status.ObservedGeneration = dnsRecord.Generation
		dnsRecord.Status.Endpoints = dnsRecord.Spec.Endpoints
	}
	setDNSRecordCondition(dnsRecord, conditions.ConditionTypeReady, status, reason, message)

	err = r.Status().Update(ctx, dnsRecord.DeepCopy())
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Update(ctx, dnsRecord.DeepCopy())
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DNSRecord{}).
		Complete(r)
}

// deleteRecord deletes record(s) in the DNSPRovider(i.e. route53) configured by the ManagedZone assigned to this
// DNSRecord (dnsRecord.Status.ParentManagedZone).
func (r *DNSRecordReconciler) deleteRecord(ctx context.Context, dnsRecord *v1alpha1.DNSRecord) error {
	managedZone, err := r.getDNSRecordManagedZone(ctx, dnsRecord)
	if err != nil {
		// If the Managed Zone isn't found, just continue
		return client.IgnoreNotFound(err)
	}

	err = r.DNSProvider.Delete(dnsRecord, managedZone)
	if err != nil {
		if strings.Contains(err.Error(), "was not found") {
			log.Log.Info("Record not found in managed zone, continuing", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)
			return nil
		} else if strings.Contains(err.Error(), "no endpoints") {
			log.Log.Info("DNS record had no endpoint, continuing", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)
			return nil
		}
		return err
	}
	log.Log.Info("Deleted DNSRecord in manage zone", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)

	return nil
}

// publishRecord publishes record(s) to the DNSPRovider(i.e. route53) configured by the ManagedZone assigned to this
// DNSRecord (dnsRecord.Status.ParentManagedZone).
func (r *DNSRecordReconciler) publishRecord(ctx context.Context, dnsRecord *v1alpha1.DNSRecord) error {

	managedZone, err := r.getDNSRecordManagedZone(ctx, dnsRecord)
	if err != nil {
		return err
	}

	if dnsRecord.Generation == dnsRecord.Status.ObservedGeneration {
		log.Log.V(3).Info("Skipping managed zone to which the DNS dnsRecord is already published", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)
		return nil
	}

	err = r.DNSProvider.Ensure(dnsRecord, managedZone)
	if err != nil {
		return err
	}
	log.Log.Info("Published DNSRecord to manage zone", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)

	return nil
}

// getDNSRecordManagedZone returns the current ManagedZone for the given DNSRecord.
func (r *DNSRecordReconciler) getDNSRecordManagedZone(ctx context.Context, dnsRecord *v1alpha1.DNSRecord) (*v1alpha1.ManagedZone, error) {

	if dnsRecord.Spec.ManagedZoneRef == nil {
		return nil, fmt.Errorf("no managed zone configured for : %s", dnsRecord.Name)
	}

	managedZone := &v1alpha1.ManagedZone{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: dnsRecord.Namespace, Name: dnsRecord.Spec.ManagedZoneRef.Name}, managedZone)
	if err != nil {
		return nil, err
	}

	managedZoneReady := meta.IsStatusConditionTrue(managedZone.Status.Conditions, "Ready")

	if !managedZoneReady {
		return nil, fmt.Errorf("the managed zone is not in a ready state : %s", managedZone.Name)
	}

	return managedZone, nil
}

// setDNSRecordCondition adds or updates a given condition in the DNSRecord status..
func setDNSRecordCondition(dnsRecord *v1alpha1.DNSRecord, conditionType string, status metav1.ConditionStatus, reason, message string) {
	cond := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: dnsRecord.Generation,
	}
	meta.SetStatusCondition(&dnsRecord.Status.Conditions, cond)
}
