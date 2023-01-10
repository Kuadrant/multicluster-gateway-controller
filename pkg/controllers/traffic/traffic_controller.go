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

package traffic

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/metadata"
	mctcv1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
)

const (
	CONTROL_PLANE_NAMESPACE string = "default"
	PATCH_ANNOTATION_PREFIX string = "MCTC_PATCH_"
	PATCH_CLEANUP_FINALIZER string = "MCTC_PATCH_CLEANUP"
)

// Reconciler reconciles a traffic object
type Reconciler struct {
	WorkloadClient client.Client
	ControlClient  client.Client
	ClusterID      string
}

func (r *Reconciler) SetWorkloadClient(c client.Client) {
	r.WorkloadClient = c
}

func (r *Reconciler) SetControlPlaneClient(c client.Client) {
	r.ControlClient = c
}

func (r *Reconciler) Handle(ctx context.Context, o runtime.Object) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	trafficAccessor := o.(traffic.Interface)
	log.Log.Info("got traffic object", "kind", trafficAccessor.GetKind(), "name", trafficAccessor.GetName(), "namespace", trafficAccessor.GetNamespace())

	dnsTargets := trafficAccessor.GetDNSTargets()
	targets := []string{}
	for _, target := range dnsTargets {
		targets = append(targets, target.Value)
	}

	//build patches to add dns targets to all matched DNSRecords
	patches := []*Patch{}
	for _, target := range dnsTargets {
		patch := &Patch{
			OP:    "add",
			Path:  "/spec/endpoints/0/targets/-",
			Value: target.Value,
		}
		patches = append(patches, patch)
	}
	patchAnnotation, err := json.Marshal(patches)

	if err != nil {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, fmt.Errorf("could not convert patches to string. Patches: %+v, error: %v", patches, err)
	}
	for _, host := range trafficAccessor.GetHosts() {
		dnsRecord := &mctcv1.DNSRecord{}
		err := r.ControlClient.Get(ctx, client.ObjectKey{Name: host, Namespace: CONTROL_PLANE_NAMESPACE}, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}

		if len(dnsTargets) > 0 && trafficAccessor.GetDeletionTimestamp() == nil {
			metadata.AddAnnotation(dnsRecord, PATCH_ANNOTATION_PREFIX+r.ClusterID, string(patchAnnotation))
			controllerutil.AddFinalizer(trafficAccessor, PATCH_CLEANUP_FINALIZER)
		} else {
			metadata.RemoveAnnotation(dnsRecord, PATCH_ANNOTATION_PREFIX+r.ClusterID)
			controllerutil.RemoveFinalizer(trafficAccessor, PATCH_CLEANUP_FINALIZER)
		}

		err = r.ControlClient.Update(ctx, dnsRecord)
		if err != nil {
			return ctrl.Result{}, err
		}

	}
	return ctrl.Result{}, nil
}
