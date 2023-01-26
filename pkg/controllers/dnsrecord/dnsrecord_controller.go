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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns/aws"
)

const (
	DNSRecordFinalizer = "kuadrant.io/dns-record"

	ConditionTrue  v1.ConditionStatus = "True"
	ConditionFalse v1.ConditionStatus = "False"
)

var Clock clock.Clock = clock.RealClock{}

// DNSRecordReconciler reconciles a DNSRecord object
type DNSRecordReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ManagedZonesNS string
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnsrecords/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnsrecords/finalizers,verbs=update

func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	previous := &v1.DNSRecord{}
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

	if dnsRecord.Status.ManagedZoneRef == nil {
		err = r.setManagedZone(ctx, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Status().Update(ctx, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	if !controllerutil.ContainsFinalizer(dnsRecord, DNSRecordFinalizer) {
		controllerutil.AddFinalizer(dnsRecord, DNSRecordFinalizer)
		err = r.Update(ctx, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	var reason, message string
	status := ConditionTrue

	// Publish the record
	err = r.publishRecord(ctx, dnsRecord)
	if err != nil {
		status = ConditionFalse
		reason = "ProviderError"
		message = fmt.Sprintf("The DNS provider failed to ensure the record: %v", err)
	} else {
		dnsRecord.Status.ObservedGeneration = dnsRecord.Generation
		dnsRecord.Status.Endpoints = dnsRecord.Spec.Endpoints
	}
	setDNSRecordCondition(dnsRecord, v1.DNSRecordConditionReady, status, reason, message)

	err = r.Status().Update(ctx, dnsRecord)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DNSRecord{}).
		Complete(r)
}

// setManagedZone sets a ManagedZone on the given DNSRecord
func (r *DNSRecordReconciler) setManagedZone(ctx context.Context, dnsRecord *v1.DNSRecord) error {
	managedZone, err := r.findManagedZone(ctx, dnsRecord)
	if err != nil {
		return err
	}
	managedZoneRef := &v1.ManagedZoneReference{
		Name:      managedZone.GetName(),
		Namespace: managedZone.GetNamespace(),
	}
	dnsRecord.Status.ManagedZoneRef = managedZoneRef
	setDNSRecordCondition(dnsRecord, v1.DNSRecordConditionReady, v1.ConditionFalse, "", "")

	return nil
}

// findManagedZone returns the most suitable ManagedZone to publish the given DNSRecord into.
// Currently, this just returns the first ManagedZone found in the DNSRecords own namespace, or if none is found,
// it looks for the first one listed in the DefautManagedZoneNS.
func (r *DNSRecordReconciler) findManagedZone(ctx context.Context, dnsRecord *v1.DNSRecord) (*v1.ManagedZone, error) {
	var managedZones v1.ManagedZoneList
	if err := r.List(ctx, &managedZones, client.InNamespace(dnsRecord.Namespace)); err != nil {
		log.Log.Error(err, "unable to list managed zones")
		return nil, err
	}

	if len(managedZones.Items) > 0 {
		return &managedZones.Items[0], nil
	}

	if err := r.List(ctx, &managedZones, client.InNamespace(r.ManagedZonesNS)); err != nil {
		log.Log.Error(err, "unable to list managed zones in default NS")
		return nil, err
	}

	if len(managedZones.Items) > 0 {
		return &managedZones.Items[0], nil
	}

	return nil, fmt.Errorf("no managed zone found for : %s", dnsRecord.Name)
}

// deleteRecord deletes record(s) in the DNSPRovider(i.e. route53) configured by the ManagedZone assigned to this
// DNSRecord (dnsRecord.Status.ManagedZoneRef).
func (r *DNSRecordReconciler) deleteRecord(ctx context.Context, dnsRecord *v1.DNSRecord) error {
	// Just return if we are deleting and a ManagedZone was never set
	if dnsRecord.Status.ManagedZoneRef == nil {
		return nil
	}

	managedZone, err := r.getDNSRecordManagedZone(ctx, dnsRecord)
	if err != nil {
		return err
	}

	dnsProvider, err := r.getManagedZoneDNSProvider(ctx, managedZone)
	if err != nil {
		return err
	}

	err = dnsProvider.Delete(dnsRecord, managedZone)
	if err != nil {
		if strings.Contains(err.Error(), "was not found") {
			log.Log.Info("Record not found in managed zone, continuing", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)
			return nil
		}
		return err
	}
	log.Log.Info("Deleted DNSRecord in manage zone", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)

	return nil
}

// publishRecord publishes record(s) to the DNSPRovider(i.e. route53) configured by the ManagedZone assigned to this
// DNSRecord (dnsRecord.Status.ManagedZoneRef).
func (r *DNSRecordReconciler) publishRecord(ctx context.Context, dnsRecord *v1.DNSRecord) error {

	managedZone, err := r.getDNSRecordManagedZone(ctx, dnsRecord)
	if err != nil {
		return err
	}

	dnsProvider, err := r.getManagedZoneDNSProvider(ctx, managedZone)
	if err != nil {
		return err
	}

	if dnsRecord.Generation == dnsRecord.Status.ObservedGeneration {
		log.Log.Info("Skipping managed zone to which the DNS dnsRecord is already published", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)
		return nil
	}

	err = dnsProvider.Ensure(dnsRecord, managedZone)
	if err != nil {
		return err
	}
	log.Log.Info("Published DNSRecord to manage zone", "dnsRecord", dnsRecord.Name, "managedZone", managedZone.Name)

	return nil
}

// getDNSRecordManagedZone returns the current ManagedZone for the given DNSRecord.
func (r *DNSRecordReconciler) getDNSRecordManagedZone(ctx context.Context, dnsRecord *v1.DNSRecord) (*v1.ManagedZone, error) {
	if dnsRecord.Status.ManagedZoneRef == nil {
		return nil, fmt.Errorf("no managed zone configured for : %s", dnsRecord.Name)
	}

	managedZone := &v1.ManagedZone{}
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: dnsRecord.Status.ManagedZoneRef.Namespace, Name: dnsRecord.Status.ManagedZoneRef.Name}, managedZone)
	if err != nil {
		return nil, err
	}
	return managedZone, nil
}

// getManagedZoneDNSProvider returns the DNSProvider for the given ManagedZone.
func (r *DNSRecordReconciler) getManagedZoneDNSProvider(ctx context.Context, managedZone *v1.ManagedZone) (dnsProvider dns.DNSProvider, err error) {
	providerConfig := managedZone.Spec

	switch {
	case providerConfig.Route53 != nil:
		log.Log.V(3).Info("preparing to create Route53 provider")
		secretAccessKey := ""
		if providerConfig.Route53.SecretAccessKey.Name != "" {
			secretAccessKeySecret := &corev1.Secret{}
			err := r.Client.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: providerConfig.Route53.SecretAccessKey.Name}, secretAccessKeySecret)
			if err != nil {
				return nil, fmt.Errorf("error getting route53 secret access key: %s", err)
			}
			secretAccessKeyBytes, ok := secretAccessKeySecret.Data[providerConfig.Route53.SecretAccessKey.Key]
			if !ok {
				return nil, fmt.Errorf("error getting route53 secret access key: key '%s' not found in secret", providerConfig.Route53.SecretAccessKey.Key)
			}
			secretAccessKey = string(secretAccessKeyBytes)
		}

		dnsProvider, err = aws.NewDNSProvider(providerConfig.Route53.AccessKeyID,
			secretAccessKey,
			providerConfig.Route53.HostedZoneID,
			providerConfig.Route53.Region)
		if err != nil {
			return nil, fmt.Errorf("error instantiating route53 dns provider: %s", err)
		}
	default:
		return dnsProvider, fmt.Errorf("no valid dns provider config found for managed zone")
	}
	return dnsProvider, nil
}

// setDNSRecordCondition adds or updates a given condition in the DNSRecord status..
func setDNSRecordCondition(dnsRecord *v1.DNSRecord, conditionType v1.DNSRecordConditionType, status v1.ConditionStatus, reason, message string) {
	newCondition := v1.DNSRecordCondition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	}

	nowTime := metav1.NewTime(Clock.Now())
	newCondition.LastTransitionTime = nowTime

	// Search through existing conditions
	for idx, cond := range dnsRecord.Status.Conditions {
		if cond.Type != conditionType {
			continue
		}

		if cond.Status == status {
			newCondition.LastTransitionTime = cond.LastTransitionTime
		} else {
			log.Log.Info(fmt.Sprintf("Found status change for DNSRecord %q condition %q: %q -> %q; setting lastTransitionTime to %v", dnsRecord.Name, conditionType, cond.Status, status, nowTime.Time))
		}

		// Overwrite the existing condition
		dnsRecord.Status.Conditions[idx] = newCondition
		return
	}

	// No existing condition, so add a new one
	dnsRecord.Status.Conditions = append(dnsRecord.Status.Conditions, newCondition)
	log.Log.Info(fmt.Sprintf("Setting lastTransitionTime for DNSREcord %q condition %q to %v", dnsRecord.Name, conditionType, nowTime.Time))
}
