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

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/gateway"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	DNSPolicyFinalizer = "kuadrant.io/dns-policy"
)

// DNSPolicyReconciler reconciles a DNSPolicy object
type DNSPolicyReconciler struct {
	Client client.Client

	DNSProvider dns.Provider
	HostService gateway.HostService
}

//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kuadrant.io,resources=dnspolicies/finalizers,verbs=update

func (r *DNSPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	previous := &v1alpha1.DNSPolicy{}
	if err := r.Client.Get(ctx, req.NamespacedName, previous); err != nil {
		if err := client.IgnoreNotFound(err); err == nil {
			return ctrl.Result{}, nil
		} else {
			return ctrl.Result{}, err
		}
	}

	dnsPolicy := previous.DeepCopy()
	log.V(3).Info("DNSPolicyReconciler Reconcile", "dnsPolicy", dnsPolicy)

	if err := r.ReconcileHealthChecks(ctx, log, dnsPolicy); err != nil {
		return ctrl.Result{}, err
	}

	if !equality.Semantic.DeepEqual(previous.Status, dnsPolicy.Status) {
		err := r.Client.Status().Update(ctx, dnsPolicy)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DNSPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DNSPolicy{}).
		Watches(&source.Kind{
			Type: &v1alpha1.DNSRecord{},
		}, &DNSRecordEventHandler{
			client:      r.Client,
			hostService: r.HostService,
		}).
		Complete(r)
}
