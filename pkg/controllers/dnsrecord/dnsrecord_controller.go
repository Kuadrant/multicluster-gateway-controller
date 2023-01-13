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
	"os"
	"reflect"
	"strings"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilclock "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
)

type ConditionStatus string

const (
	DNSRecordFinalizer = "kuadrant.io/dns-record"

	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

type DNSRecordReconcilerConfig struct {
	DNSProvider string
}

// DNSRecordReconciler reconciles a DNSRecord object
type DNSRecordReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	ReconcilerConfig DNSRecordReconcilerConfig
	DNSProvider      dns.Provider
	DNSZones         []v1.DNSZone
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

	if dnsRecord.DeletionTimestamp != nil && !dnsRecord.DeletionTimestamp.IsZero() {
		if err := r.deleteRecord(dnsRecord); err != nil && !strings.Contains(err.Error(), "was not found") {
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

	statuses := r.publishRecordToZones(r.DNSZones, dnsRecord)
	if !dnsZoneStatusSlicesEqual(statuses, dnsRecord.Status.Zones) || dnsRecord.Status.ObservedGeneration != dnsRecord.Generation {
		dnsRecord.Status.Zones = statuses
		dnsRecord.Status.ObservedGeneration = dnsRecord.Generation
	}

	err = r.Status().Update(ctx, dnsRecord)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {

	var dnsZones []v1.DNSZone
	zoneID, zoneIDSet := os.LookupEnv("AWS_DNS_PUBLIC_ZONE_ID")
	if zoneIDSet {
		dnsZone := &v1.DNSZone{
			ID: zoneID,
		}
		dnsZones = append(dnsZones, *dnsZone)
		log.Log.Info("Using AWS DNS zone", "id", zoneID)
	} else {
		log.Log.Info("No AWS DNS zone id set (AWS_DNS_PUBLIC_ZONE_ID), no DNS records will be created!")
	}
	r.DNSZones = dnsZones

	//Logging state of AWS credentials
	awsIdKey := os.Getenv("AWS_ACCESS_KEY_ID")
	if awsIdKey != "" {
		log.Log.Info("AWS Access Key set")
	} else {
		log.Log.Info("AWS Access Key is NOT set")
	}

	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretKey != "" {
		log.Log.Info("AWS Secret Key set")
	} else {
		log.Log.Info("AWS Secret Key is NOT set")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DNSRecord{}).
		Complete(r)
}

func (r *DNSRecordReconciler) publishRecordToZones(zones []v1.DNSZone, record *v1.DNSRecord) []v1.DNSZoneStatus {
	var statuses []v1.DNSZoneStatus
	for i := range zones {
		zone := zones[i]

		// Only publish the record if the DNSRecord has been modified
		// (which would mean the target could have changed) or its
		// status does not indicate that it has already been published.
		if record.Generation == record.Status.ObservedGeneration && recordIsAlreadyPublishedToZone(record, &zone) {
			log.Log.Info("Skipping zone to which the DNS record is already published", "record", record, "zone", zone)
			continue
		}

		condition := v1.DNSZoneCondition{
			Status:             string(ConditionUnknown),
			Type:               v1.DNSRecordFailedConditionType,
			LastTransitionTime: metav1.Now(),
		}

		if recordIsAlreadyPublishedToZone(record, &zone) {
			log.Log.Info("replacing DNS record", "record", record, "zone", zone)

			if err := r.DNSProvider.Ensure(record, zone); err != nil {
				log.Log.Error(err, "Failed to replace DNS record in zone", "record", record.Spec, "zone", zone)
				condition.Status = string(ConditionTrue)
				condition.Reason = "ProviderError"
				condition.Message = fmt.Sprintf("The DNS provider failed to replace the record: %v", err)
			} else {
				log.Log.Info("Replaced DNS record in zone", "record", record.Spec, "zone", zone)
				condition.Status = string(ConditionFalse)
				condition.Reason = "ProviderSuccess"
				condition.Message = "The DNS provider succeeded in replacing the record"
			}
		} else {
			if err := r.DNSProvider.Ensure(record, zone); err != nil {
				log.Log.Error(err, "Failed to publish DNS record to zone", "record", record.Spec, "zone", zone)
				condition.Status = string(ConditionTrue)
				condition.Reason = "ProviderError"
				condition.Message = fmt.Sprintf("The DNS provider failed to ensure the record: %v", err)
			} else {
				log.Log.Info("Published DNS record to zone", "record", record.Spec, "zone", zone)
				condition.Status = string(ConditionFalse)
				condition.Reason = "ProviderSuccess"
				condition.Message = "The DNS provider succeeded in ensuring the record"
			}
		}
		statuses = append(statuses, v1.DNSZoneStatus{
			DNSZone:    zone,
			Conditions: []v1.DNSZoneCondition{condition},
			Endpoints:  record.Spec.Endpoints,
		})
	}
	return mergeStatuses(zones, record.Status.DeepCopy().Zones, statuses)
}

func (r *DNSRecordReconciler) deleteRecord(record *v1.DNSRecord) error {
	var errs []error
	for i := range record.Status.Zones {
		zone := record.Status.Zones[i].DNSZone
		// If the record is currently not published in a zone,
		// skip deleting it for that zone.
		if !recordIsAlreadyPublishedToZone(record, &zone) {
			continue
		}
		err := r.DNSProvider.Delete(record, zone)
		if err != nil {
			errs = append(errs, err)
		} else {
			log.Log.Info("Deleted DNSRecord from DNS provider", "record", record.Spec, "zone", zone)
		}
	}
	if len(errs) == 0 {
		controllerutil.RemoveFinalizer(record, DNSRecordFinalizer)
	}
	return utilerrors.NewAggregate(errs)
}

// recordIsAlreadyPublishedToZone returns a Boolean value indicating whether the
// given DNSRecord is already published to the given zone, as determined from
// the DNSRecord's status conditions.
func recordIsAlreadyPublishedToZone(record *v1.DNSRecord, zoneToPublish *v1.DNSZone) bool {
	for _, zoneInStatus := range record.Status.Zones {
		if !reflect.DeepEqual(&zoneInStatus.DNSZone, zoneToPublish) {
			continue
		}

		for _, condition := range zoneInStatus.Conditions {
			if condition.Type == v1.DNSRecordFailedConditionType {
				return condition.Status == string(ConditionFalse)
			}
		}
	}

	return false
}

// mergeStatuses updates or extends the provided slice of statuses with the
// provided updates and returns the resulting slice.
func mergeStatuses(zones []v1.DNSZone, statuses, updates []v1.DNSZoneStatus) []v1.DNSZoneStatus {
	var additions []v1.DNSZoneStatus
	for i, update := range updates {
		add := true
		for j, status := range statuses {
			if cmp.Equal(status.DNSZone, update.DNSZone) {
				add = false
				statuses[j].Conditions = mergeConditions(status.Conditions, update.Conditions)
				statuses[j].Endpoints = update.Endpoints
			}
		}
		if add {
			additions = append(additions, updates[i])
		}
	}
	return append(statuses, additions...)
}

// clock is to enable unit testing
var clock utilclock.Clock = utilclock.RealClock{}

// mergeConditions adds or updates matching conditions, and updates
// the transition time if details of a condition have changed. Returns
// the updated condition array.
func mergeConditions(conditions, updates []v1.DNSZoneCondition) []v1.DNSZoneCondition {
	now := metav1.NewTime(clock.Now())
	var additions []v1.DNSZoneCondition
	for i, update := range updates {
		add := true
		for j, cond := range conditions {
			if cond.Type == update.Type {
				add = false
				if conditionChanged(cond, update) {
					conditions[j].Status = update.Status
					conditions[j].Reason = update.Reason
					conditions[j].Message = update.Message
					conditions[j].LastTransitionTime = now
					break
				}
			}
		}
		if add {
			updates[i].LastTransitionTime = now
			additions = append(additions, updates[i])
		}
	}
	conditions = append(conditions, additions...)
	return conditions
}

func conditionChanged(a, b v1.DNSZoneCondition) bool {
	return a.Status != b.Status || a.Reason != b.Reason
}

// dnsZoneStatusSlicesEqual compares two DNSZoneStatus slice values.  Returns
// true if the provided values should be considered equal for the purpose of
// determining whether an update is necessary, false otherwise.  The comparison
// is agnostic with respect to the ordering of status conditions but not with
// respect to zones.
func dnsZoneStatusSlicesEqual(a, b []v1.DNSZoneStatus) bool {
	conditionCmpOpts := []cmp.Option{
		cmpopts.EquateEmpty(),
		cmpopts.SortSlices(func(a, b v1.DNSZoneCondition) bool {
			return a.Type < b.Type
		}),
	}
	return cmp.Equal(a, b, conditionCmpOpts...)
}
