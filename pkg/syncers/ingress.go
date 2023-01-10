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

package syncers

import (
	"context"
	"os"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	mctcv1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/multiClusterWatch"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
)

// Ingress reconciles a Ingress object
type Ingress struct {
	client.Client
	Scheme           *runtime.Scheme
	ControlSecretRef client.ObjectKey
	HandlerFactory   multiClusterWatch.ResourceHandlerFactory
	Handler          multiClusterWatch.ResourceHandler
	Host             string
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *Ingress) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	if err := r.ensureHandler(); err != nil {
		return ctrl.Result{}, err
	}

	current := &networkingv1.Ingress{}
	err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, current)
	if err != nil {
		return ctrl.Result{}, err
	}

	target := current.DeepCopy()
	accessor := traffic.NewIngress(target)
	res, err := r.Handler.Handle(ctx, accessor)
	if err != nil {
		return res, err
	}
	if !equality.Semantic.DeepEqual(current, target) {
		log.Log.Info("update the ingress")
	}
	return res, nil
}

func (r *Ingress) ensureHandler() error {
	//create the control plane client
	controlConfigSecret := &corev1.Secret{}
	err := r.Client.Get(context.Background(), r.ControlSecretRef, controlConfigSecret)
	if err != nil {
		log.Log.Error(err, "Syncer agent missing control plane config secret", "name", r.ControlSecretRef.Name, "namespace", r.ControlSecretRef.Namespace)
		os.Exit(1)
	}
	ControlClient, err := multiClusterWatch.ClientFromArgoSecret(controlConfigSecret)
	if err != nil {
		log.Log.Error(err, "Syncer agent failed to create client from control plane config secret", "error", err)
		return err
	}
	//add expected custom resources to control plane client scheme
	mctcv1.AddToScheme(ControlClient.Scheme())

	//if handler already created, ensure control plane client is upto date
	if r.Handler != nil {
		r.Handler.SetControlPlaneClient(ControlClient)
		return nil
	}

	r.Handler, err = r.HandlerFactory(r.Host, r.Client, ControlClient)
	if err != nil {
		log.Log.Error(err, "Syncer agent failed to create client from control plane config secret", "error", err)
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Ingress) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(event event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(event event.DeleteEvent) bool {
				return true
			},
			UpdateFunc: func(event event.UpdateEvent) bool {
				return true
			},
		}).
		Complete(r)
}
